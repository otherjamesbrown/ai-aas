# Feature Specification: Admin CLI Enhancements

**Feature Branch**: `019-admin-cli-enhancements`
**Created**: 2025-11-28
**Status**: Draft
**Input**: Add budget management, API key rotation, and usage query commands to the admin-cli for parity with web portal functionality.

## Summary

Extend the admin-cli with new commands for:
1. **Budget Management**: View and set organization budget limits and alerts
2. **API Key Rotation/Update**: Rotate keys and update scopes without recreation
3. **Usage Queries**: Query usage data with filters (beyond full export)

These additions provide CLI parity with the web portal for automation, scripting, and CI/CD workflows.

## Scope

### In Scope

- `admin-cli budget` command group (list, set, alerts)
- `admin-cli apikey rotate` and `admin-cli apikey update` subcommands
- `admin-cli usage query` and `admin-cli usage summary` subcommands
- Unit tests for all new commands
- Integration tests against test fixtures
- Documentation updates

### Out of Scope

- New backend API endpoints (commands use existing APIs)
- Web portal changes
- Support impersonation (requires interactive consent flow)
- Visual dashboards/charts

## User Scenarios & Testing

### User Story 1 (US-001) - Budget Management (Priority: P1)

As an operator, I can view and set organization budget limits via CLI for automation and batch operations.

**Why this priority**: Budget controls are critical for cost management and are currently portal-only, blocking automation workflows.

**Independent Test**: Can be tested using existing organization fixtures with budget fields.

**Acceptance Scenarios**:

1. **[Primary]** **Given** an organization with a budget limit, **When** I run `admin-cli budget list --org-id <id>`, **Then** CLI displays current budget limit, usage, and remaining allowance in table/json format.
2. **[Primary]** **Given** an organization, **When** I run `admin-cli budget set --org-id <id> --monthly-limit 1000000 --dry-run`, **Then** CLI shows planned changes without applying them.
3. **[Primary]** **Given** the same scenario, **When** I run `admin-cli budget set --org-id <id> --monthly-limit 1000000 --confirm`, **Then** budget is updated and audit log is emitted.
4. **[Alternate]** **Given** alert configuration, **When** I run `admin-cli budget alerts --org-id <id> --threshold 80 --recipients ops@example.com`, **Then** alert is configured and confirmation displayed.
5. **[Exception]** **Given** an invalid budget value (negative), **When** I attempt to set it, **Then** CLI returns validation error with clear message.

---

### User Story 2 (US-002) - API Key Rotation (Priority: P1)

As an operator, I can rotate API keys and update scopes via CLI for automated key rotation policies.

**Why this priority**: Key rotation is a security best practice and is currently portal-only, blocking security automation.

**Independent Test**: Can be tested using existing API key fixtures.

**Acceptance Scenarios**:

1. **[Primary]** **Given** an existing API key, **When** I run `admin-cli apikey rotate --key-id <id> --dry-run`, **Then** CLI shows that key will be rotated with new value generated.
2. **[Primary]** **Given** the same scenario, **When** I run `admin-cli apikey rotate --key-id <id> --confirm`, **Then** new key is generated, old key is invalidated, new key value is displayed securely, and audit log is emitted.
3. **[Primary]** **Given** an existing API key, **When** I run `admin-cli apikey update --key-id <id> --scopes "read,write" --confirm`, **Then** scopes are updated without changing the key value.
4. **[Alternate]** **Given** key rotation with `--extend-expiry`, **When** I run rotate, **Then** expiry is extended by configured duration.
5. **[Exception]** **Given** an already-revoked key, **When** I attempt to rotate, **Then** CLI returns clear error indicating key is revoked.

---

### User Story 3 (US-003) - Usage Queries (Priority: P2)

As an operator, I can query usage data with filters via CLI for monitoring and automation without full exports.

**Why this priority**: Enables scripted monitoring and alerting workflows; more efficient than full exports for quick checks.

**Independent Test**: Can be tested using analytics-service fixtures.

**Acceptance Scenarios**:

1. **[Primary]** **Given** usage data exists, **When** I run `admin-cli usage query --org-id <id> --from 2025-01-01 --to 2025-01-31`, **Then** CLI displays usage records in table/json format with totals.
2. **[Primary]** **Given** the same scenario, **When** I add `--model gpt-4`, **Then** results are filtered to that model only.
3. **[Primary]** **Given** usage data, **When** I run `admin-cli usage summary --org-id <id>`, **Then** CLI displays aggregated totals (tokens, requests, estimated cost) for current period.
4. **[Alternate]** **Given** `--period week`, **When** I run usage summary, **Then** results are aggregated by week.
5. **[Exception]** **Given** no usage data for the period, **When** I query, **Then** CLI displays empty state with guidance.

---

### Edge Cases

1. **Concurrent Budget Updates**: CLI surfaces 409 Conflict from API with retry guidance.
2. **Key Rotation During Active Use**: Rotation invalidates old key immediately; CLI warns about potential disruption.
3. **Large Usage Queries**: CLI implements pagination for large result sets (>1000 records).
4. **Invalid Date Ranges**: CLI validates date ranges before API call (from < to).

## Requirements

### Functional Requirements

- **FR-001**: Provide `admin-cli budget list` command to display organization budget configuration and current usage.
- **FR-002**: Provide `admin-cli budget set` command with `--monthly-limit`, `--currency`, `--dry-run`, `--confirm` flags.
- **FR-003**: Provide `admin-cli budget alerts` command to configure threshold alerts with recipients.
- **FR-004**: Provide `admin-cli apikey rotate` command with `--dry-run`, `--confirm`, `--extend-expiry` flags.
- **FR-005**: Provide `admin-cli apikey update` command with `--scopes` flag to modify key permissions.
- **FR-006**: Provide `admin-cli usage query` command with `--org-id`, `--from`, `--to`, `--model` filters.
- **FR-007**: Provide `admin-cli usage summary` command with `--org-id`, `--period` (day/week/month) aggregation.
- **FR-008**: All commands support `--format` (table/json/csv), `--verbose`, `--quiet` flags per existing patterns.
- **FR-009**: All commands emit audit logs for state-changing operations.

### Non-Functional Requirements

**Performance:**
- **NFR-001**: Budget operations complete in ≤2 seconds under normal conditions.
- **NFR-002**: Key rotation completes in ≤3 seconds including key generation.
- **NFR-003**: Usage queries return in ≤5 seconds for datasets up to 10k records.

**Security:**
- **NFR-004**: New key values displayed only once at rotation; masked in logs.
- **NFR-005**: All operations require authentication via existing auth mechanism.
- **NFR-006**: Audit logs capture all state changes with user identity.

**Reliability:**
- **NFR-007**: Commands implement retry logic with exponential backoff per existing patterns.
- **NFR-008**: Validation errors return exit code 2; service errors return exit code 1.

## Architecture Notes

### API Dependencies

| Command | API Endpoint | Service | Status |
|---------|-------------|---------|--------|
| `budget list` | `GET /v1/orgs/{id}` + `GET /v1/budgets/{orgId}/status` | user-org-service | **EXISTS** |
| `budget set` | `PATCH /v1/orgs/{id}` | user-org-service | **EXISTS** |
| `budget alerts` | `POST /v1/orgs/{id}/budget-alerts` | user-org-service | **MISSING** |
| `apikey rotate` | `POST /v1/apikeys/{id}/rotate` | user-org-service | **STUB (501)** |
| `apikey update` | `PATCH /v1/apikeys/{id}` | user-org-service | **STUB (501)** |
| `usage query` | `GET /analytics/v1/orgs/{orgId}/usage` | analytics-service | **EXISTS** |
| `usage summary` | Same as query (use `totals` field) | analytics-service | **EXISTS** |

### Implementation Phases

Based on API availability, implementation is split into phases:

**Phase 1a - CLI Only (APIs exist):**
- `budget list` - Combine org details + budget status endpoints
- `budget set` - Use PATCH org with `budgetPolicyId`
- `usage query` - Use analytics usage endpoint with filters
- `usage summary` - Use same endpoint, display `totals` field

**Phase 1b - Backend + CLI (stubs need implementation):**
- Implement `POST /v1/apikeys/{id}/rotate` handler in user-org-service
- Implement `PATCH /v1/apikeys/{id}` handler in user-org-service
- Add `apikey rotate` and `apikey update` CLI commands

**Phase 2 - New Backend Work:**
- Design and implement `POST /v1/orgs/{id}/budget-alerts` endpoint
- Add `budget alerts` CLI command

### Code Structure

```
services/admin-cli/internal/commands/
├── budget.go          # NEW: budget list, set, alerts
├── apikey.go          # MODIFY: add rotate, update subcommands
└── usage.go           # NEW: usage query, summary

services/admin-cli/internal/client/
├── userorg/
│   └── budget.go      # NEW: budget API client
└── analytics/
    └── usage.go       # NEW: usage API client

services/admin-cli/test/integration/
├── budget_test.go     # NEW
├── apikey_rotate_test.go  # NEW
└── usage_query_test.go    # NEW
```

## Success Criteria

- **SC-001**: All new commands pass unit tests with ≥80% coverage.
- **SC-002**: Integration tests verify end-to-end flows against test fixtures.
- **SC-003**: Commands follow existing CLI patterns (flags, output formats, exit codes).
- **SC-004**: Documentation updated in `docs/technical/services/admin-cli.md`.
- **SC-005**: UI pages doc (`docs/platform/ui-pages.md`) updated to reflect CLI parity.

## Assumptions

- User-org-service and analytics-service APIs support the required operations (may need minor additions).
- Existing authentication mechanism works for new commands.
- Budget data is stored on organization entity (`budget_limit_tokens` field exists).
- Analytics-service has query endpoints (not just export).

## Traceability Matrix

| User Story | Functional Requirements | Non-Functional Requirements | Success Criteria |
|------------|-------------------------|-----------------------------|------------------|
| US-001 | FR-001, FR-002, FR-003, FR-008, FR-009 | NFR-001, NFR-005, NFR-006, NFR-007, NFR-008 | SC-001, SC-002, SC-003, SC-004 |
| US-002 | FR-004, FR-005, FR-008, FR-009 | NFR-002, NFR-004, NFR-005, NFR-006, NFR-007, NFR-008 | SC-001, SC-002, SC-003, SC-004 |
| US-003 | FR-006, FR-007, FR-008 | NFR-003, NFR-005, NFR-007, NFR-008 | SC-001, SC-002, SC-003, SC-004 |
