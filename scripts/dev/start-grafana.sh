#!/usr/bin/env bash
# Script to start Grafana, Loki, and Promtail for local development
# Usage: ./scripts/dev/start-grafana.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
COMPOSE_DIR="${PROJECT_ROOT}/.dev/compose"

cd "${PROJECT_ROOT}"

echo "🚀 Starting Grafana, Loki, and Promtail..."
echo ""

# Check Docker is accessible
if ! docker ps >/dev/null 2>&1; then
    echo "❌ Error: Docker is not accessible"
    echo "   Make sure you've run: newgrp docker"
    echo "   Or log out and back in after adding user to docker group"
    exit 1
fi

# Ensure network exists
echo "📡 Checking network..."
if ! docker network ls | grep -q ai-aas-dev-network; then
    echo "   Creating network..."
    docker network create ai-aas-dev-network
fi

# Verify provisioning config exists
if [ ! -f "${COMPOSE_DIR}/grafana-provisioning/datasources/loki.yaml" ]; then
    echo "❌ Error: Grafana provisioning config not found!"
    echo "   Expected: ${COMPOSE_DIR}/grafana-provisioning/datasources/loki.yaml"
    exit 1
fi

# Start services
echo "🐳 Starting services..."
docker compose -f "${COMPOSE_DIR}/compose.base.yaml" -f "${COMPOSE_DIR}/compose.local.yaml" up -d grafana loki promtail

echo ""
echo "⏳ Waiting for services to start..."
sleep 5

# Check status
echo ""
echo "📊 Service Status:"
docker ps --filter "name=grafana" --filter "name=loki" --filter "name=promtail" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"

echo ""
echo "⏳ Waiting for Grafana to be ready (this may take ~30 seconds)..."
for i in {1..18}; do
    if curl -sf http://localhost:3000/api/health >/dev/null 2>&1; then
        echo "✅ Grafana is ready!"
        break
    fi
    echo "   Waiting... ($i/18)"
    sleep 5
done

echo ""
if curl -sf http://localhost:3000/api/health >/dev/null 2>&1; then
    GRAFANA_URL="http://localhost:3000/explore?orgId=1&left=%5B%22now-1h%22,%22now%22,%22Loki%22,%7B%22expr%22:%22%7Benvironment%3D%5C%22local-dev%5C%22%7D%22%7D%5D"
    
    echo "✅ Grafana is running!"
    echo ""
    echo "🌐 Opening Grafana Explore..."
    
    if command -v xdg-open >/dev/null 2>&1; then
        xdg-open "${GRAFANA_URL}" 2>/dev/null && echo "✅ Browser opened!"
    elif command -v open >/dev/null 2>&1; then
        open "${GRAFANA_URL}" 2>/dev/null && echo "✅ Browser opened!"
    else
        echo "Please open this URL in your browser:"
        echo "${GRAFANA_URL}"
    fi
    
    echo ""
    echo "📝 Login Credentials:"
    echo "   Username: admin"
    echo "   Password: admin"
    echo ""
    echo "💡 The Loki data source should be auto-configured!"
    echo ""
    echo "🔍 Try these queries in Explore:"
    echo "   {environment=\"local-dev\"}"
    echo "   {service=\"user-org-service\"}"
    echo "   {environment=\"local-dev\"} |= \"debug\""
else
    echo "⏳ Grafana is still starting..."
    echo ""
    echo "Check logs:"
    echo "   docker compose -f ${COMPOSE_DIR}/compose.base.yaml -f ${COMPOSE_DIR}/compose.local.yaml logs grafana"
    echo ""
    echo "Or check status:"
    echo "   docker ps | grep grafana"
    echo ""
    echo "Manual access:"
    echo "   http://localhost:3000"
fi

