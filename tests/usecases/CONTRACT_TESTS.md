# CLI-to-API Contract Tests

## Purpose

These tests prevent CLI-to-API contract drift by verifying that the CLI can successfully parse actual API responses.

## Background

**Bug**: [aas-ghda] - ai-aas-org CLI JSON unmarshal error for ListAPIKeys

The CLI expected a wrapper struct:
```json
{"api_keys": [...], "total_count": N}
```

But the API returned a raw array:
```json
[{...}, {...}]
```

This caused unmarshal errors:
```
failed to decode response: json: cannot unmarshal array into Go value of type api.ListAPIKeysResponse
```

## How Contract Tests Work

1. Execute CLI commands with `--json` flag
2. Parse the JSON output
3. If parsing succeeds, the CLI structs match what the API returns
4. If parsing fails, there's a contract mismatch

## Test Coverage

| Test | Endpoint | Verifies |
|------|----------|----------|
| `TestContract_ListUsers_JSONParsing` | `GET /v1/orgs/{id}/users` | CLI parses raw array response |
| `TestContract_ListAPIKeys_JSONParsing` | `GET /v1/orgs/{id}/api-keys` | CLI parses raw array response |
| `TestContract_ListUsers_NoWrapperStruct` | `GET /v1/orgs/{id}/users` | Output is raw array, not wrapper |
| `TestContract_ListAPIKeys_NoWrapperStruct` | `GET /v1/orgs/{id}/api-keys` | Output is raw array, not wrapper |
| `TestContract_GetUser_SingleObject` | `GET /v1/orgs/{id}/users/{userId}` | Returns single object, not array |
| `TestContract_CreateUser_ResponseFields` | `POST /v1/orgs/{id}/users` | Response has required fields |
| `TestContract_ErrorResponses_Consistent` | Error responses | CLI handles errors gracefully |
| `TestContract_EmptyLists_ReturnEmptyArrays` | List endpoints | Empty results return `[]`, not `null` |

## Running Tests

### Locally (requires live API)

```bash
# Set up environment
export AI_AAS_API_ENDPOINT="https://user-org.dev.otherjamesbrown.com"
export AI_AAS_API_KEY="<your-api-key>"
export AI_AAS_ORG_ID="<your-org-id>"

# Run contract tests
cd tests/usecases
go test -v -run TestContract
```

### CI/CD

These tests run automatically in CI when:
- API endpoint environment variables are configured
- ai-aas-org CLI binary is available

If environment is not configured, tests are skipped.

## When to Update

Add new contract tests when:

1. **New list endpoints** are added (verify raw array vs wrapper struct)
2. **Response structure changes** (verify CLI can still parse)
3. **New CLI commands** are created that call APIs
4. **Bug fixes** reveal contract mismatches (regression tests)

## Pattern: List Endpoints

user-org-service follows this pattern:

- **API returns**: `[]Item` (raw JSON array)
- **CLI internal**: Decodes to `[]Item`, wraps in `ListResponse{Items: items, TotalCount: len(items)}`
- **CLI JSON output**: `[]Item` (raw array, matching API)

Contract tests verify this round-trip works.

## Related Documentation

- [aas-ghda](../../.beads/issues/aas-ghda.md) - Bug investigation
- [aas-rmt9](../../.beads/issues/aas-rmt9.md) - Fix for ListAPIKeys
- [aas-dzo8](../../.beads/issues/aas-dzo8.md) - This test suite
- [services/ai-aas-org/internal/api/](../../services/ai-aas-org/internal/api/) - CLI API client
- [services/user-org-service/](../../services/user-org-service/) - API implementation

## Maintenance

- Run these tests before releasing CLI changes
- Update when API contracts change
- Keep bead references current
