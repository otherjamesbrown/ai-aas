# Tasks: Web Portal

**Input**: Design documents from `/specs/008-web-portal/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, quickstart.md, contracts/portal-management.yaml

**Tests**: Required by constitution gates (unit + integration + e2e + accessibility).

**Organization**: Tasks are grouped by user story so each slice is independently implementable and testable.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Task can run in parallel once its prerequisites complete (different files / no ordering dependency)
- **[Story]**: Maps the task to a specific user story (US1–US4)
- Every task references an exact file path

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Establish portal workspace, tooling, and configuration baseline.

- [ ] T-S008-P01-001 Create `pnpm-workspace.yaml` at repo root including `shared/ts`, `tests/ts/*`, and new `web/portal` package.
- [ ] T-S008-P01-002 Scaffold `web/portal/package.json` with React 18, Vite 5, TanStack Query, Zod, Tailwind, Playwright, Vitest, and shared dependencies.
- [ ] T-S008-P01-003 Add TypeScript project config in `web/portal/tsconfig.json` aligning with monorepo references.
- [ ] T-S008-P01-004 Define Vite build pipeline in `web/portal/vite.config.ts` (HTTPS dev server, alias to `shared/ts`).
- [ ] T-S008-P01-005 [P] Configure Tailwind theme tokens in `web/portal/tailwind.config.ts`.
- [ ] T-S008-P01-006 [P] Add PostCSS pipeline in `web/portal/postcss.config.cjs` (Tailwind + autoprefixer).
- [ ] T-S008-P01-007 [P] Document environment variables in `web/portal/.env.example` (OAuth client, API base, telemetry keys).

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core app shell, providers, and tooling that all user stories depend on.  
**⚠️ CRITICAL**: No user story work can begin until these tasks are complete.

- [ ] T-S008-P02-008 Create root app bootstrap in `web/portal/src/main.tsx` wiring routing, auth, telemetry, feature flags, and global styles.
- [ ] T-S008-P02-009 Implement TanStack Router shell in `web/portal/src/app/AppRouter.tsx` with lazy route placeholders.
- [ ] T-S008-P02-010 Build shared layout (app bar, nav shell, notification slot) in `web/portal/src/app/Layout.tsx`.
- [ ] T-S008-P02-011 Implement OAuth2/OIDC context in `web/portal/src/providers/AuthProvider.tsx` (silent refresh + MFA prompts).
- [ ] T-S008-P02-012 [P] Implement feature flag provider backed by `@aio/feature-flags` in `web/portal/src/providers/FeatureFlagProvider.tsx`.
- [ ] T-S008-P02-013 [P] Implement OpenTelemetry wiring in `web/portal/src/providers/TelemetryProvider.tsx` (OTLP exporters, correlation IDs).
- [ ] T-S008-P02-014 [P] Create Axios client with auth/CSRF interceptors in `web/portal/src/lib/http/client.ts`.
- [ ] T-S008-P02-015 [P] Expose TanStack Query client + hydration utilities in `web/portal/src/lib/query/index.ts`.
- [ ] T-S008-P02-016 [P] Add global Tailwind styles, responsive breakpoints, and container queries in `web/portal/src/styles/global.css`.
- [ ] T-S008-P02-017 Configure Vitest runner with JSX + axe-core setup in `web/portal/vitest.config.ts`.
- [ ] T-S008-P02-018 Configure Playwright (projects, storage state, axe plugin) in `web/portal/playwright.config.ts`.
- [ ] T-S008-P02-019 [P] Document responsive viewport guidelines in `web/portal/tailwind.config.ts` (fluid typography, layout constraints).
- [ ] T-S008-P02-020 [P] Scaffold Storybook workspace (`web/portal/.storybook/*`) and scripts integrating `shared/ts`.

**Checkpoint**: App shell renders, providers resolve, tests/e2e commands execute.

---

## Phase 3: User Story 1 - Admin Orchestrates Organization Lifecycle (Priority: P1) 🎯 MVP

**Goal**: Admins can manage org details, members, budgets, and API keys safely with confirmations and audit transparency.

**Independent Test**: Seed staging org, walk invite → role → budget → API key flows using `tests/e2e/admin-workflows.spec.ts`; verify state via API responses and audit banner history without enabling dashboards.

### Tests for User Story 1

- [ ] T-S008-P03-021 [P] [US1] Add Pact contract coverage for organization/member/budget/key endpoints in `web/portal/tests/contract/admin-portal.pact.ts`.
- [ ] T-S008-P03-022 [P] [US1] Create Playwright journey for admin flows in `web/portal/tests/e2e/admin-workflows.spec.ts`.

### Implementation for User Story 1

- [ ] T-S008-P03-023 [US1] Define admin domain types (OrganizationProfile, MemberAccount, BudgetPolicy, ApiKeyCredential) in `web/portal/src/features/admin/types.ts`.
- [ ] T-S008-P03-024 [P] [US1] Implement organization profile SDK in `web/portal/src/features/admin/api/organization.ts`.
- [ ] T-S008-P03-025 [P] [US1] Implement member management SDK in `web/portal/src/features/admin/api/members.ts`.
- [ ] T-S008-P03-026 [P] [US1] Implement budget policy SDK in `web/portal/src/features/admin/api/budgets.ts`.
- [ ] T-S008-P03-027 [P] [US1] Implement API key lifecycle SDK in `web/portal/src/features/admin/api/apiKeys.ts`.
- [ ] T-S008-P03-028 [US1] Build organization settings page with optimistic updates in `web/portal/src/features/admin/org/OrganizationSettingsPage.tsx`.
- [ ] T-S008-P03-029 [US1] Build member management flows (invite/resend/remove/role) in `web/portal/src/features/admin/members/MemberManagementPage.tsx`.
- [ ] T-S008-P03-030 [US1] Build budget controls UI with confirmations in `web/portal/src/features/admin/budgets/BudgetControlsPage.tsx`.
- [ ] T-S008-P03-031 [US1] Build API key management UI (create/rotate/revoke with masked display) in `web/portal/src/features/admin/api-keys/ApiKeysPage.tsx`.
- [ ] T-S008-P03-032 [US1] Implement audit event banner + toast system in `web/portal/src/features/admin/components/AuditEventBanner.tsx`.
- [ ] T-S008-P03-033 [US1] Register admin routes and loaders in `web/portal/src/app/routes/adminRoutes.tsx`.
- [ ] T-S008-P03-034 [US1] Persist multi-step invite wizard state across session refresh in `web/portal/src/features/admin/members/hooks/useInviteWizard.ts`.
- [ ] T-S008-P03-035 [US1] Surface stale data conflicts with refresh guidance in `web/portal/src/features/admin/components/StaleDataBanner.tsx`.
- [ ] T-S008-P03-036 [P] [US1] Author Storybook stories for admin forms and confirmation flows in `web/portal/src/features/admin/**/*.stories.tsx`.
- [ ] T-S008-P03-037 [P] [US1] Add session expiry recovery Playwright spec in `web/portal/tests/e2e/session-expiry.spec.ts`.
- [ ] T-S008-P03-038 [US1] Implement session expiry re-auth banner/reset logic in `web/portal/src/features/admin/components/SessionExpiredNotice.tsx`.

**Checkpoint**: Admin flows complete within 3 minutes, audit events fire, confirmations enforced.

---

## Phase 4: User Story 2 - Role-Based Views & Safe Actions (Priority: P1)

**Goal**: Enforce least privilege UX with clear disabled states, typed confirmations, and secure error handling.

**Independent Test**: Execute `tests/integration/rbac-navigation.test.tsx` and `tests/e2e/destructive-confirmation.spec.ts` using accounts per role; ensure unauthorized routes redirect and destructive actions demand typed confirmation plus banner feedback.

### Tests for User Story 2

- [ ] T-S008-P04-039 [P] [US2] Add RBAC navigation integration tests in `web/portal/tests/integration/rbac-navigation.test.tsx`.
- [ ] T-S008-P04-040 [P] [US2] Add Playwright destructive-action confirmation suite in `web/portal/tests/e2e/destructive-confirmation.spec.ts`.
- [ ] T-S008-P04-041 [P] [US2] Add responsive navigation Playwright suite covering breakpoints in `web/portal/tests/e2e/responsive-navigation.spec.ts`.

### Implementation for User Story 2

- [ ] T-S008-P04-042 [US2] Extend role metadata and permission matrices in `web/portal/src/features/access/types.ts`.
- [ ] T-S008-P04-043 [US2] Implement permission guard hook with feature flag checks in `web/portal/src/features/access/hooks/usePermissionGuard.ts`.
- [ ] T-S008-P04-044 [US2] Build role-aware navigation/menu component in `web/portal/src/app/components/RoleAwareNav.tsx`.
- [ ] T-S008-P04-045 [US2] Implement typed confirmation modal for destructive actions in `web/portal/src/components/ConfirmDestructiveModal.tsx`.
- [ ] T-S008-P04-046 [US2] Implement permission tooltip helper in `web/portal/src/components/PermissionTooltip.tsx`.
- [ ] T-S008-P04-047 [US2] Add access denied page with support links in `web/portal/src/app/pages/AccessDeniedPage.tsx`.
- [ ] T-S008-P04-048 [US2] Wire protected route wrapper enforcing scopes in `web/portal/src/app/routes/ProtectedRoute.tsx`.

**Checkpoint**: Unauthorized users cannot access admin actions; destructive actions require type-to-confirm + logging.

---

## Phase 5: User Story 3 - Usage Insights for Quick Decisions (Priority: P2)

**Goal**: Deliver responsive dashboards with filters, empty states, and exportable summaries for usage trends.

**Independent Test**: Seed synthetic billing datasets; run `tests/unit/usage-calculations.test.ts` and `tests/e2e/usage-insights.spec.ts` to confirm filters, latency budgets (<2s), and CSV export fidelity.

### Tests for User Story 3

- [ ] T-S008-P05-049 [P] [US3] Add usage aggregation unit tests in `web/portal/tests/unit/usage-calculations.test.ts`.
- [ ] T-S008-P05-050 [P] [US3] Add Playwright dashboard filter coverage in `web/portal/tests/e2e/usage-insights.spec.ts`.

### Implementation for User Story 3

- [ ] T-S008-P05-051 [US3] Define usage and report types in `web/portal/src/features/usage/types.ts`.
- [ ] T-S008-P05-052 [P] [US3] Implement usage API client with degraded state handling in `web/portal/src/features/usage/api.ts`.
- [ ] T-S008-P05-053 [US3] Implement TanStack Query hooks for usage reports in `web/portal/src/features/usage/hooks/useUsageReport.ts`.
- [ ] T-S008-P05-054 [US3] Build dashboard UI with charts and KPIs in `web/portal/src/features/usage/components/UsageDashboard.tsx`.
- [ ] T-S008-P05-055 [US3] Build empty/degraded state components in `web/portal/src/features/usage/components/UsageEmptyState.tsx`.
- [ ] T-S008-P05-056 [US3] Implement CSV export worker handling large datasets in `web/portal/src/features/usage/workers/exportUsageCsv.ts`.
- [ ] T-S008-P05-057 [US3] Add usage routes and loaders in `web/portal/src/app/routes/usageRoutes.tsx`.
- [ ] T-S008-P05-058 [US3] Implement status badge component showing last successful sync in `web/portal/src/features/usage/components/StatusBadge.tsx`.
- [ ] T-S008-P05-059 [P] [US3] Wire status badge telemetry + tests in `web/portal/src/features/usage/hooks/useStatusBadge.ts`.
- [ ] T-S008-P05-060 [P] [US3] Author Storybook stories for usage dashboard, empty state, and status badge in `web/portal/src/features/usage/**/*.stories.tsx`.

**Checkpoint**: Usage dashboard meets latency targets, handles empty/degraded states, exports data.

---

## Phase 6: User Story 4 - Support Escalations Resolve Faster (Priority: P3)

**Goal**: Enable consented, time-bound impersonation with read-only safeguards and clear session indicators.

**Independent Test**: Use support account to run `tests/e2e/support-impersonation.spec.ts` verifying consent, banners, read-only guard, and audit emission; validate `tests/unit/support-readonly-guard.test.ts` for guard correctness.

### Tests for User Story 4

- [ ] T-S008-P06-061 [P] [US4] Add Playwright flow for support impersonation in `web/portal/tests/e2e/support-impersonation.spec.ts`.
- [ ] T-S008-P06-062 [P] [US4] Add guard unit tests in `web/portal/tests/unit/support-readonly-guard.test.ts`.

### Implementation for User Story 4

- [ ] T-S008-P06-063 [US4] Implement support impersonation API module in `web/portal/src/features/support/api/impersonation.ts`.
- [ ] T-S008-P06-064 [US4] Build consent modal capturing justification + token in `web/portal/src/features/support/components/ImpersonationConsentModal.tsx`.
- [ ] T-S008-P06-065 [US4] Build session banner with countdown + revoke button in `web/portal/src/features/support/components/ImpersonationBanner.tsx`.
- [ ] T-S008-P06-066 [US4] Implement read-only guard hook in `web/portal/src/features/support/hooks/useImpersonationGuard.ts`.
- [ ] T-S008-P06-067 [US4] Build support console page in `web/portal/src/features/support/pages/SupportConsolePage.tsx`.
- [ ] T-S008-P06-068 [US4] Forward impersonation audit events to telemetry in `web/portal/src/features/support/api/auditTrail.ts`.

**Checkpoint**: Support engineers can resolve escalations without persistent admin rights; audit trail intact.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Hardening, documentation, and compliance tasks spanning all stories.

- [ ] T-S008-P07-069 [P] Author portal usage troubleshooting guide in `docs/runbooks/portal-usage.md`.
- [ ] T-S008-P07-070 [P] Document synthetic monitoring setup in `dashboards/web-portal/README.md`.
- [ ] T-S008-P07-071 Record accessibility audit (axe + manual screen reader) in `docs/metrics/report.md`.
- [ ] T-S008-P07-072 Capture Lighthouse performance baseline (<3s TTI, <0.1 CLS) in `docs/metrics/report.md`.
- [ ] T-S008-P07-073 Execute quickstart end-to-end and annotate verification notes in `specs/008-web-portal/quickstart.md`.
- [ ] T-S008-P07-074 Update portal entries in `llms.txt` with spec, plan, quickstart, and dashboards links.
- [ ] T-S008-P07-075 Configure Storybook build + visual regression check in CI via `web/portal/package.json` scripts and `.github/workflows`.
- [ ] T-S008-P07-076 Capture destructive-action latency metrics <2.5s through synthetic monitoring in `web/portal/tests/e2e/destructive-confirmation.spec.ts` and telemetry dashboards.
- [ ] T-S008-P07-077 Run k6 load test for 5k concurrent sessions with CDN/rate-limit validation; document results in `docs/metrics/report.md`.
- [ ] T-S008-P07-078 Document CDN caching and rate-limiting configuration in `docs/platform/observability-guide.md`.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No prerequisites - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3+)**: All depend on Foundational phase completion
  - User stories can then proceed in parallel (if staffed)
  - Or sequentially in priority order (P1 → P2 → P3)
- **Polish (Final Phase)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) - foundation for notifications, audit, admin APIs
- **User Story 2 (P1)**: Can start after Foundational (Phase 2) - reuses auth context and routes from US-001
- **User Story 3 (P2)**: Can start after Foundational (Phase 2) - depends on shared telemetry/notification plumbing
- **User Story 4 (P3)**: Can start after Foundational (Phase 2) - depends on RBAC + notification patterns from earlier stories

### Within Each User Story

- Execute tests before implementation where possible (Pact, unit, e2e).
- Implement domain types → API clients → hooks → UI.
- Wire routes last, after page components pass lint/tests.

### Parallel Opportunities

- **Phase 1**: T-S008-P01-005–T-S008-P01-007 can run in parallel after workspace manifest exists.
- **Phase 2**: Providers (T-S008-P02-012–T-S008-P02-016) can divide across engineers once bootstrap (T-S008-P02-008–T-S008-P02-010) is ready.
- **User Stories**: After foundational work, US-001 and US-002 can proceed concurrently with coordination on shared navigation; US-003 and US-004 can begin when relevant API clients exist.
- **Tests**: Contract/unit/e2e tasks marked [P] can run simultaneously to shorten feedback loops.
- **Cross-Cutting**: Storybook tasks (T-S008-P02-020, T-S008-P03-036, T-S008-P05-058, T-S008-P07-073) and performance work (T-S008-P07-074–T-S008-P07-076) can continue in parallel once dependent components land.

---

## Parallel Example: User Story 1

```bash
# Parallelize contract + e2e coverage once fixtures exist
Task T-S008-P03-021 → "web/portal/tests/contract/admin-portal.pact.ts"
Task T-S008-P03-022 → "web/portal/tests/e2e/admin-workflows.spec.ts"

# Parallelize API module implementations
Task T-S008-P03-024 → "web/portal/src/features/admin/api/organization.ts"
Task T-S008-P03-025 → "web/portal/src/features/admin/api/members.ts"
Task T-S008-P03-026 → "web/portal/src/features/admin/api/budgets.ts"
Task T-S008-P03-027 → "web/portal/src/features/admin/api/apiKeys.ts"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - blocks all stories)
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: Test User Story 1 independently
5. Deploy/demo if ready

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test independently → Deploy/Demo (MVP!)
3. Add User Story 2 → Test independently → Deploy/Demo
4. Add User Story 3 → Test independently → Deploy/Demo
5. Add User Story 4 → Test independently → Deploy/Demo

### Parallel Team Strategy

1. Shared shell team handles Phases 1–2.  
2. Once Foundational is done:
   - Developer A: User Story 1
   - Developer B: User Story 2
   - Developer C: User Story 3
3. Story owners collaborate on Phase 7 polish once their slice is complete.

