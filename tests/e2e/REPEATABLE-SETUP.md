# Making E2E Tests Repeatable

## Overview

The e2e test harness is designed to be fully repeatable with minimal manual intervention. Once set up, you can run tests repeatedly without re-entering credentials or reconfiguring.

## Complete Setup Flow

### Step 1: Initial Admin Credential (One-Time)

You need an initial admin credential to bootstrap the test environment. Choose one:

#### Option A: Create Admin User via Seed Command (Recommended)

```bash
cd services/user-org-service
go run cmd/seed/main.go \
  -org-slug=e2e-admin \
  -user-email=admin@e2e.test \
  -user-name="E2E Admin User"
```

This will output:
- Organization ID
- User email
- Generated password

Then:
1. Log in via web portal with those credentials
2. Create an API key with admin scopes
3. Use that API key for bootstrap

#### Option B: Use Existing Admin Key

If you already have an admin API key:
```bash
export ADMIN_API_KEY=your-existing-admin-key
```

### Step 2: Bootstrap Test Environment

```bash
cd tests/e2e
make setup
```

Or manually:
```bash
./scripts/setup-test-env.sh
```

This will:
- ✅ Check prerequisites
- ✅ Configure service URLs (uses IP: 172.232.58.222)
- ✅ Create test organization (if needed)
- ✅ Create admin API key for tests
- ✅ Save credentials to `.admin-key.env`

### Step 3: Run Tests (Repeatable)

After initial setup, tests are fully repeatable:

```bash
cd tests/e2e
make test-dev-ip  # Automatically loads .admin-key.env
```

Or manually:
```bash
source .admin-key.env
go test -v ./suites -timeout 30m
```

## What Makes It Repeatable

### 1. Persistent Credential Storage
- `.admin-key.env` file (git-ignored) stores admin key
- Automatically loaded by Makefile targets
- Persists between test runs

### 2. Idempotent Bootstrap
- Safe to run `make setup` multiple times
- Checks for existing credentials before creating new ones
- Can recreate if credentials expire

### 3. Automatic Configuration
- Service URLs configured automatically
- Host headers set correctly for IP-based access
- HTTPS/TLS handled automatically

### 4. No Manual Steps After Setup
- No need to modify `/etc/hosts`
- No need to re-enter credentials
- No need to reconfigure URLs

## File Structure

```
tests/e2e/
├── .admin-key.env          # Admin credentials (git-ignored, auto-loaded)
├── scripts/
│   ├── setup-test-env.sh   # Complete setup orchestration
│   ├── bootstrap-admin-key.sh  # Creates admin key
│   └── test-dev-internet.sh   # Runs tests via internet
├── Makefile                 # Convenience targets
└── README.md               # Full documentation
```

## Workflow Summary

```bash
# ONE-TIME SETUP
cd tests/e2e
export ADMIN_API_KEY=your-initial-admin-key  # Or create via seed
make setup

# REPEATABLE TEST EXECUTION (any time after setup)
make test-dev-ip
```

## Troubleshooting

### Credentials Expired
```bash
rm .admin-key.env
export ADMIN_API_KEY=your-new-key
make setup
```

### Service URLs Changed
```bash
export USER_ORG_SERVICE_URL=https://new-url
export API_ROUTER_SERVICE_URL=https://new-url
make test-dev-ip
```

### Need to Re-bootstrap
```bash
rm .admin-key.env
make setup
```

## Benefits

✅ **One-time setup** - Configure once, run many times  
✅ **No manual steps** - Fully automated after initial credential  
✅ **Persistent storage** - Credentials saved for reuse  
✅ **Idempotent** - Safe to run setup multiple times  
✅ **Self-contained** - All configuration in one place  
✅ **Git-safe** - Credentials never committed (git-ignored)

