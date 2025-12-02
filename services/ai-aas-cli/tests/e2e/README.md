# AI-AAS CLI End-to-End Tests

This directory contains end-to-end tests for the `ai-aas-cli` that run against a live development environment.

## Test Structure

```
tests/e2e/
├── README.md                    # This file
├── env.sh                       # Environment setup script
├── full_workflow_test.go        # Full workflow test (org -> user -> apikey -> inference -> metrics)
└── testdata/                    # Test data and fixtures
```

## Prerequisites

1. Access to the development Kubernetes cluster
2. The `ai-aas-cli` binary built and available
3. Environment variables configured (see below)

## Environment Setup

### Option 1: Use the env.sh script

```bash
source tests/e2e/env.sh
```

### Option 2: Manual setup

```bash
# Load from secrets (git-crypt must be unlocked)
export AI_AAS_API_KEY=$(grep MASTER_ADMIN_API_KEY secrets/env/.env | cut -d'=' -f2)
export AI_AAS_USER_ORG_ENDPOINT=https://user-org.dev.otherjamesbrown.com
export AI_AAS_INFERENCE_ENDPOINT=https://api.dev.otherjamesbrown.com
export AI_AAS_ANALYTICS_ENDPOINT=https://analytics.dev.otherjamesbrown.com
export AI_AAS_TLS_INSECURE=false  # Let's Encrypt certificates
```

## Running Tests

### Run all e2e tests

```bash
cd services/ai-aas-cli
go test -v -tags=e2e ./tests/e2e/...
```

### Run specific test

```bash
go test -v -tags=e2e ./tests/e2e/... -run TestFullWorkflow
```

### Run with verbose output

```bash
go test -v -tags=e2e ./tests/e2e/... -v -count=1
```

## Test Coverage

The e2e tests cover the following workflow:

1. **Organization Creation**: Create a new test organization
2. **User Invitation**: Invite an admin user to the organization
3. **API Key Generation**: Create an API key with full model permissions
4. **Model Discovery**: List available models via the inference API
5. **Inference Request**: Send a chat completion request to the first model
6. **Metrics Verification**: Query usage analytics to verify the request was logged

## Cleanup

Tests clean up after themselves by:
- Deleting the test organization (cascades to users and API keys)

If a test fails mid-execution, you may need to manually clean up:

```bash
ai-aas-cli org delete --org-id <test-org-slug> --force
```

## Troubleshooting

### Tests fail with "service unavailable"
- Check that all services are running in the development cluster
- Verify the endpoints are correct in env.sh

### Authentication errors
- Ensure the MASTER_ADMIN_API_KEY is correct
- Check that git-crypt is unlocked: `git-crypt status`

### TLS errors
- The development environment uses Let's Encrypt certificates (AI_AAS_TLS_INSECURE=false)
- If testing locally with self-signed certs, set `AI_AAS_TLS_INSECURE=true`

### No models available
- Check that at least one vLLM deployment is running
- Verify the API router can reach the model backends
