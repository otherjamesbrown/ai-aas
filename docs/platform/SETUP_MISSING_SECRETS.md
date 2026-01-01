# Setup Missing GitHub Secrets - Manual Instructions

---
last_updated: 2025-12-31
document_type: procedure
---

## Purpose

This document provides step-by-step instructions for manually setting up the GitHub repository secrets required for CI/CD workflows to function correctly.

## Background

The E2E tests and other CI/CD workflows are failing because required GitHub secrets are not configured. GitHub secrets cannot be set via API or CLI for security reasons, so they must be added manually via the GitHub web interface.

## Before You Begin

**Verify which secrets are missing:**
```bash
cd /home/dev/worktrees/develop
./scripts/ci/validate-secrets.py
```

This will show you exactly which secrets need to be added.

## Step-by-Step Instructions

### 1. Navigate to GitHub Secrets Settings

1. Go to: https://github.com/otherjamesbrown/ai-aas/settings/secrets/actions
2. You must have admin access to the repository to add secrets
3. If you don't have access, contact the repository owner

### 2. Add Critical Secrets (Required)

For each missing secret below, click **"New repository secret"** and enter:

#### MASTER_ADMIN_API_KEY
- **Name**: `MASTER_ADMIN_API_KEY`
- **Value**: `VXDzIauNfwRdmUDowO37plULPXbf1fUBr-69oqSEWEA`
- **Purpose**: Development environment E2E test authentication

#### STAGING_MASTER_ADMIN_API_KEY
- **Name**: `STAGING_MASTER_ADMIN_API_KEY`
- **Value**: `ai-aas__HYQk1SQgY4P_f2aMjYM39zL9NAxG63tcHn_Gx4If3M`
- **Purpose**: Staging environment E2E test authentication

#### LINODE_OBJECT_STORAGE_ACCESS_KEY
- **Name**: `LINODE_OBJECT_STORAGE_ACCESS_KEY`
- **Value**: `9MM84F0AST57F5PTSZNN`
- **Purpose**: Upload E2E test results to object storage

#### LINODE_OBJECT_STORAGE_SECRET_KEY
- **Name**: `LINODE_OBJECT_STORAGE_SECRET_KEY`
- **Value**: `79zz7O0q5K3hhnvoihOVWLuChc7UzmjRgefWzA4f`
- **Purpose**: Upload E2E test results to object storage

#### LINODE_TOKEN
- **Name**: `LINODE_TOKEN`
- **Value**: `1aa4f7e4dd79e3f6da55d99c275fd8d6923c4d352ed439bc0645bb4e1a441f34`
- **Purpose**: Infrastructure provisioning and Terraform operations

### 3. Add Optional Secrets (Recommended)

These secrets enable additional functionality but are not strictly required for E2E tests to pass:

#### GRAFANA_API_KEY
- **Name**: `GRAFANA_API_KEY`
- **Value**: Generate from https://grafana.dev.otherjamesbrown.com
- **Purpose**: Post test run annotations to Grafana dashboards
- **Generation Steps**:
  1. Log into Grafana at https://grafana.dev.otherjamesbrown.com
  2. Go to Administration → API Keys
  3. Click "Add API key"
  4. Name: "GitHub Actions E2E"
  5. Role: "Editor"
  6. Time to live: Leave blank (never expires) or set to 1 year
  7. Click "Add" and copy the key value

#### GRAFANA_URL
- **Name**: `GRAFANA_URL`
- **Value**: `https://grafana.dev.otherjamesbrown.com`
- **Purpose**: Grafana instance URL for posting annotations

#### DEV_KUBECONFIG
- **Name**: `DEV_KUBECONFIG`
- **Value**: Contents of `secrets/kubeconfigs/kubeconfig-development.yaml` (NOT base64 encoded)
- **Purpose**: Failure mode tests (different from base64 version)
- **How to get the value**:
  ```bash
  cat /home/dev/worktrees/develop/secrets/kubeconfigs/kubeconfig-development.yaml
  ```
  Copy the entire output and paste as the secret value.

#### DEV_API_ENDPOINT
- **Name**: `DEV_API_ENDPOINT`
- **Value**: `https://router.api.ai-aas.dev`
- **Purpose**: Development API endpoint for failure mode tests

#### DEV_ADMIN_API_KEY
- **Name**: `DEV_ADMIN_API_KEY`
- **Value**: `VXDzIauNfwRdmUDowO37plULPXbf1fUBr-69oqSEWEA` (same as MASTER_ADMIN_API_KEY)
- **Purpose**: Admin API key for failure mode tests

### 4. Verify Secrets Are Set

After adding the secrets, verify they were added correctly:

```bash
gh secret list --repo otherjamesbrown/ai-aas
```

You should see all the secrets you added in the list. Note: You cannot view the secret values, only verify they exist.

Or run the validation script again:

```bash
cd /home/dev/worktrees/develop
./scripts/ci/validate-secrets.py
```

All critical secrets should show green checkmarks (✓).

### 5. Test the Fix

Trigger the E2E tests manually to verify they now work:

```bash
gh workflow run nightly-e2e.yml -f environment=development -f test_tier=smoke
```

Monitor the run:

```bash
# List recent runs
gh run list --workflow=nightly-e2e.yml -L 5

# Watch the most recent run
gh run watch
```

The test should now pass without 401 Unauthorized errors.

## Troubleshooting

### "Secret already exists" Error

If you get an error that the secret already exists but has the wrong value:
1. Go to the secrets page
2. Find the secret in the list
3. Click "Update" next to it
4. Enter the new value
5. Click "Update secret"

### "Cannot read secret value" Message

This is expected behavior. GitHub never displays secret values after they are set, for security reasons. This is why we provide the exact values in this document.

### E2E Tests Still Failing with 401 Error

1. Verify the secret name exactly matches (case-sensitive): `MASTER_ADMIN_API_KEY`
2. Check that the secret value doesn't have extra spaces or newlines
3. Try updating the secret (delete and recreate if needed)
4. Check the workflow file is using the correct secret name: `.github/workflows/nightly-e2e.yml` line 129

### How to Delete a Secret

If you need to start over:
1. Go to https://github.com/otherjamesbrown/ai-aas/settings/secrets/actions
2. Find the secret in the list
3. Click "Delete" next to it
4. Confirm deletion
5. Add it again following the steps above

## Security Notes

- **Do not commit these values to git** - They are already in `secrets/env/.env` which is encrypted with git-crypt
- **Do not share secret values** - Only add them to GitHub secrets
- **Rotate master admin keys every 90 days** - Update both `secrets/env/.env` and GitHub secrets
- **Use unique keys per environment** - Development and staging have different master admin keys

## Related Documentation

- [GitHub Secrets Setup Guide](github-secrets-setup.md) - Complete reference for all secrets
- [GitHub Actions Guide](github-actions-guide.md) - Workflow best practices
- [Environment Access](environment-access.md) - Cluster access and credentials
