# Implementation Plan: UI Update & Admin Portal Rebuild

**Branch**: `018-ui-update` | **Date**: 2025-01-27 | **Spec**: `/specs/018-ui-update/spec.md`
**Input**: Feature specification from `/specs/018-ui-update/spec.md`

**Note**: Generated via `/speckit.plan`.

## Summary

Rebuild the web portal with stable architecture, centralized configuration, Linode-inspired UI design, and comprehensive Playwright test coverage. The solution addresses re-rendering issues, eliminates duplicated URL logic, standardizes HTTP client usage, implements system-aware theming, and provides UI coverage for all ai-aas-cli command groups. The portal serves as a complete web-based management interface with feature parity to the CLI tool, using existing REST APIs as a thin client.

## Technical Context

**Language/Version**: TypeScript 5.x, React 18.2+, Node.js 20+, Vite 5.x  
**Primary Dependencies**: React 18, @tanstack/react-router, @tanstack/react-query, axios, TailwindCSS, shadcn/ui, Playwright, Vitest  
**Storage**: sessionStorage (tokens, user data), localStorage (theme preferences), no persistent client-side storage  
**Testing**: Playwright E2E tests (smoke + integration), Vitest unit tests, ESLint + Prettier for code quality  
**Target Platform**: Web browsers (Chrome, Firefox, Safari), desktop-focused admin portal, served via Nginx  
**Project Type**: Web application (React SPA)  
**Performance Goals**: Login page loads < 3s, API responses < 1s, smoke tests < 2 minutes, integration tests < 15 minutes  
**Constraints**: Must work in localhost, nip.io, and production domain environments; zero console errors in production; TypeScript strict mode (no `any`); WCAG 2.1 AA accessibility  
**Scale/Scope**: ~20-30 pages covering all CLI command groups, 4 command groups (Model Management, Access Control, Platform Operations, Utilities), comprehensive E2E test coverage

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

**Initial Check (Pre-Phase 0)**: ✅ All gates pass  
**Post-Design Check (After Phase 1)**: ✅ All gates still pass

Constitution v1.4.2 is ratified. This plan satisfies each gate:

- **API-First**: ✅ UI is a thin client using existing REST APIs. All operations use the **same API services and endpoints as ai-aas-cli**. No business logic in UI; all operations map to CLI commands which use identical APIs. Base URLs computed from portal domain (e.g., `portal.dev.ai-aas.local` → `api.dev.ai-aas.local`, `user-org.dev.ai-aas.local`). OpenAPI schemas exist for backend APIs.

- **Statelessness**: ✅ Frontend is stateless SPA. Authentication tokens stored in sessionStorage (ephemeral). No in-process state; all persistent state managed by backend services (PostgreSQL, Redis). UI state is React component state only.

- **Async Non-Critical**: ✅ N/A for frontend (this is a client application). Health checks and telemetry are non-blocking. Error handling is asynchronous.

- **Security**: ✅ All requests authenticated via Bearer tokens. RBAC enforced by backend APIs. CSRF protection via headers. Secrets never stored in Git (tokens in sessionStorage only). TLS enforced via Ingress. Security headers configured in Nginx. Frontend code quality: ESLint, Prettier, TypeScript strict mode.

- **GitOps/Declarative**: ✅ Infrastructure declarative via Helm charts. ArgoCD manages deployments. Configuration via environment-specific Helm values. No manual kubectl operations required.

- **Observability**: ✅ Health endpoints (`/healthz`) configured in Nginx. OpenTelemetry integration for traces. Structured logging via shared libraries. Metrics collection via backend APIs. Error tracking and reporting.

- **Testing**: ✅ Unit tests via Vitest. E2E tests via Playwright (smoke + integration). Test coverage > 80% for critical paths. Tests run against remote dev cluster. CI/CD integration with artifact collection.

- **Performance**: ✅ Performance targets defined: login < 3s, API < 1s. Smoke tests < 2 minutes. Integration tests < 15 minutes. Performance monitoring via backend metrics.

No waivers needed; all gates pass. The UI rebuild maintains constitution compliance while improving architecture stability and user experience.

## Project Structure

### Documentation (this feature)

```text
specs/018-ui-update/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
web/portal/
├── src/
│   ├── config/
│   │   ├── api.ts              # Centralized API configuration (NEW)
│   │   └── index.ts
│   ├── lib/
│   │   ├── http/
│   │   │   └── client.ts       # httpClient + publicClient (MODIFIED)
│   │   └── query/
│   │       └── queryClient.ts  # TanStack Query client
│   ├── services/
│   │   ├── tokenManager.ts     # Token refresh management (NEW)
│   │   └── healthMonitor.ts    # Health check singleton (NEW)
│   ├── providers/
│   │   ├── AppProviders.tsx    # Combined provider wrapper (NEW)
│   │   ├── AuthProvider.tsx    # Authentication (MODIFIED)
│   │   ├── TelemetryProvider.tsx
│   │   ├── FeatureFlagProviderWrapper.tsx
│   │   └── ToastProvider.tsx
│   ├── components/
│   │   ├── layout/
│   │   │   ├── Sidebar.tsx     # Collapsible sidebar (NEW)
│   │   │   ├── Header.tsx      # Header with user menu (NEW)
│   │   │   ├── ContentArea.tsx # Content wrapper (NEW)
│   │   │   ├── Footer.tsx       # Footer component (NEW)
│   │   │   └── AdminLayout.tsx # Main layout wrapper (NEW)
│   │   ├── ui/
│   │   │   ├── DataTable.tsx   # Sortable, paginated table (NEW)
│   │   │   ├── StatusBadge.tsx # Traffic light indicators (NEW)
│   │   │   ├── TabNav.tsx      # Page-level tabs (NEW)
│   │   │   ├── Card.tsx        # Detail view cards (NEW)
│   │   │   ├── FilterPanel.tsx # Filters with dropdowns (NEW)
│   │   │   └── LoadingSpinner.tsx # Loading indicator (NEW)
│   │   ├── ServiceHealthCheck.tsx # Health check component (MODIFIED)
│   │   └── ErrorBoundary.tsx
│   ├── app/
│   │   ├── pages/
│   │   │   ├── LoginPage.tsx   # Login page (MODIFIED)
│   │   │   ├── HomePage.tsx    # Dashboard (MODIFIED)
│   │   │   └── ...
│   │   ├── features/
│   │   │   ├── admin/
│   │   │   │   ├── pages/
│   │   │   │   │   ├── ApiKeysPage.tsx      # API Keys (MODIFIED)
│   │   │   │   │   ├── MembersPage.tsx      # Members (MODIFIED)
│   │   │   │   │   └── OrganizationPage.tsx # Organization (MODIFIED)
│   │   │   │   └── api/
│   │   │   │       └── apiKeys.ts           # API client (MODIFIED)
│   │   │   ├── model-management/            # Model Management UI (NEW)
│   │   │   │   ├── pages/
│   │   │   │   │   ├── ModelRegistryPage.tsx
│   │   │   │   │   ├── ModelCachePage.tsx
│   │   │   │   │   ├── ModelDeployPage.tsx
│   │   │   │   │   └── ...
│   │   │   ├── access-control/              # Access Control UI (NEW)
│   │   │   │   ├── pages/
│   │   │   │   │   ├── OrganizationsPage.tsx
│   │   │   │   │   ├── UsersPage.tsx
│   │   │   │   │   └── ...
│   │   │   ├── platform-operations/         # Platform Operations UI (NEW)
│   │   │   │   └── pages/
│   │   │   │       ├── BootstrapPage.tsx
│   │   │   │       ├── RegistryPage.tsx
│   │   │   │       └── ...
│   │   │   └── utilities/                   # Utilities UI (NEW)
│   │   │       └── pages/
│   │   │           ├── StatusPage.tsx
│   │   │           └── ConfigPage.tsx
│   │   │   └── usage/
│   │   │       └── pages/
│   │   │           └── UsageDashboardPage.tsx # Usage Dashboard (MODIFIED)
│   │   └── AppRouter.tsx                    # Router configuration
│   ├── hooks/
│   │   └── useTheme.ts                      # Theme hook (NEW)
│   ├── styles/
│   │   └── global.css                       # Global styles + theme (MODIFIED)
│   └── main.tsx                             # Entry point (MODIFIED)
├── tests/
│   └── e2e/
│       ├── smoke.spec.ts                    # Smoke tests (NEW)
│       ├── login.spec.ts                    # Login tests (MODIFIED)
│       ├── api-keys.spec.ts                 # API Keys tests (NEW)
│       ├── members.spec.ts                  # Members tests (NEW)
│       ├── usage.spec.ts                    # Usage tests (NEW)
│       ├── helpers/
│       │   └── auth.ts                      # Auth helpers (NEW)
│       ├── pages/
│       │   ├── LoginPage.ts                 # Page object (NEW)
│       │   ├── HomePage.ts                  # Page object (NEW)
│       │   └── ApiKeysPage.ts               # Page object (NEW)
│       └── fixtures/
│           └── index.ts                     # Test data (NEW)
├── tailwind.config.js                       # Tailwind + theme config (MODIFIED)
├── playwright.config.ts                     # Playwright config (MODIFIED)
├── package.json
└── vite.config.ts
```

**Structure Decision**: Web application structure following existing React SPA pattern. Frontend-only changes; no backend modifications. Organized by feature areas matching CLI command groups. Shared components in `components/`, feature-specific pages in `features/`. Test structure mirrors source structure with page objects and helpers.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

No violations. All constitution gates pass without justification needed.
