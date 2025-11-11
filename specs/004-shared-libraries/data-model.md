# Data Model: Shared Libraries & Conventions

## Entities

### Shared Library Package
- **Description**: Versioned artifact (Go module or npm package) delivering reusable components for auth, config, observability, errors, and data access.
- **Key Attributes**:
  - `name`: Identifier (e.g., `@ai-aas/shared-auth`, `github.com/ai-aas/shared/auth`).
  - `version`: Semantic version string (`MAJOR.MINOR.PATCH`).
  - `language`: `go` or `typescript`.
  - `modules`: List of exported modules (auth, config, observability, errors, dataaccess).
  - `compatibility`: Supported runtime versions (Go LTS releases, Node.js 20/22).
  - `changelog`: Structured release notes with breaking-change markers.
- **Relationships**:
  - Publishes `PolicyBundle` artifacts (for auth).
  - Depends on `TelemetryProfile` definitions.

### Policy Bundle
- **Description**: Rego policy collection governing authorization decisions consumed by the shared auth middleware.
- **Key Attributes**:
  - `bundle_id`: Unique identifier for distribution.
  - `version`: Semantic version aligned with library release.
  - `roles`: Supported roles (e.g., `service-operator`, `read-only`).
  - `rules`: Rego policy modules with rule metadata (action, resource, effect).
  - `distribution_uri`: OCI or object storage endpoint for bundle fetch.
- **Relationships**:
  - Associated with one `Shared Library Package`.
  - Referenced by `TelemetryEvent` for audit purposes.

### Telemetry Event
- **Description**: Structured log/metric/trace record emitted by shared observability components.
- **Key Attributes**:
  - `event_id`: UUID or trace/span identifier.
  - `service_name`: Consumer service emitting the event.
  - `library_component`: Component generating the event (auth, config, observability, errors, dataaccess).
  - `severity`: Log level or metric severity marker.
  - `attributes`: Key-value map (request_id, user_id, role, outcome, latency_ms).
  - `timestamp`: RFC3339 timestamp.
- **Relationships**:
  - Correlates to `Shared Library Package` version.
  - Links to `PolicyBundle` evaluation results when applicable.

### Configuration Profile
- **Description**: Declarative configuration schema consumed by the shared configuration loader.
- **Key Attributes**:
  - `profile_id`: Identifier (e.g., `service-template`, `demo-app`).
  - `parameters`: Required and optional environment variables with validation (type, required). 
  - `secrets`: Secure keys referencing Vault paths or secret managers.
  - `defaults`: Safe defaults for non-secret values.
  - `telemetry`: Sampling rates, exporter endpoints.
- **Relationships**:
  - Used by consumer services built from `Service Template`.
  - Drives `TelemetryEvent` sampling and field inclusion.

### Service Template
- **Description**: Reference application demonstrating correct usage of shared libraries.
- **Key Attributes**:
  - `template_name`: Unique name (e.g., `shared-libraries-sample`).
  - `language`: `go` or `typescript`.
  - `example_routes`: Example endpoints showcasing middleware usage.
  - `test_suite`: Integration tests verifying adoption.
- **Relationships**:
  - Depends on specific `Shared Library Package` versions.
  - Emits `TelemetryEvent` instances for validation.

## State Considerations
- Version compatibility between Go and TypeScript packages must be tracked; maintain cross-language release matrix documenting supported combinations.
- Policy bundles and configuration profiles should support hot reload without service restarts; libraries expose watchers with backoff strategies.
- Telemetry exporters buffer events; ensure bounded queues with drop metrics for backpressure visibility.

## Validation Rules
- All configuration profiles must fail fast on missing required variables and emit structured error events.
- Policy bundles must include audit metadata (author, timestamp, hash) and pass conformance tests before publication.
- Shared libraries refuse initialization if runtime version does not match declared compatibility.
- Telemetry events must include trace/span IDs and request IDs; validation prevents emission without correlation fields.

