# E2E Test Setup Guide

## Making Tests Repeatable

The e2e test harness is designed to be fully repeatable. Here's how:

### Initial Setup (One-Time)

**Recommended: Use the simple setup script** (extracts key from secrets):

```bash
cd tests/e2e
./scripts/setup-admin-key.sh
```

This automatically extracts `MASTER_ADMIN_API_KEY` from `secrets/env/.env` and saves it to `.admin-key.env`.

**Alternative: Full bootstrap** (creates new org/key):

```bash
cd tests/e2e
make setup
```

**If you don't have secrets access**, create an admin key first:

```bash
# Option A: Use seed command to create admin user
cd services/user-org-service
go run cmd/seed/main.go -org-slug=e2e-admin -user-email=admin@e2e.test
# Then log in via web portal and create API key

# Option B: Use existing admin key
export ADMIN_API_KEY=your-existing-key
cd tests/e2e
make setup
```

### Running Tests (Repeatable)

After initial setup, tests are fully repeatable:

```bash
cd tests/e2e
make test-dev-internet  # Automatically loads .admin-key.env
```

The `.admin-key.env` file persists between runs, so you only need to run setup once.

### What Gets Created

- **Admin API Key**: Saved to `.admin-key.env`
- **Service URLs**: Configured automatically by test scripts

### Cleanup

Tests automatically clean up created resources (orgs, API keys, etc.).

### Important Notes

- **Go workspace**: Tests use `GOWORK=off` because they require Go 1.25 (k8s.io deps)
- **Admin key**: The `test-dev-internet` script auto-loads `.admin-key.env`
- **Performance**: Tests should complete in ~6 seconds (not 60s - if slow, check `CLEANUP_DELAY_SECONDS`)

