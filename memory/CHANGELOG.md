# Constitution Changelog

All notable changes to the constitution are recorded here. Detailed narratives live under `memory/updates/`.

## [1.4.1] - 2025-01-27

### Added
- Task naming convention standard: `T-S{spec_number}-P{phase_number}-{task_number}` format
  - Spec numbers are three-digit (e.g., `006`)
  - Phase numbers are two-digit (e.g., `01`, `02`)
  - Task numbers are three-digit sequential within spec (continues across phases)
  - Examples: `T-S006-P01-001`, `T-S006-P02-007`
  - Task numbers continue sequentially across phases (if Phase 1 ends at 006, Phase 2 starts at 007)

### Changed
- Updated `.specify/templates/tasks-template.md` to include the new naming convention with examples

## [1.4.0] - 2025-11-07

### Added
- Core Principles consolidated and formalized:
  - API‑First Interfaces
  - Stateless Microservices & Async Non‑Critical Paths
  - Security by Default
  - Declarative Infrastructure & GitOps
  - Observability, Testing, and Performance SLOs
- Technology Standards & Non‑Negotiables for stack, ingress/TLS, observability, and GitOps.
- Development Workflow & Quality Gates with explicit Constitution Gates.
- Governance section (versioning policy, amendment flow, compliance reviews).

### Changed
- Clarified API standards (OpenAPI, RFC7807, ISO‑8601, UUIDs, streaming semantics).
- Strengthened CI security and supply‑chain scanning requirements.

### Removed
- None.


