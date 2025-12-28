# Analytics Service End-to-End Tests

## Purpose

These tests validate the complete token usage tracking pipeline from ingestion to query:

1. **UsageRecord** published to Kafka (simulating api-router-service)
2. **analytics-service Kafka consumer** ingests the record
3. Record is stored in **TimescaleDB usage_events** table
4. **Rollup workers** aggregate data into hourly/daily rollups
5. Data can be queried via **REST API**

## Test Coverage

### TestUsagePipelineE2E
Complete end-to-end flow testing:
- Publishes a UsageRecord to Kafka
- Verifies consumer ingestion into usage_events table
- Verifies rollup aggregation into hourly and daily rollups
- Queries data via REST API
- Validates totals match expected values (tokens, invocations, cost)

### TestUsagePipelineDeduplication
Tests duplicate record handling:
- Publishes the same UsageRecord twice
- Verifies only one record is stored (deduplication by record_id + organization_id)

### TestUsagePipelineMultipleRecords
Tests batch ingestion and aggregation:
- Publishes 10 different records for the same org/API key
- Verifies all records are ingested
- Verifies rollup aggregation produces correct totals

## Running the Tests

### Prerequisites

- **Docker**: Required for testcontainers (Kafka and TimescaleDB)
- **Go 1.23+**: For running tests

### Run All E2E Tests

```bash
cd /path/to/worktree
go test -v -timeout=5m github.com/otherjamesbrown/ai-aas/services/analytics-service/test/e2e
```

### Run Specific Test

```bash
go test -v -timeout=5m github.com/otherjamesbrown/ai-aas/services/analytics-service/test/e2e -run TestUsagePipelineE2E
```

### Troubleshooting

**Migration Path Issues**: The tests use `go.work` as a marker to find the project root. If tests fail with "migrations directory does not exist", verify:
- You're running from a git worktree with `go.work` in the root
- The path `/db/migrations/analytics` exists relative to `go.work`

**Docker Issues**: If testcontainers fail to start:
- Ensure Docker is running: `docker ps`
- Check Docker permissions: `docker run hello-world`

**Timeout Issues**: If tests timeout:
- Kafka and TimescaleDB containers can take 10-30s to start
- Consumer batching adds 5-7s delay for ingestion
- Rollup workers run every 5s (test configuration)
- Total test time: ~30-60s per test

## Test Architecture

### Infrastructure (testcontainers)
- **TimescaleDB** (postgres): Usage events storage + continuous aggregates
- **Kafka** (confluent): Message broker for usage records

### Components Tested
- **ingestion.Consumer**: Kafka consumer with batching and deduplication
- **aggregation.Worker**: Rollup workers for hourly/daily aggregation
- **api.UsageHandler**: REST API for querying usage data
- **postgres.Store**: Database operations

### Test Data Flow

```
UsageRecord (JSON)
  ↓
Kafka Topic: usage.records.v1
  ↓
analytics-service Consumer (batch: 10, timeout: 5s)
  ↓
TimescaleDB: usage_events table (deduplication by record_id + org_id)
  ↓
Rollup Worker (interval: 5s)
  ↓
TimescaleDB: analytics_hourly_rollups, analytics_daily_rollups
  ↓
REST API: GET /analytics/v1/orgs/{orgId}/apikeys/{apiKeyId}/usage
  ↓
Response: {series: [...], totals: {...}, freshness: {...}}
```

## Schema Validation

The test UsageRecord struct matches the schema from api-router-service:
- **Required fields**: record_id, request_id, organization_id, api_key_id, timestamp, model, backend_id, tokens_input, tokens_output, cost_usd, latency_ms, limit_state, decision_reason
- **Optional fields**: retry_count, trace_id, span_id, metadata

## Future Enhancements

- [ ] Add Redis testcontainer for freshness cache testing
- [ ] Add tests for error handling (malformed records, Kafka down, DB down)
- [ ] Add tests for high-volume scenarios (thousands of records)
- [ ] Add tests for query filtering (by model, time range, granularity)
- [ ] Add performance benchmarks
- [ ] Add tests for export job integration

## Related Beads

- `aas-4tj9`: End-to-end testing of token usage pipeline (this implementation)
- `aas-7pvq`: Replace RabbitMQ consumer with Kafka consumer
- `aas-emrx`: Align UsageRecord schema between api-router and analytics-service

## Documentation

- **API Patterns**: `/docs/go-services/api-patterns.md`
- **Testing Guide**: `/docs/go-services/testing-guide.md`
- **Analytics Architecture**: `/services/analytics-service/README.md`
