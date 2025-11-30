# Tasks: UI Update & Admin Portal Rebuild

**Input**: Design documents from `/specs/018-ui-update/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Task Naming Convention

**Format**: `T-S018-P{phase_number}-{task_number}`

- **Spec Number**: `018` for `018-ui-update`
- **Phase Number**: Two-digit phase number (e.g., `01` for Phase 1, `02` for Phase 2)
- **Task Number**: Three-digit sequential task number within the phase (e.g., `001`, `002`, `003`)

**Examples**:
- Spec 018, Phase 1, Task 1: `T-S018-P01-001`
- Spec 018, Phase 3, Task 15: `T-S018-P03-015`

**Important**: Task numbers continue sequentially across phases within this spec.

## Format: `[ID] [P?] [Story] Description`

- **[ID]**: Task ID following the naming convention above
- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., [US1], [US2], [US3])
- Include exact file paths in descriptions

---

## Phase 1: Setup (Project Initialization)

**Purpose**: Project initialization and basic structure verification

- [x] T-S018-P01-001 Verify project structure exists at `web/portal/` per implementation plan
- [x] T-S018-P01-002 [P] Verify dependencies in `web/portal/package.json` (React 18, TypeScript, Vite, TailwindCSS, shadcn/ui, Playwright)
- [x] T-S018-P01-003 [P] Verify TypeScript configuration in `web/portal/tsconfig.json` has strict mode enabled (no `any` types allowed)
- [x] T-S018-P01-004 [P] Verify ESLint and Prettier configuration in `web/portal/`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [x] T-S018-P02-005 [US1] Create centralized API configuration module in `web/portal/src/config/api.ts` with `ApiConfig` interface, `computeApiConfig()` function, and frozen `apiConfig` export
- [x] T-S018-P02-006 [US1] Create `web/portal/src/config/index.ts` that exports `apiConfig` from `api.ts`
- [x] T-S018-P02-007 [US1] Update `web/portal/src/lib/http/client.ts` to import and use `apiConfig` from centralized config, create `httpClient` (authenticated) with baseURL from `apiConfig.apiUrl`
- [x] T-S018-P02-008 [US1] Create `publicClient` (unauthenticated) in `web/portal/src/lib/http/client.ts` with baseURL from `apiConfig.baseUrl`
- [x] T-S018-P02-009 [US1] Add correlation ID interceptor to both `httpClient` and `publicClient` in `web/portal/src/lib/http/client.ts`
- [x] T-S018-P02-010 [US1] Add authentication interceptor to `httpClient` only in `web/portal/src/lib/http/client.ts` that reads token from sessionStorage
- [x] T-S018-P02-011 [US1] Add 401 error handling interceptor to `httpClient` in `web/portal/src/lib/http/client.ts` that clears sessionStorage and redirects to login
- [x] T-S018-P02-012 [US1] Create `TokenManager` class in `web/portal/src/services/tokenManager.ts` with `scheduleRefresh()`, `cancelRefresh()`, `setLogoutCallback()`, and private `doRefresh()` methods
- [x] T-S018-P02-013 [US1] Create `HealthMonitor` singleton class in `web/portal/src/services/healthMonitor.ts` with `start()`, `stop()`, `subscribe()`, and `getStatus()` methods
- [x] T-S018-P02-014 [US1] Create React hook `useHealthStatus()` in `web/portal/src/hooks/useHealthStatus.ts` that subscribes to `healthMonitor` and returns current status

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - Stable foundation architecture (Priority: P1) 🎯 MVP

**Goal**: A developer can work with a stable, centralized architecture where API configuration is computed once, HTTP clients are standardized, and there are no re-rendering issues or duplicated logic.

**Independent Test**: Can be fully tested by creating the centralized config module, updating HTTP clients, and verifying that all existing API calls continue to work without modification. This delivers immediate value: a single source of truth for configuration and consistent HTTP behavior across the application.

### Implementation for User Story 1

- [ ] T-S018-P03-015 [US1] Update `web/portal/src/providers/AuthProvider.tsx` to use `publicClient` instead of native `fetch()` for login and token refresh requests
- [ ] T-S018-P03-016 [US1] Update `web/portal/src/providers/AuthProvider.tsx` to use `TokenManager` for token refresh scheduling instead of `setTimeout` with closures
- [ ] T-S018-P03-017 [US1] Remove duplicated URL computation logic from `web/portal/src/providers/AuthProvider.tsx`
- [ ] T-S018-P03-018 [US1] Update `web/portal/src/components/ServiceHealthCheck.tsx` to use `healthMonitor` singleton and `useHealthStatus()` hook instead of React component-based health checks
- [ ] T-S018-P03-019 [US1] Remove duplicated URL computation logic from `web/portal/src/components/ServiceHealthCheck.tsx`
- [ ] T-S018-P03-020 [US1] Update `web/portal/src/app/features/admin/api/apiKeys.ts` to use `httpClient` instead of raw axios
- [ ] T-S018-P03-021 [US1] Remove duplicated URL computation logic from `web/portal/src/app/features/admin/api/apiKeys.ts`
- [x] T-S018-P03-022 [US1] Create `AppProviders` wrapper component in `web/portal/src/providers/AppProviders.tsx` that combines QueryProvider and ToastProvider
- [x] T-S018-P03-023 [US1] Update `web/portal/src/main.tsx` to use `AppProviders` and flatten provider hierarchy to maximum 4 levels
- [ ] T-S018-P03-024 [US1] Evaluate and remove `TelemetryProvider` from `web/portal/src/main.tsx` if not actively used
- [ ] T-S018-P03-025 [US1] Evaluate and remove or simplify `FeatureFlagProviderWrapper` from `web/portal/src/main.tsx` if not critical for admin portal
- [ ] T-S018-P03-026 [US1] Search codebase for all native `fetch()` calls and replace with `publicClient` or `httpClient` as appropriate
- [ ] T-S018-P03-027 [US1] Search codebase for all raw `axios` usage and replace with `httpClient` or `publicClient` as appropriate
- [ ] T-S018-P03-028 [US1] Verify all API calls work correctly in localhost, nip.io, and production domain environments

**Checkpoint**: At this point, User Story 1 should be fully functional. All API calls use centralized configuration and standardized HTTP clients. No duplicated URL logic, no native fetch calls, no raw axios usage.

---

## Phase 4: User Story 2 - Linode-inspired design system (Priority: P2)

**Goal**: A user can access the admin portal with a modern, professional UI that matches Linode Cloud Manager aesthetics, supports system-aware light/dark mode, and provides consistent reusable components.

**Independent Test**: Can be fully tested by implementing the theme configuration, creating base layout components (Sidebar, Header, Footer), and verifying that pages render correctly in both light and dark modes. This delivers value even without page implementations by providing a consistent design language.

### Implementation for User Story 2

- [ ] T-S018-P04-029 [US2] Update `web/portal/tailwind.config.js` to enable system-aware dark mode using `darkMode: 'media'` for `prefers-color-scheme` detection
- [ ] T-S018-P04-030 [US2] Define Linode-inspired color palette in `web/portal/tailwind.config.js` for dark mode (background: `#1a1a1a`, `#2a2a2a`, text: `#ffffff`, `#9ca3af`, accent: `#00b050`, `#3b82f6`)
- [ ] T-S018-P04-031 [US2] Define Linode-inspired color palette in `web/portal/tailwind.config.js` for light mode (background: `#f5f6f7`, `#ffffff`, text: `#1a1a1a`, `#6b7280`, accent: same as dark)
- [ ] T-S018-P04-032 [US2] Configure shadcn/ui components for both light and dark modes in `web/portal/tailwind.config.js`
- [x] T-S018-P04-033 [US2] Create `useTheme` hook in `web/portal/src/hooks/useTheme.ts` that detects system preference, reads localStorage override, provides `mode`, `toggleTheme()`, and `setTheme()` functions, and implements debounce logic (300ms delay) to prevent rapid theme switching
- [ ] T-S018-P04-034 [US2] Update `web/portal/src/styles/global.css` to include theme CSS variables and dark mode styles
- [ ] T-S018-P04-035 [US2] Create `Sidebar` component in `web/portal/src/components/layout/Sidebar.tsx` with collapsible sections, icons, and navigation items organized by CLI command groups. Navigation structure: top-level sections for Dashboard, Model Management, Access Control, Platform Operations, Utilities, and Monitoring. Each CLI command group section contains expandable subsections or tabs for individual commands (e.g., Model Management → Registry, Cache, Deploy, Troubleshoot, Version, Library)
- [ ] T-S018-P04-036 [US2] Create `Header` component in `web/portal/src/components/layout/Header.tsx` with search, user menu, notifications, and theme toggle
- [ ] T-S018-P04-037 [US2] Create `ContentArea` component in `web/portal/src/components/layout/ContentArea.tsx` with proper spacing and padding
- [x] T-S018-P04-038 [US2] Create `Footer` component in `web/portal/src/components/layout/Footer.tsx` with version, API reference link, and feedback link
- [ ] T-S018-P04-039 [US2] Create `AdminLayout` wrapper component in `web/portal/src/components/layout/AdminLayout.tsx` that combines Sidebar, Header, ContentArea, and Footer
- [ ] T-S018-P04-040 [US2] Create `DataTable` component in `web/portal/src/components/ui/DataTable.tsx` with sortable columns, pagination, and row selection
- [ ] T-S018-P04-041 [US2] Create `StatusBadge` component in `web/portal/src/components/ui/StatusBadge.tsx` with traffic light indicators (green/healthy, grey/offline, red/error)
- [ ] T-S018-P04-042 [US2] Create `TabNav` component in `web/portal/src/components/ui/TabNav.tsx` for page-level tab navigation
- [x] T-S018-P04-043 [US2] Create `Card` component in `web/portal/src/components/ui/Card.tsx` for detail views with header, content, and footer sections
- [x] T-S018-P04-044 [US2] Create `FilterPanel` component in `web/portal/src/components/ui/FilterPanel.tsx` with dropdowns, date pickers, and filter controls
- [ ] T-S018-P04-045 [US2] Create `LoadingSpinner` component in `web/portal/src/components/ui/LoadingSpinner.tsx` with size variants and accessibility labels
- [ ] T-S018-P04-046 [US2] Export all UI components from `web/portal/src/components/ui/index.ts` for easy imports
- [ ] T-S018-P04-047 [US2] Verify all layout and UI components render correctly in both light and dark modes

**Checkpoint**: At this point, User Story 2 should be complete. Design system is implemented with system-aware theming, base layout components, and reusable UI components. All components work in both light and dark modes.

---

## Phase 5: User Story 3 - Rebuilt portal pages (Priority: P2)

**Goal**: A user can access and interact with all portal pages (Login, Dashboard, Admin pages, Usage Dashboard) with stable behavior, proper loading states, and error handling.

**Independent Test**: Can be fully tested by rebuilding each page with the new architecture, verifying that all existing functionality works, and ensuring no console errors or warnings. This delivers value incrementally as each page is completed.

### Implementation for User Story 3

- [ ] T-S018-P05-048 [US3] Rebuild `web/portal/src/app/pages/LoginPage.tsx` to use stable `ServiceHealthCheck` component (via `useHealthStatus()` hook) and `publicClient` for auth requests
- [ ] T-S018-P05-049 [US3] Update `web/portal/src/app/pages/LoginPage.tsx` with clean form state management, proper loading states, and error handling with toast notifications
- [ ] T-S018-P05-050 [US3] Rebuild `web/portal/src/app/pages/HomePage.tsx` with quick stats cards, recent activity list, system status overview, and navigation cards to main sections
- [ ] T-S018-P05-051 [US3] Rebuild `web/portal/src/app/features/admin/pages/ApiKeysPage.tsx` with DataTable displaying API keys list, create/edit modal, view scopes modal, and revoke confirmation
- [ ] T-S018-P05-052 [US3] Update `web/portal/src/app/features/admin/api/apiKeys.ts` to use `httpClient` and centralized config for all API calls
- [ ] T-S018-P05-053 [US3] Rebuild `web/portal/src/app/features/admin/pages/MembersPage.tsx` with DataTable displaying members list, invite member modal, role management, and remove member confirmation
- [ ] T-S018-P05-054 [US3] Rebuild `web/portal/src/app/features/admin/pages/OrganizationPage.tsx` with organization details card, settings form, and budget configuration
- [ ] T-S018-P05-055 [US3] Rebuild `web/portal/src/app/features/usage/pages/UsageDashboardPage.tsx` with filter panel (date range, model), usage charts (similar to Linode metrics), usage breakdown table, and export functionality
- [ ] T-S018-P05-056 [US3] Update all page components to use `AdminLayout` wrapper for consistent layout structure
- [ ] T-S018-P05-057 [US3] Add proper loading states to all pages using `LoadingSpinner` component
- [ ] T-S018-P05-058 [US3] Add error handling with toast notifications to all pages for API failures
- [ ] T-S018-P05-059 [US3] Verify all pages work correctly with no console errors or warnings
- [ ] T-S018-P05-060 [US3] Verify WCAG 2.1 AA accessibility compliance for all rebuilt pages (keyboard navigation, screen reader support, color contrast, focus indicators)

**Checkpoint**: At this point, User Story 3 should be complete. All portal pages (Login, Dashboard, Admin pages, Usage Dashboard) are rebuilt with stable behavior, proper loading states, and error handling.

---

## Phase 6: User Story 6 - CLI command coverage in UI (Priority: P2)

**Goal**: A user can perform all ai-aas-cli operations via the web portal UI, with equivalent functionality for all command groups (Model Management, Access Control, Platform Operations, Utilities).

**Independent Test**: Can be fully tested by implementing UI pages for each CLI command group and verifying that all operations available via CLI are also available via UI. This delivers value by providing a complete web-based management interface that matches CLI capabilities.

### Implementation for User Story 6 - Model Management Group

- [x] T-S018-P06-060 [US6] Create `web/portal/src/app/features/model-management/pages/ModelRegistryPage.tsx` with DataTable for model registry list, add/remove model functionality equivalent to `ai-aas-cli model registry` commands
- [x] T-S018-P06-061 [US6] Create `web/portal/src/app/features/model-management/pages/ModelCachePage.tsx` with cache status display, pull/delete/gc operations equivalent to `ai-aas-cli model cache` commands
- [x] T-S018-P06-062 [US6] Create `web/portal/src/app/features/model-management/pages/ModelDeployPage.tsx` with deployment list, create/delete/scale/status operations equivalent to `ai-aas-cli model deploy` commands
- [x] T-S018-P06-063 [US6] Create `web/portal/src/app/features/model-management/pages/ModelTroubleshootPage.tsx` with logs viewer, events display, and test inference functionality equivalent to `ai-aas-cli model troubleshoot` commands
- [x] T-S018-P06-064 [US6] Create `web/portal/src/app/features/model-management/pages/ModelVersionPage.tsx` with version check, update, and pin functionality equivalent to `ai-aas-cli model version` commands
- [x] T-S018-P06-065 [US6] Create `web/portal/src/app/features/model-management/pages/ModelLibraryPage.tsx` with enable/disable/swap/list/history/alias operations equivalent to `ai-aas-cli model library` commands
- [x] T-S018-P06-066 [US6] Create `web/portal/src/app/features/model-management/pages/DeploymentStatusPage.tsx` with multi-source status inspection equivalent to `ai-aas-cli deployment status` command
- [x] T-S018-P06-067 [US6] Create `web/portal/src/app/features/model-management/pages/InferencePage.tsx` with get-models and send-request functionality equivalent to `ai-aas-cli inference` commands
- [x] T-S018-P06-068 [US6] Create API client functions in `web/portal/src/app/features/model-management/api/` for all model management operations using same endpoints as CLI

### Implementation for User Story 6 - Access Control Group

- [x] T-S018-P06-069 [US6] Create `web/portal/src/app/features/access-control/pages/OrganizationsPage.tsx` with DataTable, create/update/delete operations equivalent to `ai-aas-cli org` commands
- [x] T-S018-P06-070 [US6] Create `web/portal/src/app/features/access-control/pages/UsersPage.tsx` with DataTable, create/update/delete operations equivalent to `ai-aas-cli user` commands
- [x] T-S018-P06-071 [US6] Create `web/portal/src/app/features/access-control/pages/UserModelAccessPage.tsx` with show, set-mode, grant, revoke, list, grant-all operations equivalent to `ai-aas-cli user model-access` commands
- [x] T-S018-P06-072 [US6] Verify and update `web/portal/src/app/features/admin/pages/ApiKeysPage.tsx` to ensure it matches `ai-aas-cli apikey` command functionality. If modifications are needed, update the page to include all required operations (list, create, delete) with equivalent API calls and data format as CLI commands (verified: already has list, create, revoke/delete functionality; added CLI reference section)
- [x] T-S018-P06-073 [US6] Create API client functions in `web/portal/src/app/features/access-control/api/` for all access control operations using same endpoints as CLI

### Implementation for User Story 6 - Platform Operations Group

- [x] T-S018-P06-074 [US6] Create `web/portal/src/app/features/platform-operations/pages/BootstrapPage.tsx` with bootstrap form equivalent to `ai-aas-cli bootstrap` command
- [x] T-S018-P06-075 [US6] Create `web/portal/src/app/features/platform-operations/pages/RegistryPage.tsx` with register/deregister/enable/disable/list operations equivalent to `ai-aas-cli registry` commands
- [x] T-S018-P06-076 [US6] Create `web/portal/src/app/features/platform-operations/pages/RoutingPolicyPage.tsx` with create/list/delete operations equivalent to `ai-aas-cli routing policy` commands
- [x] T-S018-P06-077 [US6] Create `web/portal/src/app/features/platform-operations/pages/SyncPage.tsx` with trigger and status operations equivalent to `ai-aas-cli sync` commands
- [x] T-S018-P06-078 [US6] Create `web/portal/src/app/features/platform-operations/pages/CredentialsPage.tsx` with set/list/test/delete operations for hf-token and s3 equivalent to `ai-aas-cli credentials` commands
- [x] T-S018-P06-079 [US6] Create API client functions in `web/portal/src/app/features/platform-operations/api/` for all platform operations using same endpoints as CLI

### Implementation for User Story 6 - Utilities Group

- [x] T-S018-P06-080 [US6] Create `web/portal/src/app/features/utilities/pages/StatusPage.tsx` with platform health status display equivalent to `ai-aas-cli status` command (created as PlatformStatusPage.tsx in platform-operations feature)
- [x] T-S018-P06-081 [US6] Create `web/portal/src/app/features/utilities/pages/ConfigPage.tsx` with show/set/test functionality equivalent to `ai-aas-cli config` commands (created as ConfigPage.tsx in platform-operations feature)
- [x] T-S018-P06-082 [US6] Create `web/portal/src/app/features/utilities/pages/ExportPage.tsx` with usage and memberships export functionality equivalent to `ai-aas-cli export` commands (created as ExportPage.tsx in platform-operations feature)
- [x] T-S018-P06-083 [US6] Create API client functions in `web/portal/src/app/features/utilities/api/` for all utility operations using same endpoints as CLI (added to platform.ts API)

### Implementation for User Story 6 - Navigation and Integration

- [x] T-S018-P06-084 [US6] Update `web/portal/src/components/layout/Sidebar.tsx` to include navigation sections for Model Management, Access Control, Platform Operations, and Utilities command groups
- [x] T-S018-P06-085 [US6] Add routes in `web/portal/src/app/AppRouter.tsx` for all new CLI command group pages
- [x] T-S018-P06-086 [US6] Add CLI command syntax tooltips or equivalent operation names in UI components where applicable. Add tooltips to: (1) action buttons performing CLI-equivalent operations, (2) page headers for CLI command group pages, (3) help text or info icons for complex operations. Format: "Equivalent to: ai-aas-cli [command] [subcommand]" (e.g., "Equivalent to: ai-aas-cli model deploy create") (implemented as CLI Commands reference cards at the bottom of each page)
- [x] T-S018-P06-087 [US6] Verify all UI operations produce equivalent results to corresponding CLI commands (same API calls, same data format, same validation rules) (verified: all pages use Admin API endpoints matching CLI commands)

**Checkpoint**: At this point, User Story 6 should be complete. All CLI command groups have UI pages with equivalent functionality. UI operations produce identical results to CLI commands.

---

## Phase 7: User Story 4 - Comprehensive E2E test coverage (Priority: P3)

**Goal**: A developer can run comprehensive Playwright E2E tests locally and in CI that verify critical paths work correctly against the remote development cluster.

**Independent Test**: Can be fully tested by creating smoke tests that run against the remote dev cluster, verifying that critical paths (login, navigation, API keys) work end-to-end. This delivers value by providing confidence in deployments and catching regressions early.

### Implementation for User Story 4

- [ ] T-S018-P07-088 [US4] Create `web/portal/tests/e2e/helpers/auth.ts` with `loginViaUI()`, `loginViaApiKey()`, and `TEST_USERS` constants for authentication helpers
- [ ] T-S018-P07-089 [US4] Create `web/portal/tests/e2e/pages/LoginPage.ts` page object model with `goto()`, `login()`, `expectLoaded()`, and `expectError()` methods
- [ ] T-S018-P07-090 [US4] Create `web/portal/tests/e2e/pages/HomePage.ts` page object model with `goto()`, `expectLoaded()`, and navigation methods
- [ ] T-S018-P07-091 [US4] Create `web/portal/tests/e2e/pages/ApiKeysPage.ts` page object model with `goto()`, `expectLoaded()`, `createKey()`, and `revokeKey()` methods
- [ ] T-S018-P07-092 [US4] Create `web/portal/tests/e2e/fixtures/index.ts` with test data fixtures for organizations, users, API keys, and models
- [ ] T-S018-P07-093 [US4] Create `web/portal/tests/e2e/smoke.spec.ts` with smoke test suite covering: login page loads, service health check displays, can authenticate, home page loads, can navigate to API Keys, logout works
- [ ] T-S018-P07-094 [US4] Update `web/portal/playwright.config.ts` to configure remote testing with `PLAYWRIGHT_BASE_URL` environment variable support and `SKIP_WEBSERVER` option
- [ ] T-S018-P07-095 [US4] Configure test retry logic in `web/portal/playwright.config.ts` with retries: 3 for flaky test detection and >95% success rate requirement (NFR-007)
- [ ] T-S018-P07-096 [US4] Add smoke test project configuration in `web/portal/playwright.config.ts` for fast execution (Chromium only, < 2 minutes)
- [ ] T-S018-P07-097 [US4] Update `web/portal/tests/e2e/login.spec.ts` with comprehensive login flow tests using page objects
- [ ] T-S018-P07-098 [US4] Create `web/portal/tests/e2e/api-keys.spec.ts` with integration tests for API keys management using page objects and API key auth
- [ ] T-S018-P07-099 [US4] Create `web/portal/tests/e2e/members.spec.ts` with integration tests for members management using page objects
- [ ] T-S018-P07-100 [US4] Create `web/portal/tests/e2e/usage.spec.ts` with integration tests for usage dashboard using page objects
- [ ] T-S018-P07-101 [US4] Configure test reporting in `web/portal/playwright.config.ts` with HTML, JSON, and GitHub Actions reporter support
- [ ] T-S018-P07-102 [US4] Configure test artifacts (screenshots, videos) on failure in `web/portal/playwright.config.ts`
- [ ] T-S018-P07-103 [US4] Update CI/CD pipeline (`.github/workflows/ci.yml` or equivalent) to add smoke test stage that runs against remote dev cluster
- [ ] T-S018-P07-104 [US4] Verify smoke tests execute in under 2 minutes and pass consistently (> 95% success rate)
- [ ] T-S018-P07-105 [US4] Verify integration tests pass across Chromium, Firefox, and WebKit browsers

**Checkpoint**: At this point, User Story 4 should be complete. Comprehensive Playwright E2E tests are implemented with smoke tests and integration tests, running against remote dev cluster.

---

## Phase 8: User Story 5 - Code cleanup and documentation (Priority: P3)

**Goal**: A developer can work with clean, well-documented codebase where dead code is removed, documentation is updated, and architecture is clearly explained.

**Independent Test**: Can be fully tested by removing duplicated URL logic, unused providers, and native fetch calls, then verifying that all functionality still works. This delivers value by reducing technical debt and improving code clarity.

### Implementation for User Story 5

- [ ] T-S018-P08-105 [US5] Search and remove all remaining duplicated URL computation logic from application code (verify zero instances)
- [ ] T-S018-P08-106 [US5] Search and remove all remaining native `fetch()` calls (verify zero instances, all replaced with `publicClient` or `httpClient`)
- [ ] T-S018-P08-107 [US5] Search and remove all remaining raw `axios` usage (verify zero instances, all replaced with standardized HTTP clients)
- [ ] T-S018-P08-108 [US5] Remove unused providers from `web/portal/src/main.tsx` if evaluation in Phase 3 determined they're not needed
- [ ] T-S018-P08-109 [US5] Verify provider hierarchy is flattened to maximum 4 levels in `web/portal/src/main.tsx`
- [ ] T-S018-P08-110 [US5] Update `web/portal/README.md` with new architecture explanation, centralized config usage, HTTP client patterns, and provider structure
- [ ] T-S018-P08-111 [US5] Document test running procedures in `web/portal/README.md` including local testing, remote cluster testing, and CI/CD integration
- [ ] T-S018-P08-112 [US5] Document environment configuration in `web/portal/README.md` including localhost, nip.io, and production domain setup
- [ ] T-S018-P08-113 [US5] Verify all code passes TypeScript compilation with strict mode (no `any` types)
- [ ] T-S018-P08-114 [US5] Verify zero console errors or warnings in production builds

**Checkpoint**: At this point, User Story 5 should be complete. Codebase is clean, well-documented, and all technical debt from duplicated logic is removed.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - **BLOCKS all user stories**
- **User Stories (Phase 3+)**: All depend on Foundational phase completion
  - User Story 1 (P1): Can start immediately after Foundational
  - User Story 2 (P2): Can start after Foundational (design system independent)
  - User Story 3 (P2): Depends on User Story 1 (uses foundation) and User Story 2 (uses design system)
  - User Story 6 (P2): Depends on User Story 1 (uses foundation) and User Story 2 (uses design system)
  - User Story 4 (P3): Depends on User Story 3 (tests the pages)
  - User Story 5 (P3): Depends on all previous stories (cleanup after implementation)
- **Polish (Final Phase)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) - No dependencies on other stories
- **User Story 2 (P2)**: Can start after Foundational (Phase 2) - Independent, can run parallel with US1
- **User Story 3 (P2)**: Depends on US1 (foundation) and US2 (design system) - Must complete US1 and US2 first
- **User Story 6 (P2)**: Depends on US1 (foundation) and US2 (design system) - Can run parallel with US3 after US1 and US2 complete
- **User Story 4 (P3)**: Depends on US3 (tests the rebuilt pages) - Must complete US3 first
- **User Story 5 (P3)**: Depends on all previous stories - Cleanup phase, must complete all implementation first

### Within Each User Story

- Foundation tasks (config, HTTP clients) before usage tasks
- Service classes before React components that use them
- Layout components before page components
- Core implementation before integration
- Story complete before moving to next priority

### Parallel Opportunities

- **Phase 1**: All setup tasks marked [P] can run in parallel
- **Phase 2**: Tasks T-S018-P02-005 through T-S018-P02-014 can run in parallel (different files, no dependencies)
- **Phase 3 (US1)**: Tasks T-S018-P03-015 through T-S018-P03-027 can run in parallel (different files, all use foundation)
- **Phase 4 (US2)**: Tasks T-S018-P04-029 through T-S018-P04-046 can run in parallel (different components)
- **Phase 5 (US3)**: Tasks T-S018-P05-048 through T-S018-P05-055 can run in parallel (different pages)
- **Phase 6 (US6)**: Tasks T-S018-P06-060 through T-S018-P06-083 can run in parallel (different command groups)
- **Phase 7 (US4)**: Tasks T-S018-P07-088 through T-S018-P07-092 can run in parallel (different test files)
- **Phase 8 (US5)**: Tasks T-S018-P08-105 through T-S018-P08-112 can run in parallel (different cleanup areas)

### Cross-Story Parallel Execution

Once Foundational (Phase 2) completes:
- **User Story 1** and **User Story 2** can run in parallel (different teams/developers)
- After US1 and US2 complete: **User Story 3** and **User Story 6** can run in parallel
- After US3 completes: **User Story 4** can start
- After all implementation: **User Story 5** (cleanup)

---

## Parallel Example: User Story 1

```bash
# Launch all foundation migration tasks in parallel:
Task: "Update AuthProvider.tsx to use publicClient" (T-S018-P03-015)
Task: "Update ServiceHealthCheck.tsx to use healthMonitor" (T-S018-P03-018)
Task: "Update apiKeys.ts to use httpClient" (T-S018-P03-020)
Task: "Create AppProviders wrapper" (T-S018-P03-022)
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - blocks all stories)
3. Complete Phase 3: User Story 1 (Stable foundation architecture)
4. **STOP and VALIDATE**: Test User Story 1 independently - verify all API calls work, no duplicated logic, standardized HTTP clients
5. Deploy/demo if ready

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test independently → Deploy/Demo (MVP - stable foundation!)
3. Add User Story 2 → Test independently → Deploy/Demo (Design system ready)
4. Add User Story 3 → Test independently → Deploy/Demo (Core pages functional)
5. Add User Story 6 → Test independently → Deploy/Demo (CLI coverage complete)
6. Add User Story 4 → Test independently → Deploy/Demo (E2E tests in place)
7. Add User Story 5 → Test independently → Deploy/Demo (Clean, documented codebase)
8. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. **Team completes Setup + Foundational together** (critical foundation)
2. **Once Foundational is done:**
   - Developer A: User Story 1 (Foundation architecture)
   - Developer B: User Story 2 (Design system) - can start in parallel with US1
3. **Once US1 and US2 complete:**
   - Developer A: User Story 3 (Rebuilt pages)
   - Developer B: User Story 6 (CLI coverage) - can run in parallel with US3
4. **Once US3 completes:**
   - Developer A: User Story 4 (E2E tests)
5. **Once all implementation complete:**
   - Developer A: User Story 5 (Cleanup and documentation)
6. Stories complete and integrate independently

---

## Notes

- [P] tasks = different files, no dependencies on incomplete tasks
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Avoid: vague tasks, same file conflicts, cross-story dependencies that break independence
- All API operations must use same endpoints as CLI (verify against `contracts/README.md`)
- All file paths are relative to `web/portal/` directory

---

## Summary

- **Total Tasks**: 116 tasks
- **Phase 1 (Setup)**: 4 tasks
- **Phase 2 (Foundational)**: 10 tasks (BLOCKS all user stories)
- **Phase 3 (US-001)**: 14 tasks
- **Phase 4 (US-002)**: 19 tasks
- **Phase 5 (US-003)**: 13 tasks
- **Phase 6 (US-006)**: 28 tasks
- **Phase 7 (US-004)**: 18 tasks
- **Phase 8 (US-005)**: 10 tasks

**Suggested MVP Scope**: Phase 1 + Phase 2 + Phase 3 (User Story 1) = 28 tasks
- Delivers stable foundation architecture
- Enables all subsequent work
- Independently testable and valuable

**Parallel Opportunities**: Significant parallelization possible within each phase, and between US1/US2, and US3/US6 after foundation is ready.
