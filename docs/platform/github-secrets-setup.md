# GitHub Secrets Setup Guide

---
last_updated: 2025-12-31
document_type: reference
---

This document provides a comprehensive list of all GitHub repository secrets required for CI/CD workflows to function correctly.

## Overview

GitHub Actions workflows reference secrets using the `${{ secrets.SECRET_NAME }}` syntax. Secrets are encrypted environment variables stored in GitHub repository settings. They are used for:

- API keys and authentication tokens
- Cloud provider credentials
- Kubernetes cluster access
- External service credentials

**Security Note**: Secret values cannot be read via GitHub API or CLI after they are set. This is by design for security.

## Setting Secrets

### Via GitHub Web UI

1. Navigate to: `https://github.com/otherjamesbrown/ai-aas/settings/secrets/actions`
2. Click "New repository secret"
3. Enter the secret name (exactly as shown below)
4. Enter the secret value
5. Click "Add secret"

### Verification

To check if a secret exists (but not its value):
```bash
gh secret list --repo otherjamesbrown/ai-aas
```

## Required Secrets

### E2E Test Authentication

#### MASTER_ADMIN_API_KEY
- **Purpose**: Master admin API key for development environment E2E tests
- **Value**: `VXDzIauNfwRdmUDowO37plULPXbf1fUBr-69oqSEWEA`
- **Source**: `secrets/env/.env` line 79
- **Used in**:
  - `.github/workflows/nightly-e2e.yml` (line 129)
  - `.github/workflows/e2e.yml` (line 75)
- **Scopes**: `["*"]` (full admin access)
- **Expires**: 2026-11-23

#### STAGING_MASTER_ADMIN_API_KEY
- **Purpose**: Master admin API key for staging environment E2E tests
- **Value**: `ai-aas__HYQk1SQgY4P_f2aMjYM39zL9NAxG63tcHn_Gx4If3M`
- **Source**: `secrets/env/.env` line 101
- **Used in**: `.github/workflows/nightly-e2e.yml` (line 131)
- **Scopes**: `["*"]` (full admin access)
- **Regenerated**: 2025-12-09 (rotated for security)

### Object Storage (Test Results & Artifacts)

#### LINODE_OBJECT_STORAGE_ACCESS_KEY
- **Purpose**: Linode Object Storage access key for uploading E2E test results
- **Value**: `9MM84F0AST57F5PTSZNN`
- **Source**: `secrets/env/.env` line 41
- **Used in**:
  - `.github/workflows/nightly-e2e.yml` (lines 198, 270)
- **Region**: us-east-1
- **Bucket**: `ai-aas-models` (with `e2e-results/` prefix)

#### LINODE_OBJECT_STORAGE_SECRET_KEY
- **Purpose**: Linode Object Storage secret key for uploading E2E test results
- **Value**: `79zz7O0q5K3hhnvoihOVWLuChc7UzmjRgefWzA4f`
- **Source**: `secrets/env/.env` line 42
- **Used in**:
  - `.github/workflows/nightly-e2e.yml` (lines 199, 271)

### Observability (Optional but Recommended)

#### GRAFANA_API_KEY
- **Purpose**: Grafana API key for posting E2E test run annotations
- **Value**: Generate from Grafana UI at `https://grafana.dev.otherjamesbrown.com`
- **Used in**: `.github/workflows/nightly-e2e.yml` (line 235)
- **Generation**:
  1. Log into Grafana
  2. Go to Administration → API Keys
  3. Create new API key with "Editor" role
  4. Copy the key value
- **Note**: Workflow will continue even if this secret is missing (step has `continue-on-error: true`)

#### GRAFANA_URL
- **Purpose**: Base URL for Grafana instance
- **Value**: `https://grafana.dev.otherjamesbrown.com`
- **Used in**: `.github/workflows/nightly-e2e.yml` (line 237)

### Kubernetes Cluster Access

#### DEV_KUBECONFIG_B64
- **Purpose**: Base64-encoded kubeconfig for development cluster
- **Value**: Generate from `secrets/kubeconfigs/kubeconfig-development.yaml`
- **Used in**: `.github/workflows/infra-availability.yml` (line 12)
- **Generation**:
  ```bash
  base64 -w 0 secrets/kubeconfigs/kubeconfig-development.yaml
  ```

#### DEV_KUBE_CONTEXT
- **Purpose**: Kubernetes context name for development cluster
- **Value**: `lke258421-ctx` (check your kubeconfig for exact value)
- **Used in**: `.github/workflows/infra-availability.yml` (line 13)
- **Verification**:
  ```bash
  kubectl config get-contexts --kubeconfig=secrets/kubeconfigs/kubeconfig-development.yaml
  ```

#### PROD_KUBECONFIG_B64
- **Purpose**: Base64-encoded kubeconfig for production cluster
- **Value**: Generate from `secrets/kubeconfigs/kubeconfig-production.yaml`
- **Used in**: GitHub workflows requiring production access
- **Generation**:
  ```bash
  base64 -w 0 secrets/kubeconfigs/kubeconfig-production.yaml
  ```

#### PROD_KUBE_CONTEXT
- **Purpose**: Kubernetes context name for production cluster
- **Value**: Check your production kubeconfig for exact value
- **Used in**: GitHub workflows requiring production access

### Container Registry

#### GHCR_TOKEN
- **Purpose**: GitHub Container Registry (ghcr.io) authentication token
- **Value**: GitHub Personal Access Token (PAT) with `write:packages` scope
- **Used in**:
  - `.github/workflows/service-ci.yml` (line 192)
  - `.github/workflows/web-portal.yml` (line 167)
  - `.github/workflows/ai-model-operator-ci.yml` (line 145)
- **Generation**:
  1. GitHub Settings → Developer settings → Personal access tokens → Tokens (classic)
  2. Generate new token with `write:packages` and `read:packages` scopes
  3. Copy the token value

### Infrastructure Provisioning

#### LINODE_TOKEN
- **Purpose**: Linode API token for infrastructure provisioning and CI/CD
- **Value**: `1aa4f7e4dd79e3f6da55d99c275fd8d6923c4d352ed439bc0645bb4e1a441f34`
- **Source**: `secrets/env/.env` line 62
- **Used in**:
  - `.github/workflows/dev-environment-ci.yml` (lines 84, 150)
  - `.github/workflows/infra-terraform.yml` (line 14)
- **Scopes**: Full Linode API access

### Failure Mode Tests (Additional)

#### DEV_KUBECONFIG
- **Purpose**: Non-base64 encoded kubeconfig for development cluster
- **Value**: Contents of `secrets/kubeconfigs/kubeconfig-development.yaml`
- **Used in**: `.github/workflows/failure-mode-tests.yml` (lines 129, 206)
- **Note**: This is different from `DEV_KUBECONFIG_B64` (not base64 encoded)

#### DEV_API_ENDPOINT
- **Purpose**: Development environment API endpoint URL
- **Value**: `https://router.api.ai-aas.dev`
- **Used in**: `.github/workflows/failure-mode-tests.yml` (lines 152, 156, 223, 227)

#### DEV_ADMIN_API_KEY
- **Purpose**: Admin API key for development environment failure mode tests
- **Value**: Same as `MASTER_ADMIN_API_KEY` (for consistency)
- **Used in**: `.github/workflows/failure-mode-tests.yml` (lines 153, 157, 224, 228)

### API Router Validation (Additional)

#### API_ROUTER_DEV_TEST_KEY
- **Purpose**: Test API key for API router validation in development
- **Value**: Generate using `ai-aas-cli apikey create` in development
- **Used in**: `.github/workflows/api-router-validation.yml` (line 88)

#### API_ROUTER_STAGING_TEST_KEY
- **Purpose**: Test API key for API router validation in staging
- **Value**: Generate using `ai-aas-cli apikey create` in staging
- **Used in**: `.github/workflows/api-router-validation.yml` (line 89)

#### API_ROUTER_PROD_TEST_KEY
- **Purpose**: Test API key for API router validation in production
- **Value**: Generate using `ai-aas-cli apikey create` in production
- **Used in**: `.github/workflows/api-router-validation.yml` (line 90)

### Web Portal Environment Variables (Optional)

#### VITE_USER_ORG_SERVICE_URL_E2E
- **Purpose**: User org service URL for web portal E2E tests
- **Value**: `http://localhost:8081` (default if not set)
- **Used in**: `.github/workflows/web-portal.yml` (line 139)

#### VITE_API_BASE_URL
- **Purpose**: API base URL for web portal build
- **Used in**: `.github/workflows/web-portal.yml` (line 193)

#### VITE_OAUTH_CLIENT_ID
- **Purpose**: OAuth client ID for web portal authentication
- **Used in**: `.github/workflows/web-portal.yml` (line 194)

#### VITE_OAUTH_ISSUER_URL
- **Purpose**: OAuth issuer URL for web portal authentication
- **Used in**: `.github/workflows/web-portal.yml` (line 195)

#### VITE_OAUTH_REDIRECT_URI
- **Purpose**: OAuth redirect URI for web portal authentication
- **Used in**: `.github/workflows/web-portal.yml` (line 196)

## Secrets Checklist

Use this checklist when setting up a new repository or troubleshooting CI/CD failures:

### Critical (Blocks CI/CD)
- [ ] MASTER_ADMIN_API_KEY
- [ ] STAGING_MASTER_ADMIN_API_KEY
- [ ] LINODE_OBJECT_STORAGE_ACCESS_KEY
- [ ] LINODE_OBJECT_STORAGE_SECRET_KEY
- [ ] DEV_KUBECONFIG_B64
- [ ] DEV_KUBE_CONTEXT
- [ ] PROD_KUBECONFIG_B64
- [ ] PROD_KUBE_CONTEXT
- [ ] GHCR_TOKEN
- [ ] LINODE_TOKEN

### Optional (Degrades functionality)
- [ ] GRAFANA_API_KEY
- [ ] GRAFANA_URL
- [ ] DEV_KUBECONFIG (non-base64)
- [ ] DEV_API_ENDPOINT
- [ ] DEV_ADMIN_API_KEY
- [ ] API_ROUTER_DEV_TEST_KEY
- [ ] API_ROUTER_STAGING_TEST_KEY
- [ ] API_ROUTER_PROD_TEST_KEY

## Validation

After setting secrets, trigger a test workflow to verify they work:

```bash
# Trigger nightly E2E tests manually
gh workflow run nightly-e2e.yml -f environment=development -f test_tier=smoke

# Monitor the run
gh run list --workflow=nightly-e2e.yml -L 1
gh run watch <run-id>
```

## Security Best Practices

1. **Never commit secrets to git** - Always use GitHub secrets or git-crypt
2. **Rotate regularly** - Master admin keys should be rotated every 90 days
3. **Least privilege** - Use service-specific API keys where possible
4. **Audit access** - Review who has access to repository secrets periodically
5. **Monitor usage** - Check GitHub Actions logs for unauthorized secret usage

## Troubleshooting

### Secret Not Found Error

```
Error: The secret `MASTER_ADMIN_API_KEY` was not found
```

**Solution**: Verify the secret exists and name matches exactly (case-sensitive):
```bash
gh secret list --repo otherjamesbrown/ai-aas | grep MASTER_ADMIN_API_KEY
```

### Authentication Failures

```
Error: 401 Unauthorized
```

**Solution**:
1. Check if the secret value matches the expected value in `secrets/env/.env`
2. Verify the API key hasn't expired
3. Check the API key scopes include the required permissions

### Base64 Decoding Errors

```
Error: base64: invalid input
```

**Solution**: Ensure kubeconfig secrets are base64-encoded correctly:
```bash
base64 -w 0 secrets/kubeconfigs/kubeconfig-development.yaml
```

The `-w 0` flag ensures no line wrapping.

## Related Documentation

- [GitHub Actions Guide](github-actions-guide.md) - Workflow best practices
- [CI/CD Pipeline](ci-cd-pipeline.md) - Overall pipeline architecture
- [Environment Access](environment-access.md) - Cluster access and credentials
