# Seeded Test Data

This document provides details on the users, organizations, and API keys that are seeded into the development environment for testing purposes.

## How to Seed the Data

To seed the database, run the following script from the root of the repository:

```bash
./scripts/seed-test-users.sh
```

## Seeded Organizations and Users

### System Admin
- **User**: `admin@ai-aas.dev`
- **Note**: This user is the super-admin for the entire system.

### Acme Ltd
- **Organization Slug**: `acme-ltd`
- **Admin User**: `admin@acme-ltd.com`
- **Manager User**: `manager@acme-ltd.com`

### JoeBlogs Ltd
- **Organization Slug**: `joeblogs-ltd`
- **Admin User**: `admin@joeblogs-ltd.com`
- **Manager User**: `manager@joeblogs-ltd.com`

**Note**: The passwords for these users are not documented.

## E2E Admin API Key

For end-to-end testing, a special admin API key is required. This key has full `["*"]` scope.

### How to Create the E2E Admin Key

The key is **not** seeded by default. You must create it by running the following script:

```bash
python3 scripts/create-e2e-admin-key.py > /tmp/e2e-admin-setup.sql
```

Then, apply the generated SQL to the database as described in the [E2E Admin API Key Setup Guide](./e2e/SETUP_E2E_ADMIN_KEY.md).

The script will output the API key secret. **Save this key securely**.
