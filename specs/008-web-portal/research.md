# Research Findings: Web Portal

## Decision 1: Frontend Runtime & Build Tooling
- **Decision**: Use React 18 with Vite 5 and the existing `shared/ts` design system as the foundation for the portal.
- **Rationale**: Aligns with constitution technology standards, enables fast HMR for development, and keeps parity with other TypeScript packages already relying on Vite. The design system delivers accessible primitives that satisfy WCAG goals.
- **Alternatives Considered**:
  - *Next.js + SSR*: Rejected because server rendering complicates stateless deployment requirements and adds backend footprint not yet mandated for the portal.
  - *Create React App*: Deprecated and lacks modern build performance and configuration flexibility.

## Decision 2: Data Fetching & State Synchronization
- **Decision**: Adopt TanStack Query for API interaction, caching, and background refresh, combined with Zod validators to enforce contract alignment.
- **Rationale**: TanStack Query handles caching, retries, and stale data management crucial for usage dashboards and concurrent edits, while Zod provides runtime schema validation to guard against API drift. Both integrate smoothly with React Suspense and error boundaries.
- **Alternatives Considered**:
  - *Custom `fetch` wrappers with manual caching*: Rejected due to higher maintenance effort and lack of standardized retry/backoff handling.
  - *Redux Toolkit Query*: Adds global store complexity and is less aligned with existing TypeScript packages that prefer hook-based local state.

## Decision 3: Authentication & Session Management
- **Decision**: Integrate with the existing OAuth2/OIDC flow using PKCE, silent token refresh, and MFA challenge hand-offs via the identity service SDK.
- **Rationale**: Matches production identity patterns, supports session renewal without losing in-progress flows, and reuses vetted SDK components for security compliance.
- **Alternatives Considered**:
  - *Embedding Auth0 or third-party widgets*: Rejected to avoid vendor lock-in and ensure policies stay under IaC control.
  - *Custom JWT handling without SDK*: Rejected because it risks deviating from security baselines and increases maintenance overhead.

## Decision 4: Observability & Telemetry Strategy
- **Decision**: Instrument the portal with OpenTelemetry JS SDK for traces/metrics, ship logs to existing ingestion endpoints, and expose user-facing status badges sourced from telemetry.
- **Rationale**: Satisfies constitution observability gates, keeps portal instrumentation consistent with other services, and enables synthetic monitoring to validate SLOs.
- **Alternatives Considered**:
  - *Ad-hoc logging to console + Sentry-only tracing*: Rejected because it fragments telemetry pipelines and does not meet OpenTelemetry-first mandate.
  - *Manual metrics via REST endpoints*: Rejected due to higher effort and missing distributed trace correlation.

## Decision 5: Accessibility Verification
- **Decision**: Bake axe-core checks into Vitest and Playwright pipelines, schedule quarterly manual audits with NVDA/VoiceOver, and rely on design system tokens for color/contrast.
- **Rationale**: Automated checks provide fast feedback, while manual audits address nuances that tooling misses. Reusing design system tokens accelerates compliance with WCAG 2.2 AA.
- **Alternatives Considered**:
  - *Manual audits only*: Rejected because they delay feedback and risk regressions between releases.
  - *Automated checks only*: Rejected since automated tooling cannot fully cover screen reader and keyboard navigation requirements.

