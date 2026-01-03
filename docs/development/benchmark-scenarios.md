# Benchmark Scenarios

---
last_updated: 2026-01-03
document_type: guide
---

## Overview

Benchmark scenarios define the test configurations used by the benchmark system to evaluate model performance. Scenarios are stored in the `benchmark_scenarios` table and referenced by benchmark targets.

## Required Scenarios

The following scenarios are required for UC-BM (Benchmark) use case tests:

| Scenario | Description | Use Case |
|----------|-------------|----------|
| `standard` | Balanced load testing with moderate parameters | General benchmarking |
| `throughput` | High-concurrency stress testing | Performance limits |

## Database Schema

```sql
CREATE TABLE benchmark_scenarios (
    name TEXT PRIMARY KEY,
    description TEXT,
    version TEXT,
    config JSONB NOT NULL,
    synced_at TIMESTAMP NOT NULL
);
```

## Syncing Scenarios

Scenarios are synced to the database via the Admin API's `/v1/benchmarks/scenarios/sync` endpoint.

### Using the Script

The easiest way to seed scenarios is using the provided script:

```bash
./scripts/seed-benchmark-scenarios.sh
```

This script:
- Reads the master API key from `secrets/env/.env`
- Syncs the required scenarios to the development database
- Verifies the sync by listing scenarios

### Manual Sync via API

You can also manually sync scenarios:

```bash
# Get the master API key
MASTER_API_KEY=$(grep MASTER_ADMIN_API_KEY secrets/env/.env | cut -d'=' -f2)

# Sync scenarios
curl -X POST https://admin-api.dev.otherjamesbrown.com/v1/benchmarks/scenarios/sync \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $MASTER_API_KEY" \
  -d '{
    "scenarios": [
      {
        "name": "standard",
        "description": "Standard benchmark scenario with balanced load testing",
        "version": "1.0.0",
        "config": {
          "duration_seconds": 300,
          "concurrency": 10,
          "request_rate": 1,
          "max_tokens": 100,
          "temperature": 0.7
        }
      },
      {
        "name": "throughput",
        "description": "High-throughput benchmark scenario for stress testing",
        "version": "1.0.0",
        "config": {
          "duration_seconds": 600,
          "concurrency": 50,
          "request_rate": 10,
          "max_tokens": 200,
          "temperature": 0.8
        }
      }
    ],
    "delete_orphans": false
  }'
```

### Listing Scenarios

```bash
curl -X GET https://admin-api.dev.otherjamesbrown.com/v1/benchmarks/scenarios \
  -H "Authorization: Bearer $MASTER_API_KEY" | jq '.scenarios[] | {name, description}'
```

## Configuration Format

Each scenario has the following structure:

```json
{
  "name": "scenario-name",
  "description": "Human-readable description",
  "version": "1.0.0",
  "config": {
    "duration_seconds": 300,
    "concurrency": 10,
    "request_rate": 1,
    "max_tokens": 100,
    "temperature": 0.7
  }
}
```

### Config Fields

| Field | Type | Description |
|-------|------|-------------|
| `duration_seconds` | integer | How long the benchmark should run |
| `concurrency` | integer | Number of parallel requests |
| `request_rate` | integer | Requests per second |
| `max_tokens` | integer | Maximum tokens per response |
| `temperature` | float | LLM temperature parameter |

Note: The actual config schema is flexible (JSONB) and can contain additional fields as needed by the benchmark runner.

## Adding New Scenarios

To add a new scenario:

1. Update the `scenarios` array in `scripts/seed-benchmark-scenarios.sh`
2. Run the script to sync to development
3. For staging/production, use GitOps to sync via ai-aas-config repository

## Testing

UC-BM tests require scenarios to exist in the database. Run the seed script before running usecase tests:

```bash
# Seed scenarios
./scripts/seed-benchmark-scenarios.sh

# Run UC-BM tests
cd tests/usecases
go test -v -run TestUC_BM
```

## Authentication

The scenarios sync endpoint requires a **master API key**. Organization API keys cannot sync scenarios.

This is a platform-level operation that should only be performed by:
- CI/CD pipelines
- Platform administrators
- Development scripts

## Related Documentation

- [Admin API Service README](../../services/admin-api-service/README.md)
- [Benchmark API Endpoints](../../services/admin-api-service/README.md#benchmark-endpoints)
- [Use Case Tests](../../tests/usecases/README.md)
