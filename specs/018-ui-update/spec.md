# Feature Specification: UI Update & Admin Portal Rebuild

**Feature Branch**: `018-ui-update`  
**Created**: 2025-01-27  
**Status**: Draft  
**Input**: Rebuild the web portal with stable architecture, centralized configuration, Linode-inspired UI design, and comprehensive Playwright test coverage. Address re-rendering issues, eliminate duplicated URL logic, standardize HTTP client usage, and implement system-aware theming.

## Clarifications

### Session 2025-01-27

- Q: What is the scope of pages to rebuild? → A: Login Page, Home/Dashboard, Admin Pages (API Keys, Members, Organization), Usage Dashboard
- Q: What design system should be used? → A: Linode Cloud Manager-inspired design with system-aware theming (light/dark mode)
- Q: What testing strategy is required? → A: Comprehensive Playwright E2E tests with smoke tests (< 2 minutes) and full integration tests, run against remote dev cluster
- Q: Should the UI cover CLI commands? → A: Yes, the UI MUST provide equivalent functionality to all ai-aas-cli command groups and commands. Users should be able to perform all CLI operations via the web portal.
- Q: Which areas are explicitly out of scope? → A: Backend API changes (UI is client-only), new features beyond existing functionality, mobile-specific optimizations

## User Scenarios & Testing *(mandatory)*

### User Story 1 (US-001) - Stable foundation architecture (Priority: P1)

A developer can work with a stable, centralized architecture where API configuration is computed once, HTTP clients are standardized, and there are no re-rendering issues or duplicated logic.

**Why this priority**: Foundation stability is critical - all other work depends on reliable configuration and HTTP client behavior. Without this, the portal will continue to have reliability issues.

**Independent Test**: Can be fully tested by creating the centralized config module, updating HTTP clients, and verifying that all existing API calls continue to work without modification. This delivers immediate value: a single source of truth for configuration and consistent HTTP behavior across the application.

**Acceptance Scenarios**:

1. **[Primary]** **Given** the application starts, **When** API configuration is accessed, **Then** URLs are computed once at startup and remain immutable throughout the session.
2. **[Primary]** **Given** an authenticated API request, **When** the request is made via `httpClient`, **Then** the request includes correlation ID, authorization header, and proper error handling (401 redirect).
3. **[Primary]** **Given** an unauthenticated API request (login, health check), **When** the request is made via `publicClient`, **Then** the request includes correlation ID but no authorization header.
4. **[Exception]** **Given** a 401 response from an authenticated request, **When** the error is received, **Then** the user is automatically redirected to login and session storage is cleared.
5. **[Alternate]** **Given** the application runs in different environments (localhost, nip.io, production domain), **When** configuration is accessed, **Then** the correct API URLs are computed for each environment without code changes.
6. **[Recovery]** **Given** token refresh is scheduled, **When** the refresh token expires or refresh fails, **Then** the user is logged out gracefully without errors.

---

### User Story 2 (US-002) - Linode-inspired design system (Priority: P2)

A user can access the admin portal with a modern, professional UI that matches Linode Cloud Manager aesthetics, supports system-aware light/dark mode, and provides consistent reusable components.

**Why this priority**: Design system provides the visual foundation and user experience. While not blocking functionality, it significantly improves usability and professional appearance.

**Independent Test**: Can be fully tested by implementing the theme configuration, creating base layout components (Sidebar, Header, Footer), and verifying that pages render correctly in both light and dark modes. This delivers value even without page implementations by providing a consistent design language.

**Acceptance Scenarios**:

1. **[Primary]** **Given** a user's system preference is dark mode, **When** the portal loads, **Then** the portal automatically uses dark theme with Linode-style colors.
2. **[Primary]** **Given** a user's system preference is light mode, **When** the portal loads, **Then** the portal automatically uses light theme with Linode-style colors.
3. **[Alternate]** **Given** a user manually toggles theme preference, **When** the preference is changed, **Then** the preference is stored in localStorage and persists across sessions.
4. **[Primary]** **Given** the portal layout, **When** a user views any page, **Then** the page includes a collapsible sidebar, header with user menu, and consistent content area spacing.
5. **[Primary]** **Given** reusable UI components (DataTable, StatusBadge, Card), **When** components are used across pages, **Then** they render consistently with proper styling in both themes.

---

### User Story 3 (US-003) - Rebuilt portal pages (Priority: P2)

A user can access and interact with all portal pages (Login, Dashboard, Admin pages, Usage Dashboard) with stable behavior, proper loading states, and error handling.

**Why this priority**: Core functionality - users need to access and use the portal pages. This builds on the foundation (US-001) and design system (US-002) to deliver complete user journeys.

**Independent Test**: Can be fully tested by rebuilding each page with the new architecture, verifying that all existing functionality works, and ensuring no console errors or warnings. This delivers value incrementally as each page is completed.

**Acceptance Scenarios**:

1. **[Primary]** **Given** a user visits the login page, **When** the page loads, **Then** the service health check displays without rapid flickering, and login form is functional.
2. **[Primary]** **Given** valid credentials, **When** a user submits the login form, **Then** authentication succeeds, token is stored, and user is redirected to dashboard.
3. **[Primary]** **Given** an authenticated user, **When** they navigate to API Keys page, **Then** the page displays API keys in a sortable DataTable with create/edit/revoke functionality.
4. **[Primary]** **Given** an authenticated admin user, **When** they navigate to Members page, **Then** the page displays organization members with invite/role management/remove functionality.
5. **[Primary]** **Given** an authenticated user, **When** they navigate to Usage Dashboard, **Then** the page displays usage metrics with filterable charts and export functionality.
6. **[Exception]** **Given** an API request fails, **When** an error occurs, **Then** the user sees a toast notification with error details and the UI remains stable.

---

### User Story 4 (US-004) - Comprehensive E2E test coverage (Priority: P3)

A developer can run comprehensive Playwright E2E tests locally and in CI that verify critical paths work correctly against the remote development cluster.

**Why this priority**: Testing ensures quality and prevents regressions. While important, it can be implemented after core functionality is stable.

**Independent Test**: Can be fully tested by creating smoke tests that run against the remote dev cluster, verifying that critical paths (login, navigation, API keys) work end-to-end. This delivers value by providing confidence in deployments and catching regressions early.

**Acceptance Scenarios**:

1. **[Primary]** **Given** smoke tests are configured, **When** tests run against remote dev cluster, **Then** all smoke tests pass in under 2 minutes.
2. **[Primary]** **Given** integration tests are configured, **When** tests run against remote dev cluster, **Then** all integration tests pass across Chromium, Firefox, and WebKit browsers.
3. **[Primary]** **Given** a test failure occurs, **When** tests complete, **Then** screenshots and videos are captured for debugging.
4. **[Alternate]** **Given** CI pipeline is configured, **When** code is pushed to feature branch, **Then** smoke tests run automatically and block deployment on failure.
5. **[Exception]** **Given** a flaky test, **When** test is rerun 3 times, **Then** test passes consistently (no flakiness).

---

### User Story 5 (US-005) - Code cleanup and documentation (Priority: P3)

A developer can work with clean, well-documented codebase where dead code is removed, documentation is updated, and architecture is clearly explained.

**Why this priority**: Maintainability and developer experience. Important for long-term health but not blocking for functionality.

**Independent Test**: Can be fully tested by removing duplicated URL logic, unused providers, and native fetch calls, then verifying that all functionality still works. This delivers value by reducing technical debt and improving code clarity.

**Acceptance Scenarios**:

1. **[Primary]** **Given** the codebase after page implementation, **When** dead code removal is performed, **Then** all duplicated URL logic is removed and replaced with centralized config usage.
2. **[Primary]** **Given** the codebase, **When** documentation is updated, **Then** README includes new architecture explanation, test running procedures, and environment configuration.
3. **[Primary]** **Given** unused providers exist, **When** cleanup is performed, **Then** unused providers are removed and provider tree is flattened to 4 levels max.

---

### User Story 6 (US-006) - CLI command coverage in UI (Priority: P2)

A user can perform all ai-aas-cli operations via the web portal UI, with equivalent functionality for all command groups (Model Management, Access Control, Platform Operations, Utilities).

**Why this priority**: The UI should provide feature parity with the CLI tool. Users who prefer GUI over CLI should have access to all platform capabilities. This ensures the portal is a complete management interface, not just a subset of functionality.

**Independent Test**: Can be fully tested by implementing UI pages for each CLI command group and verifying that all operations available via CLI are also available via UI. This delivers value by providing a complete web-based management interface that matches CLI capabilities.

**Acceptance Scenarios**:

1. **[Primary]** **Given** an authenticated admin user, **When** they navigate to Model Management section, **Then** they can perform all model operations (registry, cache, deploy, troubleshoot, version, library) equivalent to `ai-aas-cli model` commands.
2. **[Primary]** **Given** an authenticated admin user, **When** they navigate to Access Control section, **Then** they can manage organizations, users, API keys, and user model access equivalent to `ai-aas-cli org`, `ai-aas-cli user`, `ai-aas-cli apikey` commands.
3. **[Primary]** **Given** an authenticated admin user, **When** they navigate to Platform Operations section, **Then** they can perform bootstrap, registry management, routing policies, sync operations, and credentials management equivalent to platform CLI commands.
4. **[Primary]** **Given** an authenticated user, **When** they navigate to Utilities section, **Then** they can view platform status, manage CLI configuration, and export reports equivalent to `ai-aas-cli status`, `ai-aas-cli config`, `ai-aas-cli export` commands.
5. **[Primary]** **Given** a user performs an operation via UI, **When** the operation completes, **Then** the result matches what would be returned by the equivalent CLI command.
6. **[Alternate]** **Given** a user views a list page (e.g., models, organizations, users), **When** data is displayed, **Then** the UI shows the same information that would be shown by the equivalent CLI `list` command with table format.
7. **[Exception]** **Given** a CLI command requires complex flags or file inputs, **When** the operation is performed via UI, **Then** the UI provides equivalent form inputs or file upload capabilities.

---

### Edge Cases

- What happens when the application runs in an environment with no network connectivity? → Health check should show unhealthy status, but UI should remain functional for cached content.
- How does the system handle token refresh failures? → User is logged out gracefully, session storage is cleared, and user is redirected to login with a clear error message.
- What happens when API responses are slow (> 30 seconds)? → Requests timeout with proper error handling, and users see loading states with timeout messages.
- How does the system handle rapid theme switching? → Theme changes are debounced (300ms delay) to prevent rapid toggling, and localStorage updates are batched to prevent flickering. The `useTheme` hook implements debounce logic using a timeout that cancels previous pending updates.
- What happens when Playwright tests run against a down remote cluster? → Tests fail gracefully with clear error messages, and CI reports the failure without blocking other jobs.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST centralize API URL configuration in a single module (`/src/config/api.ts`) that computes URLs once at startup.
- **FR-002**: System MUST provide two HTTP clients: `httpClient` (authenticated) and `publicClient` (unauthenticated), both with correlation ID interceptors.
- **FR-003**: System MUST implement `TokenManager` class to handle token refresh logic without closure issues.
- **FR-004**: System MUST flatten provider hierarchy to maximum 4 levels and combine providers where possible.
- **FR-005**: System MUST implement system-aware theming (light/dark mode) that respects `prefers-color-scheme` and stores manual overrides in localStorage.
- **FR-006**: System MUST provide Linode-inspired color palette for both light and dark modes with consistent brand colors.
- **FR-007**: System MUST include base layout components: Sidebar (collapsible), Header (search, user menu), ContentArea, Footer (version, API reference).
- **FR-008**: System MUST provide reusable UI components: DataTable (sortable, paginated), StatusBadge (traffic light indicators), TabNav, Card, FilterPanel, LoadingSpinner.
- **FR-009**: System MUST rebuild Login Page with stable ServiceHealthCheck (non-React singleton pattern) and proper error handling.
- **FR-010**: System MUST rebuild Home/Dashboard Page with quick stats cards, recent activity list, and system status overview.
- **FR-011**: System MUST rebuild API Keys Page with DataTable, create/edit modals, view scopes modal, and revoke confirmation.
- **FR-012**: System MUST rebuild Members Page with DataTable, invite member modal, role management, and remove member confirmation.
- **FR-013**: System MUST rebuild Organization Page with organization details card, settings form, and budget configuration.
- **FR-014**: System MUST rebuild Usage Dashboard with filter panel (date range, model), usage charts, usage breakdown table, and export functionality.
- **FR-015**: System MUST implement Playwright smoke tests that execute in under 2 minutes and cover critical paths (login, navigation, API keys).
- **FR-016**: System MUST implement Playwright integration tests for all pages (login, API keys, members, usage) across Chromium, Firefox, and WebKit.
- **FR-017**: System MUST provide test utilities: auth helpers, page object models, and test data fixtures.
- **FR-018**: System MUST integrate Playwright tests into CI/CD pipeline with smoke test stage and test artifact collection.
- **FR-019**: System MUST remove all duplicated URL computation logic from application code.
- **FR-020**: System MUST remove all native `fetch()` calls and replace with `publicClient` or `httpClient`.
- **FR-021**: System MUST remove all raw `axios` usage and replace with standardized HTTP clients.
- **FR-022**: System MUST provide UI pages for Model Management command group covering: model registry (add, list, info, remove), model cache (pull, list, delete, gc), model deploy (create, delete, scale, status), model troubleshoot (logs, events, test), model version (check, update, pin), and model library (enable, disable, swap, list, history, alias).
- **FR-023**: System MUST provide UI pages for Access Control command group covering: organizations (list, create, update, delete), users (list, create, update, delete, model-access), and API keys (list, create, delete).
- **FR-024**: System MUST provide UI pages for Platform Operations command group covering: bootstrap, registry (register, deregister, enable, disable, list), routing (policy create/list/delete), sync (trigger, status), and credentials (set, list, test, delete).
- **FR-025**: System MUST provide UI pages for Utilities command group covering: status (platform health), config (show, set, test), and export (usage, memberships).
- **FR-026**: System MUST ensure UI operations produce equivalent results to corresponding CLI commands (same API calls, same data format, same validation rules).
- **FR-027**: System MUST display CLI command syntax or equivalent operation name in UI where applicable. CLI command tooltips or equivalent operation names MUST appear on: (1) action buttons that perform CLI-equivalent operations (e.g., "Create Deployment" button shows "Equivalent to: ai-aas-cli model deploy create"), (2) page headers for CLI command group pages, (3) help text or info icons for complex operations. Tooltips are optional for simple CRUD operations where the UI action is self-explanatory.

### Non-Functional Requirements

- **NFR-001**: Smoke tests MUST execute in under 2 minutes total.
- **NFR-002**: Integration tests MUST execute in under 15 minutes total.
- **NFR-003**: Login page MUST load in under 3 seconds.
- **NFR-004**: Service health check MUST display without visible flickering or rapid re-rendering.
- **NFR-005**: API responses MUST complete in under 1 second for typical requests.
- **NFR-006**: Tests MUST support execution against remote dev cluster (`portal.172.232.58.222.nip.io`).
- **NFR-007**: Tests MUST have > 95% success rate (no flaky tests after 3 retries).
- **NFR-008**: Portal MUST work correctly in localhost, nip.io, and production domain environments.
- **NFR-009**: Portal MUST support system-aware theming with automatic light/dark mode detection.
- **NFR-010**: Portal MUST maintain TypeScript strict mode compliance (no `any` types).
- **NFR-011**: Portal MUST have zero console errors or warnings in production builds.
- **NFR-012**: Portal MUST be accessible (WCAG 2.1 AA compliance for admin portal use cases).
- **NFR-013**: UI MUST provide feature parity with ai-aas-cli tool (all CLI command groups and commands must have UI equivalents).
- **NFR-014**: UI operations MUST use the same backend APIs as CLI commands (no separate UI-only endpoints).

### Key Entities *(include if feature involves data)*

- **ApiConfig**: Centralized configuration object containing `baseUrl`, `apiUrl`, `isNipIo`, and `environment`. Computed once at startup, immutable thereafter.
- **TokenManager**: Service class managing token refresh scheduling, cancellation, and logout callbacks. Handles refresh logic without React closure issues.
- **ServiceHealth**: Status information for backend services (API Gateway, CORS) with health status and optional latency metrics.
- **Theme**: User theme preference (system, light, dark) stored in localStorage with manual override support.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All smoke tests pass consistently (> 95% success rate) and execute in under 2 minutes.
- **SC-002**: All integration tests pass across Chromium, Firefox, and WebKit browsers with > 90% success rate.
- **SC-003**: Login page loads in under 3 seconds with no visible flickering on service health check.
- **SC-004**: All API calls use centralized configuration and standardized HTTP clients (zero duplicated URL logic, zero native fetch calls, zero raw axios usage).
- **SC-005**: Portal renders correctly in both light and dark modes with Linode-inspired color palette.
- **SC-006**: All pages (Login, Dashboard, Admin pages, Usage Dashboard) function correctly with proper loading states and error handling.
- **SC-007**: Provider hierarchy is flattened to maximum 4 levels with no unused providers.
- **SC-008**: Zero console errors or warnings in production builds.
- **SC-009**: Code coverage for critical paths (login, API keys, members) is > 80% via E2E tests.
- **SC-010**: Documentation is updated with new architecture explanation, test procedures, and environment configuration.
- **SC-011**: All CLI command groups (Model Management, Access Control, Platform Operations, Utilities) have UI pages with equivalent functionality.
- **SC-012**: UI operations produce identical results to CLI commands (verified via comparison testing).

### Definition of Done

Each task is complete when:
1. Code is implemented and passes TypeScript compilation with strict mode
2. Relevant unit/integration tests pass locally
3. Smoke tests pass against remote dev cluster
4. No console errors or warnings
5. Code is committed and pushed to feature branch
6. ArgoCD successfully deploys to development environment
7. Manual testing confirms functionality works as expected

---

## Out of Scope

- Backend API changes (UI is client-only, uses existing REST APIs)
- New features beyond existing functionality (this is a rebuild, not feature addition)
- Mobile-specific optimizations (admin portal is desktop-focused)
- Performance optimizations beyond basic requirements (login < 3s, API < 1s)
- Accessibility improvements beyond WCAG 2.1 AA for admin portal use cases
- Internationalization (i18n) support
- Real-time features (WebSockets, SSE)

---

## Dependencies

### External Dependencies
- Existing backend REST APIs (no changes required)
- Playwright test framework
- TailwindCSS and shadcn/ui component library
- React 18, TypeScript, Vite (existing stack)

### Internal Dependencies
- Feature branch: `feature/018-ui-update`
- Remote dev cluster access for testing
- Seeded test users in development environment

---

## Assumptions

1. Backend APIs remain stable and unchanged during UI rebuild
2. Existing authentication flow (email/password, OAuth) continues to work
3. Test users (`admin@example-acme.com`, `member@example-acme.com`) are seeded in dev environment
4. Remote dev cluster (`portal.172.232.58.222.nip.io`) is accessible for testing
5. CI/CD pipeline can be updated to include Playwright smoke tests
6. ArgoCD deployment to development environment is available

---

## Traceability Matrix

| User Story | Functional Requirements | Success Criteria |
|------------|------------------------|------------------|
| US-001 | FR-001, FR-002, FR-003, FR-004, FR-019, FR-020, FR-021 | SC-004, SC-007 |
| US-002 | FR-005, FR-006, FR-007, FR-008 | SC-005 |
| US-003 | FR-009, FR-010, FR-011, FR-012, FR-013, FR-014 | SC-006, SC-008 |
| US-004 | FR-015, FR-016, FR-017, FR-018 | SC-001, SC-002, SC-009 |
| US-005 | FR-019, FR-020, FR-021 | SC-010 |
| US-006 | FR-022, FR-023, FR-024, FR-025, FR-026, FR-027 | SC-011, SC-012 |

---

## Notes

- This specification focuses on UI rebuild and architecture stabilization, not new features
- Design system is inspired by Linode Cloud Manager but adapted for AI-AAS platform branding
- Testing strategy prioritizes E2E tests over unit tests for admin portal (fewer, slower, more valuable)
- Architecture recommendations from `architecture-recommendations.md` are incorporated into requirements
- UI must provide feature parity with ai-aas-cli tool - all CLI command groups and commands must have UI equivalents

## CLI Command Coverage Reference

The UI must cover all command groups and commands from `ai-aas-cli`. Reference structure:

### Model Management Group (`model`)
- **model registry**: add, list, info, remove
- **model cache**: pull, list, delete, gc
- **model deploy**: create, delete, scale, status
- **model troubleshoot**: logs, events, test
- **model version**: check, update, pin
- **model library**: enable, disable, swap, list, history, alias
- **deployment**: status (multi-source inspection)
- **inference**: get-models, send-request

### Access Control Group (`access`)
- **org**: list, create, update, delete
- **user**: list, create, update, delete
- **user model-access**: show, set-mode, grant, revoke, list, grant-all, migrate
- **apikey**: list, create, delete

### Platform Operations Group (`platform`)
- **bootstrap**: Create first admin account
- **registry**: register, deregister, enable, disable, list
- **routing policy**: create, list, delete
- **sync**: trigger, status
- **credentials**: set, list, test, delete (hf-token, s3)

### Utilities Group (`util`)
- **status**: Platform health status
- **config**: show, set, test, check-path
- **export**: usage, memberships

**Note**: UI pages should be organized by command group in the navigation structure, with sub-pages or tabs for individual commands within each group.

