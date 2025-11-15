# Feature Specification: Admin CLI

**Feature Branch**: `009-admin-cli`  
**Created**: 2025-11-07  
**Status**: Draft  
**Input**: User description: "Provide a command-line tool for platform administrators to perform privileged operations quickly and safely: bootstrap the first admin, manage organizations and users, rotate credentials, trigger syncs, and export reports. The outcome is reliable, scriptable operations with confirmations, dry-runs, and clear errors."

## Clarifications

### Session 2025-11-08

- Q: Should the CLI interact directly with databases or only via service APIs? → A: CLI should call service APIs (user-org-service, analytics-service) to maintain separation of concerns and leverage existing authentication/authorization. Direct database access only for break-glass recovery scenarios with explicit flags.
- Q: What level of observability is needed for CLI operations? → A: All privileged operations must emit audit logs with user identity, timestamp, command, and outcomes. CLI should support structured output (JSON) for integration with monitoring systems. Operations exceeding 30 seconds should provide progress indicators.
- Q: Which areas are explicitly out of scope for this feature? → A: Exclude service implementation logic, API endpoint creation (CLI consumes existing APIs), UI development, and deployment automation (CLI is a development/deployment tool, not a deployment target).

## User Scenarios & Testing *(mandatory)*

<!--
  IMPORTANT: User stories should be PRIORITIZED as user journeys ordered by importance.
  Each user story/journey must be INDEPENDENTLY TESTABLE - meaning if you implement just ONE of them,
  you should still have a viable MVP (Minimum Viable Product) that delivers value.
  
  Assign priorities (P1, P2, P3, etc.) to each story, where P1 is the most critical.
  Think of each story as a standalone slice of functionality that can be:
  - Developed independently
  - Tested independently
  - Deployed independently
  - Demonstrated to users independently
-->

### User Story 1 (US-001) - Bootstrap and break-glass operations (Priority: P1)

As a platform admin, I can bootstrap the first admin, recover access, and perform essential operations from a reliable CLI with dry-run and confirmation support.

**Why this priority**: Ensures operability in new or degraded environments. Without this capability, platform cannot be initialized or recovered from access loss scenarios.

**Independent Test**: Can be fully tested by running bootstrap and recovery flows in a test environment using dry-run and live modes. This delivers immediate value: platform initialization and break-glass recovery even without day-2 management (US-002) or reporting (US-003).

**Acceptance Scenarios**:

1. **[Primary]** **Given** a fresh environment with user-org-service running, **When** I run the bootstrap flow with `admin-cli bootstrap --dry-run`, **Then** CLI shows planned changes without side effects, and output includes clear next steps and required confirmations.
2. **[Primary]** **Given** the same environment, **When** I run `admin-cli bootstrap --confirm`, **Then** a system admin account is created, credentials are displayed securely (masked in logs), and audit logs record the operation with user identity and timestamp.
3. **[Exception]** **Given** a recovery scenario where admin access is lost, **When** I run `admin-cli recovery --break-glass --confirm` with proper authentication (service account token or recovery key), **Then** new admin credentials are produced, old credentials are invalidated, and operation is logged with break-glass flag for security review.
4. **[Alternate]** **Given** bootstrap is interrupted midway, **When** I re-run bootstrap, **Then** CLI detects existing admin and either skips creation or prompts for overwrite confirmation with clear risk warnings.
5. **[Exception]** **Given** required services (user-org-service) are unavailable, **When** I attempt bootstrap, **Then** CLI fails gracefully with clear error message indicating which service is unreachable and suggested remediation steps.

---

### User Story 2 (US-002) - Day-2 management at speed (Priority: P2)

As an operator, I can manage orgs/users/keys and trigger syncs via scripts for repeatable changes with predictable output and exit codes.

**Why this priority**: Enables automation and safe batch operations, reducing operational toil and enabling GitOps workflows.

**Independent Test**: Can be fully tested by integrating CLI commands into scripts and verifying idempotent outcomes and consistent exit codes. This delivers value even without bootstrap (US-001) by providing operational automation, and works with existing platform state.

**Acceptance Scenarios**:

1. **[Primary]** **Given** a list of org updates in a JSON/YAML file, **When** I run `admin-cli org update --file=orgs.yaml --dry-run`, **Then** CLI shows planned changes (create, update, delete) without side effects, with clear diff output and summary count.
2. **[Primary]** **Given** the same file, **When** I run `admin-cli org update --file=orgs.yaml --confirm`, **Then** changes are applied, CLI outputs structured summary (JSON when `--format=json`), and exit code is 0 on success or non-zero with error details.
3. **[Primary]** **Given** a script that calls `admin-cli user create --org=acme --email=user@example.com`, **When** user already exists, **Then** CLI returns non-zero exit code with clear error message, and operation is idempotent (re-running with `--upsert` succeeds).
4. **[Alternate]** **Given** declarative sync needs, **When** I trigger `admin-cli sync trigger --component=analytics --confirm`, **Then** CLI initiates sync, returns sync job ID, and provides `admin-cli sync status --job-id=<id>` to monitor progress with progress indicators for long-running operations.
5. **[Exception]** **Given** a batch operation partially fails (e.g., 5 of 10 orgs updated successfully), **When** operation completes, **Then** CLI outputs detailed failure report, returns exit code 1, and provides `--continue-on-error` flag for resilient batch processing.
6. **[Alternate]** **Given** structured output is needed for automation, **When** I use `--format=json` flag, **Then** all output (success, errors, summaries) is valid JSON suitable for parsing by CI/CD systems or monitoring tools.

---

### User Story 3 (US-003) - Exports and reporting (Priority: P3)

As a finance or operations stakeholder, I can export usage and membership reports from the CLI for reconciliation and audits.

**Why this priority**: Supports governance and compliance workflows, enabling self-service reporting without direct database access.

**Independent Test**: Can be fully tested by running exports and verifying column definitions and totals match system-of-record data. This delivers value independently by providing reporting capabilities even if only analytics-service APIs are available.

**Acceptance Scenarios**:

1. **[Primary]** **Given** a time range (e.g., `--from=2025-01-01 --to=2025-01-31`), **When** I export `admin-cli export usage --org=acme --format=csv --output=usage.csv`, **Then** CSV file includes column headers, usage totals, and CLI displays summary showing total rows and reconciliation checksum.
2. **[Primary]** **Given** the same export, **When** I compare CSV totals against analytics-service counters for the same window, **Then** values reconcile to within 1% tolerance (documented in export metadata), and discrepancies are flagged in CLI output.
3. **[Primary]** **Given** an audit request for membership changes, **When** I run `admin-cli export memberships --org=acme --changes-only --output=audit-2025-01-15.csv`, **Then** I receive a timestamped file with required columns (user_id, email, action, timestamp, changed_by), and file includes schema header comments.
4. **[Alternate]** **Given** a large export that may take >30 seconds, **When** export is initiated, **Then** CLI shows progress indicators (percentage complete, estimated time remaining) and allows cancellation via Ctrl+C with partial file cleanup.
5. **[Exception]** **Given** export fails due to service unavailability, **When** error occurs, **Then** CLI provides clear error message with retry suggestions, and any partial data is cleaned up or clearly marked as incomplete.
6. **[Alternate]** **Given** export data is sensitive, **When** file is written, **Then** CLI sets appropriate file permissions (0600) and logs export operation in audit logs for compliance tracking. 

---

### Edge Cases

1. **Partial Connectivity**
   - **Given** CLI runs in environment with intermittent connectivity, **When** API calls fail, **Then** CLI implements exponential backoff retry with configurable max attempts, and clear error messages indicate transient vs permanent failures.

2. **Long-Running Batch Operations**
   - **Given** a batch operation processing 1000+ orgs, **When** operation is interrupted or partially fails, **Then** CLI supports `--resume-from=<checkpoint>` flag to continue from last successful item, and provides checkpoint files for manual recovery.

3. **Accidental Destructive Commands**
   - **Given** a destructive operation (e.g., `admin-cli org delete --org=acme`), **When** command is issued without `--confirm`, **Then** CLI shows dry-run preview and requires explicit confirmation with risk warnings. Critical operations require `--force` flag in addition to `--confirm`.

4. **Time Synchronization**
   - **Given** system clock drift affecting token validity, **When** CLI authenticates, **Then** CLI validates token expiration with reasonable clock skew tolerance (±5 minutes), and provides clear error if tokens are invalid due to clock issues.

5. **Concurrent Operations**
   - **Given** multiple CLI instances running concurrently, **When** conflicting operations occur (e.g., two admins updating same org), **Then** API-level optimistic locking prevents data corruption, and CLI displays clear conflict errors with suggested resolution steps.

6. **Large Export Files**
   - **Given** export generates file >1GB, **When** export completes, **Then** CLI provides streaming/chunked output option, warns about file size, and supports compression (`--compress=gzip`) to reduce storage requirements.

7. **Missing Dependencies**
   - **Given** CLI requires services (user-org-service, analytics-service) that are not running, **When** commands are executed, **Then** CLI performs health checks upfront, fails fast with clear service dependency list, and provides troubleshooting guidance.

## Requirements *(mandatory)*

<!--
  ACTION REQUIRED: The content in this section represents placeholders.
  Fill them out with the right functional requirements.
-->

### Functional Requirements

- **FR-001**: Provide bootstrap commands for first-admin creation with clear prompts, dry-run preview, and secure credential output.  
- **FR-002**: Provide org/user/key lifecycle operations (create, read, update, delete, list) with confirmations, dry-run support, and idempotent behavior.  
- **FR-003**: Provide credential rotation flows with safe output (masked in logs), post-rotation guidance, and audit logging.  
- **FR-004**: Provide declarative sync triggers and status checks with machine-readable output (JSON format option) and progress indicators for long-running operations.  
- **FR-005**: Provide exports for usage/membership with clear column definitions, timestamps, schema headers, and reconciliation verification against service counters.  
- **FR-006**: Provide predictable exit codes (0 for success, non-zero for failures) and structured output (JSON, CSV) for scripting and automation.  
- **FR-007**: Provide safeguards for destructive operations (confirmations, `--force` flags, dry-run previews) and break-glass recovery mode with explicit authentication.  
- **FR-008**: Provide health checks for service dependencies (user-org-service, analytics-service) before executing operations, with clear error messages when services are unavailable.  
- **FR-009**: Provide retry logic with exponential backoff for transient API failures, with configurable max attempts and timeout values.  
- **FR-010**: Provide audit logging for all privileged operations, including user identity, timestamp, command, parameters (masked where sensitive), and outcomes.  
- **FR-011**: Provide batch operations with file input support (JSON/YAML), partial failure handling, checkpoint/resume capability, and structured error reporting.  
- **FR-012**: Provide help system with `--help` flags, command completion (bash/zsh), and example usage in documentation.

### Non-Functional Requirements

**Performance:**
- **NFR-001**: Bootstrap operation completes in ≤2 minutes on standard hardware with services running locally or in same network region.
- **NFR-002**: Single org/user operation completes in ≤5 seconds under normal conditions (services responding within 200ms).
- **NFR-003**: Batch operations process 100 items in ≤2 minutes, with progress indicators for operations exceeding 10 seconds.
- **NFR-004**: Export operations stream output for large datasets (>10k rows) to avoid memory exhaustion, completing within reasonable time (≤1 minute per 10k rows).

**Reliability:**
- **NFR-005**: CLI implements idempotent operations (re-running same command produces same outcome without errors when state matches expectations).
- **NFR-006**: Retry logic handles transient failures (network timeouts, 5xx errors) with exponential backoff (max 3 attempts, 1s/2s/4s delays).
- **NFR-007**: CLI fails fast when required services are unavailable (health check within 5 seconds), avoiding partial operations and unclear error states.
- **NFR-008**: Batch operations support checkpoint/resume for operations interrupted mid-execution, with clear checkpoint file format and recovery instructions.

**Security:**
- **NFR-009**: All authentication credentials and API keys are masked in logs and output (replaced with `***` or last 4 characters only).
- **NFR-010**: Audit logs capture all privileged operations (bootstrap, credential rotation, destructive operations) with user identity, timestamp, command, and outcomes.
- **NFR-011**: CLI validates authentication tokens before executing operations, with clear error messages if tokens are expired or invalid.
- **NFR-012**: Break-glass recovery operations require explicit authentication flags or service account tokens, and are logged with special audit flags for security review.
- **NFR-013**: Export files are written with restrictive permissions (0600) to prevent unauthorized access, and export operations are logged for compliance tracking.

**Usability:**
- **NFR-014**: CLI provides clear, actionable error messages with suggested remediation steps and links to documentation when errors occur.
- **NFR-015**: Help system (`--help`) is comprehensive, includes examples, and responds instantly (under 1 second).
- **NFR-016**: Command completion (bash/zsh) is provided for all commands and flags, reducing typing errors and improving discoverability.
- **NFR-017**: Dry-run mode provides clear, human-readable previews of planned changes, with summaries showing counts (e.g., "3 orgs to create, 2 to update, 1 to delete").

**Scriptability:**
- **NFR-018**: Structured output (JSON format) is consistent across all commands, with stable schema suitable for parsing by automation tools.
- **NFR-019**: Exit codes are predictable (0 for success, 1 for general errors, 2 for usage errors, 3 for service unavailability) and documented.
- **NFR-020**: CLI supports non-interactive mode (all prompts can be satisfied via flags) for CI/CD integration and automation workflows.
- **NFR-021**: Progress indicators for long-running operations are suitable for CI logs (single-line updates or structured JSON progress events).

**Maintainability:**
- **NFR-022**: CLI code is modular and testable, with clear separation between command parsing, API client, and output formatting layers.
- **NFR-023**: Error handling is consistent across all commands, with structured error types and recovery suggestions.
- **NFR-024**: CLI supports configuration files (YAML/JSON) for default values (API endpoints, timeouts, retry settings) while allowing flag overrides.

**Observability:**
- **NFR-025**: All operations emit structured logs (JSON format option) suitable for ingestion by logging systems (Loki, Elasticsearch).
- **NFR-026**: CLI supports `--verbose` flag for debug output and `--quiet` flag for minimal output in scripts.
- **NFR-027**: Long-running operations (>30 seconds) emit progress events that can be consumed by monitoring systems for operational visibility.
- **NFR-028**: Audit logs include operation duration, success/failure status, and resource identifiers for correlation with service-side logs.

## Out of Scope

- Service implementation logic (CLI consumes existing service APIs; does not implement business logic).
- API endpoint creation (CLI uses existing user-org-service and analytics-service APIs; no new endpoints required).
- UI development (CLI is command-line only; no web interface or GUI components).
- Deployment automation (CLI is a development/deployment tool, not a deployment target; distribution and installation are out of scope for initial version).
- Direct database access in normal operations (CLI uses service APIs; direct DB access only for break-glass recovery with explicit flags).
- Advanced reporting features (beyond basic exports; complex analytics, dashboards, or visualization are out of scope).
- Multi-platform distribution (initial version targets Linux/macOS with Go binary distribution; Windows support and package managers are future work).

## Success Criteria *(mandatory)*

<!--
  ACTION REQUIRED: Define measurable success criteria.
  These must be technology-agnostic and measurable.
-->

### Measurable Outcomes

- **SC-001**: First-admin bootstrap completes in ≤2 minutes on standard hardware with services running, and outputs clear, copyable credentials with next-step guidance.  
- **SC-002**: All lifecycle commands (org/user/key operations) support a dry-run mode that lists planned changes without side effects, with human-readable summaries showing counts and diffs.  
- **SC-003**: Batch scripts using the CLI return consistent exit codes (0 for success, non-zero for failures) and machine-readable summaries (JSON format) suitable for CI/CD integration, with 100% idempotent behavior when re-run with same inputs.  
- **SC-004**: Exports reconcile to within 1% of internal service counters for the same time window, include schema headers in CSV output, and complete within reasonable time (≤1 minute per 10k rows with progress indicators).  
- **SC-005**: CLI help system responds in under 1 second and provides comprehensive examples for all commands, with command completion (bash/zsh) reducing typing errors by 80% in user testing.  
- **SC-006**: Audit logs capture 100% of privileged operations (bootstrap, credential rotation, destructive operations) with required fields (user identity, timestamp, command, masked parameters, outcomes), and logs are queryable via standard tools.

## Key Entities

- **Admin CLI Command**: Represents a single CLI operation (e.g., `bootstrap`, `org create`, `export usage`) with command structure (verb, subject, flags), validation rules, authentication requirements, and output format options. Commands are organized hierarchically (e.g., `org` subcommands: `create`, `list`, `update`, `delete`).

- **Bootstrap Operation**: Initial platform setup process that creates the first system admin account, validates service dependencies, and establishes initial configuration. Includes dry-run preview, confirmation prompts, secure credential generation, and audit logging. Triggered via `admin-cli bootstrap` command.

- **Credential Rotation**: Secure process for updating authentication credentials (API keys, service account tokens) with invalidation of old credentials, secure output of new credentials, and post-rotation guidance. Includes break-glass recovery mode for access loss scenarios. Triggered via `admin-cli credentials rotate` command.

- **Batch Operation**: Multi-item processing workflow (e.g., updating 100 orgs from file) with file input parsing (JSON/YAML), validation, dry-run preview, checkpoint/resume capability, partial failure handling, and structured error reporting. Supports idempotent execution and structured output formats.

- **Export Operation**: Data extraction process for usage/membership reports that queries service APIs (analytics-service, user-org-service), formats output (CSV/JSON), includes schema headers, performs reconciliation checks against service counters, and writes files with appropriate permissions. Supports streaming for large datasets and compression options.

- **Audit Log Entry**: Structured log record capturing privileged operation details (user identity from authentication token, timestamp in ISO-8601, command executed, parameters with sensitive values masked, operation outcome, duration). Emitted for all bootstrap, credential rotation, and destructive operations. Stored in audit log system (separate from application logs).

- **Service Dependency**: External service (user-org-service, analytics-service) required for CLI operations. Includes health check logic, connection configuration (endpoint URL, timeout, retry settings), and error handling (transient vs permanent failures, retry strategies). CLI performs upfront health checks before executing operations.

- **CLI Configuration**: Persistent settings (API endpoints, authentication tokens, default timeouts, retry policies) stored in configuration file (YAML/JSON) or environment variables, with flag-based overrides. Includes validation, secure storage of credentials, and configuration file location discovery (home directory, project directory, explicit path).

## Assumptions

- Platform services (user-org-service, analytics-service) expose REST APIs that the CLI can consume; APIs follow platform conventions (OpenAPI specs, RFC7807 error format, authentication via Bearer tokens).
- CLI operators have access to authentication credentials (API keys, service account tokens, or OAuth2 tokens) for authenticating with platform services; credentials are obtained via separate onboarding process.
- Operators have basic command-line proficiency (can navigate directories, run commands, parse JSON/CSV output) and access to standard terminal environments (bash/zsh on Linux/macOS).
- Platform services are deployed and accessible (either locally for development or remotely via network); CLI performs health checks but does not provision or deploy services.
- Audit log system is available for storing privileged operation logs; CLI emits logs but does not implement log storage or querying (assumes external log aggregation system).
- CLI is distributed as a single Go binary for Linux/macOS; operators can download and execute binary without package manager dependencies (initial version).
- Export operations assume services can handle queries for large time ranges (up to 1 year) and large datasets (up to 100k rows) within reasonable time; streaming/chunking is implemented if needed.
- Batch operations assume input files are reasonably sized (<10MB) and can be parsed in memory; very large batches may require streaming or chunking in future versions.
- CLI operators understand the risks of privileged operations and use dry-run and confirmation flags appropriately; CLI provides safeguards but cannot prevent misuse.
- Platform services implement proper authorization checks; CLI relies on service-side RBAC enforcement and does not duplicate authorization logic.

## Traceability Matrix

| User Story | Functional Requirements | Non-Functional Requirements | Success Criteria |
|------------|-------------------------|-----------------------------|------------------|
| US-001 | FR-001, FR-007, FR-008, FR-010 | NFR-001, NFR-005, NFR-009, NFR-010, NFR-011, NFR-012, NFR-028 | SC-001, SC-006 |
| US-002 | FR-002, FR-004, FR-006, FR-009, FR-011, FR-012 | NFR-002, NFR-003, NFR-005, NFR-006, NFR-007, NFR-008, NFR-014, NFR-017, NFR-018, NFR-019, NFR-020, NFR-021, NFR-024, NFR-026 | SC-002, SC-003, SC-005 |
| US-003 | FR-005, FR-006, FR-010, FR-012 | NFR-004, NFR-013, NFR-018, NFR-019, NFR-020, NFR-021, NFR-025, NFR-027 | SC-004, SC-006 |
