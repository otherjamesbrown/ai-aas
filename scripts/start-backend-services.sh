#!/bin/bash
# Start backend services for login testing
# This script starts the required services for the web portal login

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${GREEN}Starting Backend Services for Login Testing${NC}"
echo ""

# Setup Go path
export PATH="/home/dev/go-bin/go/bin:$PATH"
export GOTOOLCHAIN=go1.24.10

# Check Docker access
if ! docker ps &>/dev/null; then
    echo -e "${RED}Error: Cannot access Docker.${NC}"
    echo -e "${YELLOW}Please either:${NC}"
    echo "  1. Add yourself to the docker group: sudo usermod -aG docker $USER"
    echo "     Then log out and back in, or run: newgrp docker"
    echo "  2. Use sudo (will prompt for password)"
    echo ""
    read -p "Would you like to use sudo? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Exiting. Please fix Docker permissions and try again."
        exit 1
    fi
    DOCKER_CMD="sudo docker"
    DOCKER_COMPOSE_CMD="sudo docker compose"
else
    DOCKER_CMD="docker"
    DOCKER_COMPOSE_CMD="docker compose"
fi

# Start PostgreSQL database
echo -e "${GREEN}[1/6] Starting PostgreSQL database...${NC}"
cd "$PROJECT_ROOT"
$DOCKER_COMPOSE_CMD -f services/analytics-service/dev/docker-compose.yml up -d postgres 2>&1 || {
    # Try alternative: create a simple postgres container for user-org-service
    echo "Starting dedicated PostgreSQL for user-org-service..."
    $DOCKER_CMD run -d \
        --name user-org-postgres \
        -e POSTGRES_USER=postgres \
        -e POSTGRES_PASSWORD=postgres \
        -e POSTGRES_DB=user_org \
        -p 5433:5432 \
        postgres:15-alpine \
        >/dev/null 2>&1 || echo "Database might already be running"
    
    echo "Waiting for database to be ready..."
    sleep 5
}

# Wait for database
echo -e "${GREEN}[2/6] Waiting for database to be ready...${NC}"
for i in {1..30}; do
    if $DOCKER_CMD exec user-org-postgres pg_isready -U postgres &>/dev/null || \
       nc -z localhost 5432 &>/dev/null || \
       nc -z localhost 5433 &>/dev/null; then
        echo "Database is ready!"
        break
    fi
    sleep 1
done

# Set database URL
if $DOCKER_CMD ps --format "{{.Names}}" | grep -q "^user-org-postgres$"; then
    export DATABASE_URL="postgres://postgres:postgres@localhost:5433/user_org?sslmode=disable"
else
    export DATABASE_URL="postgres://postgres:postgres@localhost:5432/ai_aas?sslmode=disable"
fi
export USER_ORG_DATABASE_URL="$DATABASE_URL"

# Run migrations
echo -e "${GREEN}[3/6] Running database migrations...${NC}"
cd "$PROJECT_ROOT/services/user-org-service"
make migrate 2>&1 || {
    echo -e "${YELLOW}Warning: Migrations might have already been applied${NC}"
}

# Seed database
echo -e "${GREEN}[4/6] Seeding database with test user...${NC}"
cd "$PROJECT_ROOT/services/user-org-service"
go run cmd/seed/main.go \
    -user-email=admin@example.com \
    -user-password=nubipwdkryfmtaho123! \
    -org-slug=demo \
    -org-name="Demo Organization" \
    2>&1 || {
    echo -e "${YELLOW}Warning: Database might already be seeded${NC}"
}

# Start user-org-service
echo -e "${GREEN}[5/6] Starting user-org-service on port 8081...${NC}"
cd "$PROJECT_ROOT/services/user-org-service"
export HTTP_PORT=8081
export OAUTH_HMAC_SECRET=$(openssl rand -hex 32 2>/dev/null || echo "dev-secret-key-change-in-production")
export OAUTH_CLIENT_SECRET=$(openssl rand -hex 32 2>/dev/null || echo "dev-client-secret-change-in-production")
./admin-api > /tmp/user-org-service.log 2>&1 &
USER_ORG_PID=$!
echo "user-org-service started (PID: $USER_ORG_PID)"

# Wait for service to be ready
echo "Waiting for user-org-service to be ready..."
sleep 3
for i in {1..30}; do
    if curl -s http://localhost:8081/healthz &>/dev/null; then
        echo "user-org-service is ready!"
        break
    fi
    sleep 1
done

# Start dependencies for API router (Redis, Kafka)
echo -e "${GREEN}[6/6] Starting API router dependencies (Redis, Kafka)...${NC}"
cd "$PROJECT_ROOT/services/api-router-service"
make dev-up 2>&1 || echo "Dependencies might already be running"

# Wait for Redis
sleep 5

# Start API router service
echo -e "${GREEN}Starting API router service on port 8080...${NC}"
cd "$PROJECT_ROOT/services/api-router-service"
export HTTP_PORT=8080
export USER_ORG_SERVICE_URL=http://localhost:8081
export REDIS_ADDR=localhost:6379
export KAFKA_BROKERS=localhost:9092
./router > /tmp/api-router-service.log 2>&1 &
API_ROUTER_PID=$!
echo "API router service started (PID: $API_ROUTER_PID)"

# Wait for service to be ready
echo "Waiting for API router to be ready..."
sleep 3
for i in {1..30}; do
    if curl -s http://localhost:8080/v1/status/healthz &>/dev/null; then
        echo "API router service is ready!"
        break
    fi
    sleep 1
done

echo ""
echo -e "${GREEN}✓ All services started successfully!${NC}"
echo ""
echo "Services running:"
echo "  - PostgreSQL: $DATABASE_URL"
echo "  - user-org-service: http://localhost:8081 (PID: $USER_ORG_PID)"
echo "  - API router: http://localhost:8080 (PID: $API_ROUTER_PID)"
echo ""
echo "Logs:"
echo "  - user-org-service: tail -f /tmp/user-org-service.log"
echo "  - API router: tail -f /tmp/api-router-service.log"
echo ""
echo "To stop services:"
echo "  pkill -f admin-api"
echo "  pkill -f router"
echo "  docker stop user-org-postgres"
echo ""
echo "Test credentials:"
echo "  Email: admin@example.com"
echo "  Password: nubipwdkryfmtaho123!"

