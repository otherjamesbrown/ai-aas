# 018-ui-update: Task List

This document outlines all tasks required to rebuild the admin portal with a stable architecture and Linode-inspired UI design.

## Overview

**Goal**: Rebuild the web portal with:
- Stable, simple architecture (no re-rendering issues)
- Centralized configuration and HTTP client
- Linode Cloud Manager-inspired design with system-aware theming (light/dark)
- Comprehensive Playwright test coverage

**Branch**: `feature/018-ui-update`

**Pages in Scope**:
- Login Page
- Home/Dashboard
- Admin Pages (API Keys, Members, Organization)
- Usage Dashboard

---

## Phase 1: Foundation (Architecture)

### 1.1 Create Centralized Configuration
- [ ] Create `/src/config/api.ts` with single URL computation
- [ ] Export `apiConfig` object with `baseUrl`, `apiUrl`, `isNipIo`, `environment`
- [ ] Add runtime config loading from `/config.json` (optional, for Kubernetes)
- [ ] Remove URL computation from all other files

**Files to create:**
- `web/portal/src/config/api.ts`
- `web/portal/src/config/index.ts`

### 1.2 Standardize HTTP Client
- [ ] Update `httpClient` to use centralized config
- [ ] Create `publicClient` for unauthenticated requests
- [ ] Add shared interceptors (correlation ID, error handling)
- [ ] Export both clients from single module

**Files to modify:**
- `web/portal/src/lib/http/client.ts`

### 1.3 Create Token Manager
- [ ] Implement `TokenManager` class for refresh logic
- [ ] Move token refresh from AuthProvider to TokenManager
- [ ] Handle refresh scheduling without closure issues

**Files to create:**
- `web/portal/src/services/tokenManager.ts`

### 1.4 Simplify Provider Hierarchy
- [ ] Create `AppProviders` wrapper component
- [ ] Evaluate necessity of TelemetryProvider
- [ ] Flatten provider tree to 4 levels max

**Files to create/modify:**
- `web/portal/src/providers/AppProviders.tsx`
- `web/portal/src/main.tsx`

---

## Phase 2: Design System

### 2.1 Configure System-Aware Theming
- [ ] Update Tailwind config for system-aware dark/light mode (`prefers-color-scheme`)
- [ ] Define color palette matching Linode style for both modes:

  **Dark Mode (Linode reference):**
  - Background: `#1a1a1a` (sidebar), `#2a2a2a` (content)
  - Text: `#ffffff` (primary), `#9ca3af` (secondary)
  - Accent: `#00b050` (success/teal), `#3b82f6` (primary blue)

  **Light Mode:**
  - Background: `#f5f6f7` (sidebar), `#ffffff` (content)
  - Text: `#1a1a1a` (primary), `#6b7280` (secondary)
  - Accent: Same as dark (consistent brand colors)

  **Both modes:**
  - Status: Green (healthy), Grey (offline), Red (error)

- [ ] Configure shadcn/ui components for both modes
- [ ] Add theme toggle (optional - default to system preference)
- [ ] Store user preference in localStorage if manually overridden

**Files to modify:**
- `web/portal/tailwind.config.js`
- `web/portal/src/index.css`

**Files to create:**
- `web/portal/src/hooks/useTheme.ts`

### 2.2 Create Base Layout Components
- [ ] Sidebar component (collapsible sections, icons)
- [ ] Header component (search, user menu, notifications)
- [ ] Content area with proper spacing
- [ ] Footer component (version, API reference, feedback)

**Files to create:**
- `web/portal/src/components/layout/Sidebar.tsx`
- `web/portal/src/components/layout/Header.tsx`
- `web/portal/src/components/layout/ContentArea.tsx`
- `web/portal/src/components/layout/Footer.tsx`
- `web/portal/src/components/layout/AdminLayout.tsx`

### 2.3 Create Reusable UI Components
- [ ] DataTable component (sortable columns, pagination)
- [ ] StatusBadge component (traffic light indicators)
- [ ] TabNav component (page-level tab navigation)
- [ ] Card component (for detail views)
- [ ] FilterPanel component (dropdowns, date pickers)
- [ ] LoadingSpinner component

**Files to create:**
- `web/portal/src/components/ui/DataTable.tsx`
- `web/portal/src/components/ui/StatusBadge.tsx`
- `web/portal/src/components/ui/TabNav.tsx`
- `web/portal/src/components/ui/Card.tsx`
- `web/portal/src/components/ui/FilterPanel.tsx`
- `web/portal/src/components/ui/LoadingSpinner.tsx`

---

## Phase 3: Page Implementation

### 3.1 Login Page
- [ ] Rebuild with stable ServiceHealthCheck (non-React singleton pattern)
- [ ] Use `publicClient` for auth requests
- [ ] Clean form state management
- [ ] Proper loading states
- [ ] Error handling with toast notifications

**Files to modify/create:**
- `web/portal/src/app/pages/LoginPage.tsx`
- `web/portal/src/components/ServiceHealthCheck.tsx`
- `web/portal/src/services/healthMonitor.ts`

### 3.2 Home/Dashboard Page
- [ ] Quick stats cards (API calls, budget usage, etc.)
- [ ] Recent activity list
- [ ] System status overview
- [ ] Navigation cards to main sections

**Files to modify:**
- `web/portal/src/app/pages/HomePage.tsx`

### 3.3 Admin - API Keys Page
- [ ] DataTable with API keys list
- [ ] Create/Edit modal
- [ ] View scopes modal
- [ ] Revoke confirmation
- [ ] Use `httpClient` instead of raw axios

**Files to modify:**
- `web/portal/src/features/admin/pages/ApiKeysPage.tsx`
- `web/portal/src/features/admin/api/apiKeys.ts`

### 3.4 Admin - Members Page
- [ ] DataTable with members list
- [ ] Invite member modal
- [ ] Role management
- [ ] Remove member confirmation

**Files to modify:**
- `web/portal/src/features/admin/pages/MembersPage.tsx`

### 3.5 Admin - Organization Page
- [ ] Organization details card
- [ ] Settings form
- [ ] Budget configuration

**Files to modify:**
- `web/portal/src/features/admin/pages/OrganizationPage.tsx`

### 3.6 Usage Dashboard
- [ ] Filter panel (date range, model, etc.)
- [ ] Usage charts (similar to Linode metrics)
- [ ] Usage breakdown table
- [ ] Export functionality

**Files to modify:**
- `web/portal/src/features/usage/pages/UsageDashboardPage.tsx`

---

## Phase 4: Testing Infrastructure

### 4.1 Playwright Smoke Tests
- [ ] Create smoke test suite (`smoke.spec.ts`)
- [ ] Configure for CI/CD pipeline
- [ ] Tests run against remote dev cluster
- [ ] Fast execution (< 2 minutes)

**Smoke test coverage:**
- [ ] Login page loads
- [ ] Can authenticate with test user
- [ ] Home page loads after login
- [ ] API Keys page loads
- [ ] Logout works

**Files to create:**
- `web/portal/tests/e2e/smoke.spec.ts`

### 4.2 Full Integration Tests
- [ ] Update existing login.spec.ts
- [ ] Create API keys integration tests
- [ ] Create members integration tests
- [ ] Create usage dashboard tests
- [ ] Use API key auth for faster test setup

**Files to create/modify:**
- `web/portal/tests/e2e/login.spec.ts`
- `web/portal/tests/e2e/api-keys.spec.ts`
- `web/portal/tests/e2e/members.spec.ts`
- `web/portal/tests/e2e/usage.spec.ts`

### 4.3 Test Utilities
- [ ] Create auth helper for API key authentication
- [ ] Create page object models for common pages
- [ ] Create test data fixtures

**Files to create:**
- `web/portal/tests/e2e/helpers/auth.ts`
- `web/portal/tests/e2e/pages/LoginPage.ts`
- `web/portal/tests/e2e/pages/HomePage.ts`
- `web/portal/tests/e2e/pages/ApiKeysPage.ts`
- `web/portal/tests/e2e/fixtures/index.ts`

### 4.4 CI/CD Integration
- [ ] Update playwright.config.ts for remote testing
- [ ] Add smoke test stage to CI pipeline
- [ ] Configure test reporting
- [ ] Add test artifacts (screenshots, videos) on failure

**Files to modify:**
- `web/portal/playwright.config.ts`
- `.github/workflows/ci.yml` (or equivalent)

---

## Phase 5: Cleanup & Documentation

### 5.1 Remove Dead Code
- [ ] Remove duplicated URL logic from all files
- [ ] Remove unused providers
- [ ] Remove native fetch() calls (replace with publicClient)
- [ ] Remove raw axios usage (replace with httpClient)

### 5.2 Update Documentation
- [ ] Update README with new architecture
- [ ] Document test running procedures
- [ ] Document environment configuration

---

## Definition of Done

Each task is complete when:
1. Code is implemented and passes TypeScript compilation
2. Relevant tests pass locally
3. Smoke tests pass against remote dev cluster
4. No console errors or warnings
5. Code is committed and pushed to feature branch
6. ArgoCD successfully deploys to development

---

## Dependencies

| Task | Depends On |
|------|------------|
| 1.2 HTTP Client | 1.1 Centralized Config |
| 1.3 Token Manager | 1.2 HTTP Client |
| 1.4 Providers | 1.3 Token Manager |
| 2.2 Layout Components | 2.1 Dark Theme |
| 2.3 UI Components | 2.1 Dark Theme |
| 3.x Pages | 2.x Design System |
| 4.1 Smoke Tests | 3.1 Login Page |
| 4.2 Integration Tests | 3.x Pages |

---

## Estimated Effort

| Phase | Tasks | Complexity |
|-------|-------|------------|
| Phase 1: Foundation | 4 | Medium |
| Phase 2: Design System | 3 | Medium |
| Phase 3: Pages | 6 | High |
| Phase 4: Testing | 4 | Medium |
| Phase 5: Cleanup | 2 | Low |

**Total**: 19 task groups
