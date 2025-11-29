# Implementation Tasks: Admin CLI Enhancements

**Spec**: `specs/019-admin-cli-enhancements/spec.md`
**Branch**: `019-admin-cli-enhancements`

## Task Overview

| ID | Task | Phase | Priority | Estimate | Dependencies | API Status |
|----|------|-------|----------|----------|--------------|------------|
| T-001 | Budget list/set commands | 1a | P1 | Medium | - | **EXISTS** |
| T-002 | Usage query command | 1a | P1 | Medium | - | **EXISTS** |
| T-003 | Usage summary command | 1a | P1 | Small | T-002 | **EXISTS** |
| T-004 | API key rotate backend | 1b | P1 | Medium | - | **STUB** |
| T-005 | API key update backend | 1b | P1 | Small | T-004 | **STUB** |
| T-006 | API key rotate/update CLI | 1b | P1 | Medium | T-004, T-005 | - |
| T-007 | Budget alerts backend | 2 | P2 | Medium | - | **MISSING** |
| T-008 | Budget alerts CLI | 2 | P2 | Small | T-007 | - |
| T-009 | Unit tests | All | P1 | Medium | Per phase | - |
| T-010 | Integration tests | All | P1 | Medium | Per phase | - |
| T-011 | Documentation updates | All | P2 | Small | Per phase | - |

### Phase Summary

- **Phase 1a**: CLI commands using existing APIs (budget list/set, usage query/summary)
- **Phase 1b**: Backend implementation for API key stubs + CLI commands
- **Phase 2**: New budget alerts endpoint + CLI command

---

## T-001: Budget List/Set Commands (Phase 1a)

**Priority**: P1 | **Estimate**: Medium | **API Status**: EXISTS

### Description
Implement `admin-cli budget` command group with `list` and `set` subcommands using existing APIs.

### Files to Create/Modify

```
services/admin-cli/internal/commands/budget.go       # NEW
services/admin-cli/internal/client/userorg/budget.go # NEW
services/admin-cli/cmd/admin-cli/main.go            # MODIFY (add command)
```

### Implementation Details

#### budget.go Command Structure

```go
// BudgetCommand creates the budget command group
func BudgetCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "budget",
        Short: "Manage organization budgets",
        Long:  "View and configure organization budget limits",
    }
    cmd.AddCommand(budgetListCommand())
    cmd.AddCommand(budgetSetCommand())
    // budgetAlertsCommand() - Phase 2, requires new backend endpoint
    return cmd
}
```

#### Subcommands

**budget list**
```bash
admin-cli budget list --org-id <id> [--format table|json|csv]
```
- Displays: budget_limit_tokens, current_usage, remaining, status
- Uses: `GET /v1/orgs/{id}` + `GET /v1/budgets/{orgId}/status`
- Combine responses to show complete budget picture

**budget set**
```bash
admin-cli budget set --org-id <id> --monthly-limit <tokens> [--dry-run] [--confirm]
```
- Uses: `PATCH /v1/orgs/{id}` with `budgetPolicyId` field
- Requires `--confirm` for changes (or `--dry-run` for preview)

### API Client Implementation

```go
// internal/client/userorg/budget.go
func (c *Client) GetBudgetStatus(ctx context.Context, orgID string) (*BudgetStatus, error) {
    resp, err := c.httpClient.Get(fmt.Sprintf("%s/v1/budgets/%s/status", c.baseURL, orgID))
    // ...
}

func (c *Client) UpdateOrgBudget(ctx context.Context, orgID string, budgetPolicyID string) error {
    body := map[string]any{"budgetPolicyId": budgetPolicyID}
    // PATCH /v1/orgs/{orgID}
}
```

### Acceptance Criteria
- [ ] `budget list` shows current budget configuration from both endpoints
- [ ] `budget set` updates budget with dry-run support
- [ ] All operations emit audit logs
- [ ] Follows existing command patterns (flags, output, errors)

---

## T-002: Usage Query Command (Phase 1a)

**Priority**: P1 | **Estimate**: Medium | **API Status**: EXISTS

### Description
Implement `admin-cli usage query` command using existing analytics API.

### Files to Create/Modify

```
services/admin-cli/internal/commands/usage.go            # NEW
services/admin-cli/internal/client/analytics/client.go   # NEW
services/admin-cli/internal/client/analytics/usage.go    # NEW
services/admin-cli/cmd/admin-cli/main.go                # MODIFY
```

### Implementation Details

#### Command Structure

```bash
admin-cli usage query \
  --org-id <id> \
  --from 2025-01-01 \
  --to 2025-01-31 \
  [--model <model-id>] \
  [--granularity day|hour] \
  [--format table|json|csv]
```

**API Endpoint**: `GET /analytics/v1/orgs/{orgId}/usage`
- Query params: `start`, `end`, `granularity`, `modelId`
- Response includes `series` (data points) and `totals`

#### Client Implementation

```go
// internal/client/analytics/usage.go
type UsageQueryParams struct {
    OrgID       string
    Start       time.Time
    End         time.Time
    Granularity string // "hour" or "day"
    ModelID     string // optional
}

func (c *Client) QueryUsage(ctx context.Context, params UsageQueryParams) (*UsageResponse, error) {
    url := fmt.Sprintf("%s/analytics/v1/orgs/%s/usage", c.baseURL, params.OrgID)
    // Add query params: start, end, granularity, modelId
}
```

#### Date Validation

```go
func validateDateRange(from, to string) error {
    fromDate, err := time.Parse("2006-01-02", from)
    if err != nil {
        return errors.NewValidationError("invalid 'from' date format, use YYYY-MM-DD")
    }
    toDate, err := time.Parse("2006-01-02", to)
    if err != nil {
        return errors.NewValidationError("invalid 'to' date format, use YYYY-MM-DD")
    }
    if fromDate.After(toDate) {
        return errors.NewValidationError("'from' date must be before 'to' date")
    }
    return nil
}
```

### Acceptance Criteria
- [ ] Query returns filtered usage data
- [ ] Date range validation works
- [ ] Model filter works
- [ ] Output formats (table, json, csv) work
- [ ] Empty results handled gracefully

---

## T-003: Usage Summary Command (Phase 1a)

**Priority**: P1 | **Estimate**: Small | **API Status**: EXISTS

### Description
Implement `admin-cli usage summary` command using the `totals` field from existing usage API.

### Files to Modify

```
services/admin-cli/internal/commands/usage.go  # MODIFY (add summary subcommand)
```

### Implementation Details

```bash
admin-cli usage summary \
  --org-id <id> \
  [--period day|week|month] \
  [--format table|json]
```

**Implementation**: Uses same API as `usage query` but only displays the `totals` field.

**Output Example:**
```
Organization: acme-corp
Period: 2025-01 (month)
─────────────────────────
Total Requests:  15,234
Total Tokens:    2,456,789
  - Input:       1,234,567
  - Output:      1,222,222
Estimated Cost:  $123.45
```

### Acceptance Criteria
- [ ] Summary shows aggregated totals
- [ ] Period parameter calculates date range automatically
- [ ] Handles empty data gracefully

---

## T-004: API Key Rotate/Update Backend (Phase 1b)

**Priority**: P1 | **Estimate**: Medium | **API Status**: STUB (501)

### Description
Implement the API key rotate and update handlers in user-org-service. Stubs exist but return 501.

### Files to Modify

```
services/user-org-service/internal/httpapi/apikeys/handlers.go  # Implement stubs
services/user-org-service/internal/storage/postgres/apikeys.go  # Add rotate logic
```

### Implementation Details

**Rotate Handler** (`POST /v1/apikeys/{id}/rotate`):
1. Validate key exists and is not revoked
2. Generate new key value (crypto-secure random)
3. Update key record with new hash, reset expiry if requested
4. Return new key value (only time it's shown)
5. Emit audit log

**Update Handler** (`PATCH /v1/apikeys/{id}`):
1. Validate key exists
2. Update allowed fields (scopes, name, expiry)
3. Key value remains unchanged
4. Emit audit log

### Acceptance Criteria
- [ ] Rotate endpoint generates new key, invalidates old
- [ ] Update endpoint modifies metadata only
- [ ] Both emit audit logs
- [ ] Unit tests pass

---

## T-005: API Key Rotate/Update CLI (Phase 1b)

**Priority**: P1 | **Estimate**: Medium | **Dependencies**: T-004

### Description
Add `rotate` and `update` subcommands to existing `apikey` CLI command.

### Files to Modify

```
services/admin-cli/internal/commands/apikey.go          # Add subcommands
services/admin-cli/internal/client/userorg/apikey.go    # Add client methods
```

### Implementation Details

**apikey rotate**
```bash
admin-cli apikey rotate --key-id <id> [--extend-expiry] [--dry-run] [--confirm]
```

**apikey update**
```bash
admin-cli apikey update --key-id <id> --scopes "read,write" [--dry-run] [--confirm]
```

### Key Display Security

```go
fmt.Printf("New API Key: %s\n", newKey)
fmt.Println("WARNING: This key will not be shown again. Store it securely.")
```

### Acceptance Criteria
- [ ] Rotate displays new key securely (once)
- [ ] Update modifies scopes without rotation
- [ ] Both support dry-run
- [ ] Audit logs emitted

---

## T-006: Budget Alerts Backend (Phase 2)

**Priority**: P2 | **Estimate**: Medium | **API Status**: MISSING

### Description
Design and implement budget alerts endpoint in user-org-service.

### Files to Create/Modify

```
services/user-org-service/internal/httpapi/budgets/alerts.go    # NEW
services/user-org-service/internal/storage/postgres/alerts.go   # NEW
specs/005-user-org-service/contracts/user-org-service.openapi.yaml  # Update
```

### Implementation Details

**Endpoint**: `POST /v1/orgs/{id}/budget-alerts`

**Request Body**:
```json
{
  "threshold": 80,
  "recipients": ["ops@example.com"],
  "enabled": true
}
```

### Acceptance Criteria
- [ ] Endpoint creates/updates alerts
- [ ] Validation for threshold (0-100)
- [ ] Email validation for recipients
- [ ] OpenAPI spec updated

---

## T-007: Budget Alerts CLI (Phase 2)

**Priority**: P2 | **Estimate**: Small | **Dependencies**: T-006

### Description
Add `alerts` subcommand to budget CLI command.

### Files to Modify

```
services/admin-cli/internal/commands/budget.go  # Add alerts subcommand
```

### Implementation Details

```bash
admin-cli budget alerts --org-id <id> --threshold 80 --recipients ops@example.com [--disable]
```

### Acceptance Criteria
- [ ] Configure alert thresholds
- [ ] Set recipient emails
- [ ] Enable/disable alerts

---

## T-008: Unit Tests (All Phases)

**Priority**: P1 | **Estimate**: Medium

### Description
Unit tests for all new commands and handlers.

### Files to Create

**Phase 1a:**
```
services/admin-cli/internal/commands/budget_test.go
services/admin-cli/internal/commands/usage_test.go
services/admin-cli/internal/client/analytics/usage_test.go
```

**Phase 1b:**
```
services/user-org-service/internal/httpapi/apikeys/rotate_test.go
services/admin-cli/internal/commands/apikey_rotate_test.go
```

**Phase 2:**
```
services/user-org-service/internal/httpapi/budgets/alerts_test.go
```

### Acceptance Criteria
- [ ] ≥80% code coverage for new code
- [ ] All error paths tested
- [ ] Mock API responses for isolation

---

## T-009: Integration Tests (All Phases)

**Priority**: P1 | **Estimate**: Medium

### Description
Integration tests verifying end-to-end flows.

### Files to Create

```
services/admin-cli/test/integration/budget_test.go
services/admin-cli/test/integration/usage_query_test.go
services/admin-cli/test/integration/apikey_rotate_test.go
```

### Test Scenarios

**Phase 1a:**
- Budget list/set flows
- Usage query with filters

**Phase 1b:**
- API key rotation (verify old key invalidated)
- API key scope updates

### Acceptance Criteria
- [ ] Tests run against test services
- [ ] Cleanup after tests
- [ ] CI integration

---

## T-010: Documentation Updates (All Phases)

**Priority**: P2 | **Estimate**: Small

### Description
Update documentation to reflect new commands.

### Files to Modify

```
docs/technical/services/admin-cli.md   # Add new commands
docs/platform/ui-pages.md              # Update CLI parity table
```

### Acceptance Criteria
- [ ] All new commands documented with examples
- [ ] UI pages matrix updated
- [ ] Help text matches documentation

---

## Implementation Order

### Phase 1a (CLI Only - APIs Exist)
1. T-001: Budget list/set commands
2. T-002: Usage query command
3. T-003: Usage summary command
4. T-008: Unit tests (Phase 1a)
5. T-009: Integration tests (Phase 1a)

### Phase 1b (Backend + CLI)
1. T-004: API key rotate/update backend
2. T-005: API key rotate/update CLI
3. T-008: Unit tests (Phase 1b)
4. T-009: Integration tests (Phase 1b)

### Phase 2 (New Backend + CLI)
1. T-006: Budget alerts backend
2. T-007: Budget alerts CLI
3. T-008: Unit tests (Phase 2)
4. T-010: Documentation updates

---

## API Endpoint Status (Verified)

| Endpoint | Service | Status | Notes |
|----------|---------|--------|-------|
| `GET /v1/orgs/{id}` | user-org-service | **EXISTS** | Full implementation |
| `GET /v1/budgets/{orgId}/status` | user-org-service | **EXISTS** | Budget status |
| `PATCH /v1/orgs/{id}` | user-org-service | **EXISTS** | Full implementation |
| `POST /v1/apikeys/{id}/rotate` | user-org-service | **STUB** | Returns 501 |
| `PATCH /v1/apikeys/{id}` | user-org-service | **STUB** | Returns 501 |
| `GET /analytics/v1/orgs/{orgId}/usage` | analytics-service | **EXISTS** | Includes totals |
| `POST /v1/orgs/{id}/budget-alerts` | user-org-service | **MISSING** | Phase 2 |

---

## Definition of Done

- [ ] All Phase 1a commands working (budget, usage)
- [ ] All Phase 1b backend+CLI working (apikey rotate/update)
- [ ] All Phase 2 features working (budget alerts)
- [ ] Unit tests pass with ≥80% coverage
- [ ] Integration tests pass
- [ ] Documentation updated
- [ ] Code reviewed and approved
- [ ] CI pipeline passes
