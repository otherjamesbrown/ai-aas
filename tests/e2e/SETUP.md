# E2E Test Setup Guide

## Making Tests Repeatable

The e2e test harness is designed to be fully repeatable. Here's how:

### Initial Setup (One-Time)

1. **Bootstrap Admin Credentials**:
   ```bash
   cd tests/e2e
   make setup
   ```

2. **If you don't have an admin key**, create one first:
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
make test-dev-ip  # Automatically loads .admin-key.env
```

The `.admin-key.env` file persists between runs, so you only need to run `make setup` once.

### What Gets Created

- **Test Organization**: `e2e-test-admin-org` (or similar)
- **Admin API Key**: Saved to `.admin-key.env`
- **Service URLs**: Configured automatically

### Cleanup

Tests automatically clean up created resources. The admin key and org are kept for reuse.

