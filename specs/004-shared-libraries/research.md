# Research Findings: Shared Libraries & Conventions

## Decision 1: Observability Instrumentation Standard
- **Decision**: Adopt OpenTelemetry SDKs as the primary instrumentation layer for logs, metrics, and traces in both Go and TypeScript shared libraries.
- **Rationale**: OpenTelemetry provides vendor-neutral APIs covering the triad of telemetry signals and integrates with existing collectors referenced in infrastructure specs. Both language ecosystems have mature SDKs with automatic context propagation, reducing custom instrumentation effort.
- **Alternatives Considered**:
  - *Language-specific logging packages only (e.g., `zap`, `pino`)* — rejected because they do not natively unify metrics and tracing, requiring separate integrations per signal.
  - *Proprietary APM agents* — rejected due to licensing costs and lack of alignment with open governance requirements.

## Decision 2: Package Distribution Strategy
- **Decision**: Publish shared libraries via internal package registries: Go modules served from the monorepo using `go.work` with semantic version tags, and TypeScript packages released to the private npm registry managed by the infrastructure team.
- **Rationale**: Aligns with current repo tooling (Go workspace, npm usage) and supports semver-based promotion flows with existing CI/CD. Internal registries simplify access control and auditability.
- **Alternatives Considered**:
  - *Single monolithic multi-language repo exported as git submodules* — rejected because consumer services would need to vendor Git history and lose semver metadata.
  - *Binary distribution via artifact storage* — rejected since libraries need source-level integration for type safety and tree-shaking.

## Decision 3: Configuration & Secrets Management Pattern
- **Decision**: Standardize configuration loading on environment variables backed by `.env` files for local dev, with optional integration to HashiCorp Vault via pluggable providers.
- **Rationale**: Environment variables align with Twelve-Factor practices, minimize runtime coupling, and match existing service conventions. Vault integration satisfies security requirements for secrets without forcing every service to adopt it immediately.
- **Alternatives Considered**:
  - *Static YAML configuration files checked into repos* — rejected due to risk of secret leakage and lack of runtime overrides.
  - *Custom configuration service* — rejected as out of scope and duplicates existing infrastructure capabilities.

## Decision 4: Authorization Policy Format
- **Decision**: Define authorization policies using Rego (Open Policy Agent) bundles bundled with the shared authorization middleware.
- **Rationale**: OPA policies are already referenced in security standards, support fine-grained evaluation, and can be versioned alongside libraries. Bundles can be hot-reloaded from the platform policy distribution channel.
- **Alternatives Considered**:
  - *Ad-hoc role maps in code* — rejected due to lack of auditability and increased drift risk.
  - *Centralized remote policy service* — rejected for this phase to avoid adding runtime network dependencies; can be revisited later.

## Decision 5: Upgrade Safety Mechanisms
- **Decision**: Provide consumer-facing upgrade checklists and automated compatibility tests using contract tests and consumer-driven tests in a sample service repository.
- **Rationale**: Ensures FR-007/FR-008 requirements by coupling release notes with executable verification, reducing regression risk during upgrades.
- **Alternatives Considered**:
  - *Manual release notes only* — rejected because they do not provide automated verification.
  - *Forcing consumers to run full integration suites per release* — rejected due to time cost; curated contract tests provide faster signals.

