# PostgreSQL Authentication Issue - Resolution Steps

## Problem Summary

- PostgreSQL is running on port 5432
- Authentication fails with credentials `postgres:postgres`
- Cannot determine correct credentials or reset password without sudo access
- Need database access for user-org-service migrations and login functionality

## Root Cause

PostgreSQL on port 5432 appears to be a system PostgreSQL instance (not Docker) with:
- Unknown authentication credentials
- Possibly different authentication method (peer, ident, etc.)
- Cannot access pg_hba.conf without sudo to verify settings

## Solutions

### Option 1: Use Docker PostgreSQL on Alternate Port (Recommended)

Since Docker Compose PostgreSQL expects port 5432 but it's already in use, use port 5433:

```bash
# Set environment variable to use port 5433
export POSTGRES_PORT=5433

# Start Docker PostgreSQL stack on port 5433
cd /home/dev/ai-aas
make up  # Or: docker compose -f .dev/compose/compose.base.yaml -f .dev/compose/compose.local.yaml up -d

# Update connection string to use port 5433
export USER_ORG_DATABASE_URL="postgres://postgres:postgres@localhost:5433/ai_aas?sslmode=disable"

# Run migrations
cd services/user-org-service
make migrate
```

**Note**: This requires Docker access. If you get permission errors:
```bash
# Option A: Add yourself to docker group (requires sudo, then logout/login)
sudo usermod -aG docker $USER
newgrp docker  # or logout and login

# Option B: Use sudo for docker commands
sudo make up
```

### Option 2: Fix Existing PostgreSQL Password (Requires sudo)

If you have sudo access and want to use existing PostgreSQL:

```bash
# Connect as postgres user (peer authentication)
sudo -u postgres psql

# In psql, reset password:
ALTER USER postgres WITH PASSWORD 'postgres';
\q

# Test connection
PGPASSWORD=postgres psql -h localhost -U postgres -d postgres -c "SELECT 1;"
```

### Option 3: Stop Existing PostgreSQL and Use Docker

If you can stop the existing PostgreSQL:

```bash
# Stop system PostgreSQL
sudo systemctl stop postgresql  # or find the process and kill it

# Start Docker PostgreSQL
make up

# This will use port 5432 with known credentials (postgres:postgres)
```

### Option 4: Use Different Database Name/Port

If existing PostgreSQL has a different database or you want to keep both:

```bash
# Use a different database name in existing PostgreSQL
export USER_ORG_DATABASE_URL="postgres://postgres:UNKNOWN_PASSWORD@localhost:5432/user_org?sslmode=disable"

# Or use port 5433 with Docker
export POSTGRES_PORT=5433
make up
```

## Recommended Next Steps

1. **Check Docker permissions**: Try `docker ps` - if it works, use Option 1
2. **If Docker doesn't work**: Get Docker access (Option A or B above)
3. **Once Docker PostgreSQL is running on port 5433**:
   - Update environment profile (local-dev) to use port 5433
   - Run migrations
   - Start services

## Verification

After starting Docker PostgreSQL:

```bash
# Check container is running
docker ps | grep postgres

# Test connection (if psql is available)
PGPASSWORD=postgres psql -h localhost -p 5433 -U postgres -d ai_aas -c "SELECT 1;"

# Or use Docker exec
docker exec dev-postgres psql -U postgres -d ai_aas -c "SELECT 1;"
```

## Integration with Environment Profiles

Once Docker PostgreSQL is running, we should update the environment profile system:

1. Create/update `configs/environments/local-dev.yaml` to use port 5433
2. Add component definition for PostgreSQL in `configs/components.yaml`
3. Use `make env-activate ENVIRONMENT=local-dev` to activate profile
4. Connection strings will come from the profile automatically

