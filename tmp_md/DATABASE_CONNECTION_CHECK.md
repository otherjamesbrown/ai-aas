# Database Connection Analysis

## Current Status

Based on process inspection, the PostgreSQL running on port 5432 appears to be **local** (not Linode DBaaS):

- Process: `postgres` running as UID 70
- Port: Listening on `0.0.0.0:5432` (all interfaces)
- Features: TimescaleDB extension detected

## Authentication Issue

The connection is failing with:
```
password authentication failed for user "postgres" (SQLSTATE 28P01)
```

## How to Determine Database Type

### Check if it's Local PostgreSQL

```bash
# Check if PostgreSQL is a system service
systemctl status postgresql 2>&1 | head -10

# Check PostgreSQL data directory
sudo -u postgres psql -c "SHOW data_directory;" 2>&1

# Check PostgreSQL version
sudo -u postgres psql -c "SELECT version();" 2>&1
```

### Check if it's Linode DBaaS (via SSH tunnel or proxy)

```bash
# Check for SSH tunnels
ps aux | grep -E "ssh.*5432|linode.*5432" | grep -v grep

# Check environment variables for Linode connection
env | grep -iE "LINODE.*DB|DBaaS|DATABASE.*HOST"

# Check for connection strings pointing to remote hosts
grep -r "db.linode\|db-.*\.linode\|linode.*database" ~/.config ~/.env* 2>/dev/null
```

## Solutions

### If it's Local PostgreSQL:

#### Option 1: Reset PostgreSQL Password (requires sudo)

```bash
# Connect as postgres user without password
sudo -u postgres psql

# Then in psql:
ALTER USER postgres WITH PASSWORD 'postgres';
\q
```

#### Option 2: Check PostgreSQL Authentication Config

```bash
# Check pg_hba.conf
sudo cat /etc/postgresql/*/main/pg_hba.conf | grep -E "^[^#]"

# Or for Docker/container:
docker exec -it <postgres-container> cat /var/lib/postgresql/data/pg_hba.conf
```

#### Option 3: Use Peer Authentication (local only)

If `pg_hba.conf` uses `peer` authentication, connect as postgres user:
```bash
sudo -u postgres psql -d ai_aas
```

### If it's Linode DBaaS:

#### Option 1: Get Connection String from Linode Dashboard

1. Log into Linode Dashboard
2. Go to Databases → Your Database Instance
3. Copy the connection string (should include host, port, SSL settings)
4. Update `USER_ORG_DATABASE_URL`:

```bash
export USER_ORG_DATABASE_URL="postgres://user:password@db-xxxxx-xxx.linode:5432/dbname?sslmode=require"
```

#### Option 2: Use SSH Tunnel (if not already set up)

If database requires SSH tunneling:
```bash
ssh -L 5432:db-xxxxx-xxx.linode:5432 user@jump-host
```

Then use `localhost:5432` in connection string.

#### Option 3: Check for Existing Tunnel

```bash
# Check if there's already an SSH tunnel
ps aux | grep ssh | grep 5432
netstat -tuln | grep 5432

# If tunnel exists, credentials might be in:
# - Environment variables
# - Config files (~/.config/ai-aas/*)
# - Secret management (Vault, 1Password, etc.)
```

## Recommended Next Steps

1. **Determine database type**: Run the checks above
2. **Get correct credentials**: 
   - Local: Reset password or check pg_hba.conf
   - Linode: Get from Linode Dashboard or existing config
3. **Test connection**:
   ```bash
   psql "$USER_ORG_DATABASE_URL" -c "SELECT 1;"
   ```
4. **Run migrations** once connection works
5. **Start services** with correct database URL

## Quick Test Commands

```bash
# Test with different passwords
PGPASSWORD=postgres psql -h localhost -U postgres -d ai_aas -c "SELECT 1;" 2>&1
PGPASSWORD=your-password psql -h localhost -U postgres -d ai_aas -c "SELECT 1;" 2>&1

# Test with peer auth (local system user)
sudo -u postgres psql -d ai_aas -c "SELECT 1;" 2>&1

# Test if it's actually Linode (will timeout if local)
# Replace with actual Linode DBaaS host
ping db-xxxxx-xxx.linode.com 2>&1 | head -3
```

