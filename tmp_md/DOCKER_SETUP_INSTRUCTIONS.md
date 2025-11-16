# Docker Setup Instructions

## Quick Setup

To enable Docker access and start the database, run:

```bash
# Step 1: Add yourself to docker group (requires sudo password)
sudo usermod -aG docker $USER

# Step 2: Apply new group membership (choose one):
# Option A: Start new shell session
newgrp docker

# Option B: Logout and login again
# (Then continue with remaining steps)

# Step 3: Verify Docker access
docker ps

# Step 4: Activate local-dev environment profile
cd /home/dev/ai-aas
make env-activate ENVIRONMENT=local-dev

# Step 5: Start Docker PostgreSQL on port 5433
export POSTGRES_PORT=5433
make up

# Step 6: Verify PostgreSQL is running
docker ps | grep postgres
make env-component-status

# Step 7: Run migrations
export USER_ORG_DATABASE_URL="postgres://postgres:postgres@localhost:5433/ai_aas?sslmode=disable"
cd services/user-org-service && make migrate

# Step 8: Seed database
go run cmd/seed/main.go \
    -user-email=admin@example.com \
    -user-password=nubipwdkryfmtaho123! \
    -org-slug=demo \
    -org-name="Demo Organization"

# Step 9: Start backend services
# (user-org-service and api-router-service)
```

## Alternative: Use sudo for Docker

If you can't be added to docker group, use sudo:

```bash
# Start with sudo
export POSTGRES_PORT=5433
sudo docker compose -f .dev/compose/compose.base.yaml -f .dev/compose/compose.local.yaml up -d

# Verify
sudo docker ps | grep postgres

# Note: For ongoing use, adding to docker group is recommended
```

## Environment Profile System

The environment profile system automatically keeps configuration in sync:

1. **Activate environment**: `make env-activate ENVIRONMENT=local-dev`
   - Sets active environment to local-dev
   - Generates `.env.local` from profile

2. **View configuration**: `make env-show [COMPONENT=postgres]`
   - Shows current environment configuration
   - Optionally filter by component

3. **Sync changes**: `make env-sync`
   - Updates `.env.local` from active profile
   - Syncs secrets if available

4. **Check status**: `make env-component-status`
   - Shows which components are running
   - Checks port accessibility

5. **Export variables**: `make env-export FORMAT=env > .env`
   - Exports environment variables in various formats

## Troubleshooting

### Docker permission denied
```bash
# Check if user is in docker group
groups | grep docker

# If not, add yourself (requires sudo)
sudo usermod -aG docker $USER
newgrp docker  # or logout/login

# Verify
docker ps
```

### Port 5432 conflict
```bash
# Use port 5433 instead (already configured in local-dev profile)
export POSTGRES_PORT=5433
make up
```

### Environment profile not found
```bash
# List available environments
make env-list

# Activate one
make env-activate ENVIRONMENT=local-dev
```

### Configuration out of sync
```bash
# Regenerate .env file from profile
make env-sync

# Or manually generate
./configs/manage-env.sh generate-env-file
```

