# Use Case YAML Schema

This document defines the schema for use case files in the `usecases/` directory.

## File Organization

- One YAML file per feature (e.g., `benchmarks.yaml`, `users.yaml`)
- Split when file exceeds ~10 use cases or ~500 lines
- Feature prefixes define UC IDs (see Naming Conventions)

## Naming Conventions

### Feature Prefixes

| Prefix | Feature Area |
|--------|--------------|
| `UC-AUTH-` | Authentication/authorization |
| `UC-USR-` | User management |
| `UC-KEY-` | API key management |
| `UC-MDL-` | Model access |
| `UC-BM-` | Benchmarks |
| `UC-AUD-` | Audit logs |
| `UC-USG-` | Usage/billing |
| `UC-ORG-` | Organization management |
| `UC-OPS-` | Platform operations |

### ID Format

- Use Case: `UC-{PREFIX}-{NNN}` (e.g., `UC-BM-001`)
- Acceptance Criteria: `AC-{NN}` scoped to UC (referenced as `UC-BM-001/AC-01`)

## Schema Definition

```yaml
# usecases/{feature}.yaml

# Feature-level metadata
feature: string                    # Feature name (e.g., "Benchmarks")
description: |                     # Multiline markdown description
  Feature overview paragraph...

dependencies:                      # Other UC files this feature depends on
  - usecases/authentication.yaml
  - usecases/models.yaml

# List of use cases in this feature
usecases:
  - id: UC-{PREFIX}-{NNN}          # Required: Unique identifier
    title: string                  # Required: Short descriptive title
    interface: cli | web | api     # Required: Primary interface (default: cli)
    status: draft | active | deprecated  # Required: Current status
    bead: string                   # Optional: Parent bead ID (auto-populated by jb-3.1b)

    # Deprecation info (only when status: deprecated)
    deprecated_reason: string      # Why deprecated
    deprecated_by: UC-XXX-NNN      # Replacement UC if any

    # Detailed description (1-3 paragraphs)
    description: |                 # Required: Multiline markdown
      Explain what the user wants to achieve and why.
      This should focus on user intent, not implementation.

      Include context about when/why this use case applies.

    # Who performs this action
    actor: string                  # Required: Role (e.g., "Organization Admin")

    # What must be true before this UC can execute
    preconditions:                 # Required: List of prerequisites
      - User has valid org admin API key
      - At least one model available

    # Dependencies on other use cases
    depends_on:                    # Optional: List of UC IDs
      - UC-AUTH-001
      - UC-KEY-001

    # Acceptance Criteria - each becomes a test
    acceptance_criteria:           # Required: At least one AC
      - id: AC-{NN}                # Required: Unique within UC
        criterion: string          # Required: One-line description
        given: string              # Required: Initial state/context
        when: string               # Required: Action taken (include example command)
        then:                      # Required: Expected outcomes (list)
          - Outcome 1
          - Outcome 2

    # Scope boundaries - critical for preventing drift
    in_scope:                      # Required: What this UC covers
      - Creating benchmark target
      - Validating model access

    out_of_scope:                  # Required: What this UC explicitly excludes
      - Starting benchmark execution
      - Modifying existing targets

    must_not:                      # Required: Anti-requirements
      - Auto-start benchmark after creation
      - Modify any existing resources
```

## Example

```yaml
feature: Benchmarks
description: |
  Benchmark testing allows organization admins to validate model performance
  before production deployment. Users can create test targets, trigger runs,
  and analyze results.

dependencies:
  - usecases/authentication.yaml
  - usecases/apikeys.yaml
  - usecases/models.yaml

usecases:
  - id: UC-BM-001
    title: Create Benchmark Target
    interface: cli
    status: active
    bead: aas-ucbm001

    description: |
      An organization admin wants to create a benchmark target configuration
      that defines which model to test and with what parameters. This is the
      first step before running any benchmark tests.

      The target persists and can be reused for multiple benchmark runs,
      allowing consistent performance comparisons over time.

    actor: Organization Admin

    preconditions:
      - User has valid org admin API key
      - User has access to the specified model
      - Specified benchmark scenario exists

    depends_on:
      - UC-AUTH-001
      - UC-KEY-001
      - UC-MDL-001

    acceptance_criteria:
      - id: AC-01
        criterion: Create target with required fields
        given: User is authenticated with org admin API key
        when: User runs `ai-aas-org benchmark target add --model llama-7b --scenario standard`
        then:
          - Benchmark target is created
          - Target ID is returned and displayed
          - Target appears in `ai-aas-org benchmark target list`

      - id: AC-02
        criterion: Reject unauthorized model
        given: User specifies a model they don't have access to
        when: User runs `ai-aas-org benchmark target add --model restricted-model --scenario standard`
        then:
          - Command fails with exit code 4 (auth error)
          - Error message explains lack of model access
          - No target is created

      - id: AC-03
        criterion: Reject invalid scenario
        given: User specifies a non-existent scenario
        when: User runs `ai-aas-org benchmark target add --model llama-7b --scenario nonexistent`
        then:
          - Command fails with exit code 5 (not found)
          - Error message lists available scenarios
          - No target is created

    in_scope:
      - Creating target with model, scenario, optional parameters
      - Validating user has access to specified model
      - Validating scenario exists
      - Persisting target configuration
      - Returning target ID on success

    out_of_scope:
      - Starting benchmark execution (UC-BM-002)
      - Modifying existing targets (UC-BM-003)
      - Creating new scenarios (admin-only operation)
      - Real-time progress streaming

    must_not:
      - Auto-start benchmark after target creation
      - Modify any existing targets
      - Expose internal system metrics
      - Change model configurations
```

## Validation

Use the linter to validate UC files:

```bash
./scripts/lint-usecases.sh
```

The linter checks:
- ID format matches `UC-{PREFIX}-{NNN}` pattern
- Required fields present
- AC IDs follow `AC-{NN}` pattern
- Dependencies reference valid UCs
- No duplicate IDs within feature

## Test Mapping

Tests should use subtests with AC references:

```go
func TestUC_BM_001_CreateBenchmarkTarget(t *testing.T) {
    t.Run("AC-01: create target with required fields", func(t *testing.T) {
        // Given: authenticated org admin
        // When: create benchmark target
        // Then: target is created
    })

    t.Run("AC-02: reject unauthorized model", func(t *testing.T) {
        // ...
    })
}
```

## Related Documentation

- [CLAUDE.md](../CLAUDE.md) - Agent instructions for UC workflow
- [context/agents.md](../context/agents.md) - Core agent rules
