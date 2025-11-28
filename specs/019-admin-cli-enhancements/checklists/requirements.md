# Requirements Checklist: Admin CLI Enhancements

**Spec**: `specs/019-admin-cli-enhancements/spec.md`

## Functional Requirements

### Budget Management
- [ ] **FR-001**: `budget list` displays organization budget configuration
- [ ] **FR-002**: `budget set` updates budget with dry-run/confirm support
- [ ] **FR-003**: `budget alerts` configures threshold alerts

### API Key Management
- [ ] **FR-004**: `apikey rotate` regenerates key with dry-run/confirm
- [ ] **FR-005**: `apikey update` modifies scopes without rotation

### Usage Queries
- [ ] **FR-006**: `usage query` retrieves filtered usage data
- [ ] **FR-007**: `usage summary` shows aggregated totals

### Common Features
- [ ] **FR-008**: All commands support --format, --verbose, --quiet flags
- [ ] **FR-009**: All state-changing commands emit audit logs

## Non-Functional Requirements

### Performance
- [ ] **NFR-001**: Budget operations ≤2s response time
- [ ] **NFR-002**: Key rotation ≤3s response time
- [ ] **NFR-003**: Usage queries ≤5s for ≤10k records

### Security
- [ ] **NFR-004**: New keys displayed once, masked in logs
- [ ] **NFR-005**: Authentication required for all operations
- [ ] **NFR-006**: Audit logs capture user identity

### Reliability
- [ ] **NFR-007**: Retry logic with exponential backoff
- [ ] **NFR-008**: Consistent exit codes (0=success, 1=error, 2=validation)

## Testing Requirements

- [ ] Unit tests with ≥80% coverage
- [ ] Integration tests for all commands
- [ ] Error scenario tests
- [ ] Output format tests (table, json, csv)

## Documentation Requirements

- [ ] admin-cli.md updated with new commands
- [ ] ui-pages.md updated with CLI parity
- [ ] Help text for all commands
- [ ] Examples in documentation

## API Endpoint Status (Verified 2025-11-28)

| Endpoint | Service | Status | Action |
|----------|---------|--------|--------|
| `GET /v1/orgs/{id}` | user-org-service | **EXISTS** | Use as-is |
| `GET /v1/budgets/{orgId}/status` | user-org-service | **EXISTS** | Use as-is |
| `PATCH /v1/orgs/{id}` | user-org-service | **EXISTS** | Use as-is |
| `POST /v1/apikeys/{id}/rotate` | user-org-service | **STUB** | Implement handler |
| `PATCH /v1/apikeys/{id}` | user-org-service | **STUB** | Implement handler |
| `GET /analytics/v1/orgs/{orgId}/usage` | analytics-service | **EXISTS** | Use as-is |
| `POST /v1/orgs/{id}/budget-alerts` | user-org-service | **MISSING** | Phase 2 - new endpoint |

## Phase Sign-off

### Phase 1a (CLI Only)
- [ ] Budget list/set commands working
- [ ] Usage query/summary commands working
- [ ] Unit tests passing
- [ ] Integration tests passing

### Phase 1b (Backend + CLI)
- [ ] API key rotate handler implemented
- [ ] API key update handler implemented
- [ ] CLI commands working
- [ ] Unit tests passing
- [ ] Integration tests passing

### Phase 2 (New Backend)
- [ ] Budget alerts endpoint implemented
- [ ] Budget alerts CLI working
- [ ] Documentation updated
- [ ] All tests passing

## Final Sign-off

- [ ] All phases complete
- [ ] Code reviewed and approved
- [ ] Merged to main
