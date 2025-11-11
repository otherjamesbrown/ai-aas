# Implementation Plan: Web Portal

**Branch**: `008-web-portal` | **Date**: 2025-11-11 | **Spec**: `/specs/008-web-portal/spec.md`
**Input**: Feature specification from `/specs/008-web-portal/spec.md`

**Note**: Generated via `/speckit.plan`. Follow-up artifacts (`research.md`, `data-model.md`, `quickstart.md`, `contracts/`) are produced in Phases 0–1.

## Summary

Deliver a production-ready React 18 + TypeScript web portal that surfaces existing management APIs for organization administration, usage insights, and support tooling. The UI consumes API Router/identity/billing/audit services, enforces RBAC and confirmation flows, and ships with observability, accessibility, and performance guardrails (≤3s initial load, ≤2s dashboard interactions). Support impersonation is constrained to read-only, consented sessions with full audit coverage.

## Technical Context

**Language/Version**: TypeScript 5.x, React 18, Vite 5, Node.js 20 LTS  
**Primary Dependencies**: `shared/ts` design system, Storybook 8 for component coverage, TanStack Router/Query, Tailwind CSS + shadcn/ui, Zod validation, OpenTelemetry JS SDK, Axios (REST client), `@aio/feature-flags` (internal)  
**Storage**: Browser local/session storage for ephemeral state only; authoritative data in existing Identity, Organization, Billing, Audit services (PostgreSQL/Redis), feature flag config in LaunchDarkly (proxy)  
**Testing**: Vitest + React Testing Library, Playwright for E2E, Axe-core for automated accessibility, Contract tests via Pact with backend APIs  
**Target Platform**: Modern Chromium/Firefox/Safari desktop browsers with responsive support down to 375px; deployed via Nginx on LKE behind existing Ingress  
**Project Type**: Web SPA served statically with API integrations; co-located within repository `web/portal` package using shared monorepo tooling  
**Performance Goals**: Initial authenticated load <3s P50, dashboard interactions <2s P50 / <3s P95, destructive action confirmation latency <2.5s P95, support impersonation bootstraps <5s  
**Constraints**: Must uphold WCAG 2.2 AA, leverage existing auth flows (OAuth2/OIDC + MFA), emit telemetry with <5% overhead, honor GitOps deployment patterns, no sensitive data persisted client-side  
**Scale/Scope**: 5k concurrent sessions, orgs up to 500 members, usage datasets up to 30k records per 7-day window, feature rollout via flags per organization tier

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **API-First**: Portal exclusively consumes existing REST/GraphQL APIs; new surface area documented via OpenAPI contracts in `specs/008-web-portal/contracts/`. ✅
- **Stateless Microservices & Async**: UI remains stateless; relies on backend services that already satisfy persistence rules. Background analytics/exports trigger asynchronous jobs through existing services. ✅
- **Security by Default**: OAuth2/OIDC auth, RBAC enforcement, MFA prompts, masked API keys, audit logging, and secure storage plan align with mandates. Additional security testing (SAST/DAST) captured in tasks. ✅
- **Declarative Infrastructure & GitOps**: Deployment via Helm chart updates and ArgoCD sync; environment config managed in Git, feature flags via standard workflow. ✅
- **Observability/Test/Performance SLOs**: OpenTelemetry instrumentation, structured logs, dashboards for latency/usage, automated tests (unit, integration, E2E, accessibility), and defined performance budgets satisfy gates. ✅
- **Testing Discipline**: Plan includes Vitest, Playwright, Pact tests covering core flows with ≥80% coverage and no DB mocks. ✅
- **Performance**: Explicit SLO targets and synthetic monitoring plan; budget enforcement for heavy datasets documented. ✅

No waivers required; downstream implementation must adhere to these checkpoints.

## Project Structure

### Documentation (this feature)

```text
specs/[###-feature]/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)
<!--
  ACTION REQUIRED: Replace the placeholder tree below with the concrete layout
  for this feature. Delete unused options and expand the chosen structure with
  real paths (e.g., apps/admin, packages/something). The delivered plan must
  not include Option labels.
-->

```text
web/
└── portal/
    ├── src/
    │   ├── app/                 # routing and layout shells
    │   ├── components/          # shared UI primitives (modals, tables, banners) with Storybook stories
    │   ├── features/
    │   │   ├── org/             # org profile + member management flows
    │   │   ├── budgets/         # budget controls, alerts, history timelines
    │   │   ├── api-keys/        # key lifecycle management
    │   │   ├── usage/           # dashboards, charts, exports
    │   │   └── support/         # impersonation console
    │   ├── hooks/               # cross-cutting data-fetch, auth, feature flag hooks
    │   ├── lib/                 # client adapters for identity, billing, audit APIs
    │   ├── providers/           # auth, telemetry, theming, error boundaries
    │   └── styles/              # Tailwind config, theme tokens
    ├── public/                  # static assets, manifest, security headers
    ├── .storybook/              # Storybook config, preview, decorators
    ├── tests/
    │   ├── unit/                # Vitest suites per component/feature
    │   ├── integration/         # data-fetch + state management tests
    │   └── e2e/                 # Playwright specs for primary journeys
    ├── contracts/               # generated OpenAPI fragments & Pact files
    ├── package.json             # workspace package metadata
    ├── vite.config.ts           # build tooling
    ├── tailwind.config.ts
    └── tsconfig.json

shared/ts/                       # existing design system + utilities (consumed)
tests/ts/                        # cross-feature Playwright harness, shared fixtures
```

**Structure Decision**: Implement portal as a dedicated workspace package under `web/portal`, reusing `shared/ts` design system components and `tests/ts` Playwright harness. Feature folders encapsulate domain workflows, while `lib/` houses API adapters aligned with OpenAPI contracts. Tests mirror the constitution’s unit/integration/e2e layering and share fixtures with other TypeScript packages. Helm chart and GitOps manifests remain under existing `gitops/` and `infra/` directories; no new backend services introduced.

## Complexity Tracking

No constitution violations introduced; plan stays within existing front-end and deployment patterns.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| _None_ | — | — |
