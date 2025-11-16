# Fix Docker Permissions to Start Database

## Quick Fix

```bash
# Add yourself to docker group (requires sudo password)
sudo usermod -aG docker $USER

# Apply new group membership immediately
newgrp docker

# Verify Docker access
docker ps

# Start PostgreSQL on port 5433 to avoid conflict
export POSTGRES_PORT=5433
make up

# Test connection
docker exec dev-postgres psql -U postgres -d ai_aas -c "SELECT 1;"
```

## Alternative: Use sudo for Docker

If you can't be added to docker group, use sudo:

```bash
# Start stack with sudo
export POSTGRES_PORT=5433
sudo docker compose -f .dev/compose/compose.base.yaml -f .dev/compose/compose.local.yaml up -d

# Verify
sudo docker ps | grep postgres
```

