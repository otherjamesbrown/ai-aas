# Research: KServe Migration

**Feature**: KServe Migration (`016`) | **Date**: 2025-11-24

## Decisions

### 1. Protocol Adapter Pattern
**Decision**: Implement "OpenAI-to-KServe Adapter" within `api-router-service`.
**Rationale**: 
- Maintains `specs/006` immutability (external contract unchanged).
- Avoids extra network hop of a sidecar or separate service.
- Allows leveraging existing router logic for auth/billing.
**Alternatives Considered**: 
- **Sidecar Proxy**: Rejected due to complexity in managing sidecars for the router.
- **Separate Microservice**: Rejected due to latency concerns (extra hop).

### 2. Hybrid Routing Strategy
**Decision**: Use `backend_type` column in Model Registry.
**Rationale**: 
- Simple schema change.
- Explicit control over which models are legacy vs KServe.
- Enables gradual migration (can flip flag per model).
**Alternatives Considered**: 
- **DNS Convention**: Rejected as too brittle.
- **Separate Registry Table**: Rejected as it complicates queries.

### 3. Autoscaling Verification
**Decision**: Use `specs/014-load-testing-harness`.
**Rationale**: 
- Existing tool designed for this purpose.
- Can generate realistic load patterns to trigger scale-up/down.
**Alternatives Considered**: 
- **Manual `curl` loops**: Insufficient for concurrency testing.
- **K6/JMeter**: Good, but `014` is already integrated.

## Resolved Unknowns

- **Q**: How to handle commercial licenses?
  - **A**: Use initContainers for validation (resolved in Spec).
- **Q**: MinReplicas for production?
  - **A**: Start with 1, optimize later (resolved in Spec).
