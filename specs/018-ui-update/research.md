# Research: UI Update & Admin Portal Rebuild

**Feature**: 018-ui-update  
**Date**: 2025-01-27  
**Phase**: 0 - Outline & Research

This document consolidates research findings and architectural decisions for the UI rebuild.

## Architecture Decisions

### Decision 1: Centralized API Configuration

**Decision**: Create single `/src/config/api.ts` module that computes API URLs once at startup.

**Rationale**: 
- Eliminates duplication across 4+ files
- Single source of truth for URL computation
- Immutable configuration prevents runtime changes
- Clear distinction between `baseUrl` and `apiUrl`
- Supports multiple environments (localhost, nip.io, production) without code changes

**Alternatives considered**:
- Keep distributed URL logic: Rejected - maintenance burden and inconsistency risk
- Environment variables only: Rejected - doesn't handle nip.io pattern automatically
- Runtime config endpoint: Considered but optional - fallback to computed config

**Implementation**: Module-level computation with `Object.freeze()` to ensure immutability.

---

### Decision 2: Standardized HTTP Clients

**Decision**: Use two Axios instances: `httpClient` (authenticated) and `publicClient` (unauthenticated), both with correlation ID interceptors.

**Rationale**:
- Consistent error handling (401 redirect, timeouts)
- Consistent header injection (correlation IDs, CSRF tokens)
- Eliminates mixed approaches (native fetch, raw axios)
- Clear separation between authenticated and unauthenticated requests
- Shared interceptors reduce code duplication

**Alternatives considered**:
- Single client with conditional auth: Rejected - less clear intent, harder to reason about
- Keep native fetch for public requests: Rejected - inconsistent behavior and error handling
- Separate clients per feature: Rejected - unnecessary complexity for admin portal

**Implementation**: Axios instances with shared request/response interceptors. Auth interceptor only on `httpClient`.

---

### Decision 3: Token Manager Service

**Decision**: Extract token refresh logic to `TokenManager` class outside React component tree.

**Rationale**:
- Avoids React closure issues with `setTimeout` callbacks
- Centralized token refresh scheduling and cancellation
- Easier to test and reason about
- Prevents memory leaks from stale closures
- Clear separation of concerns

**Alternatives considered**:
- Keep refresh logic in AuthProvider: Rejected - closure issues and complexity
- Use React hooks for refresh: Rejected - still has closure issues with timeouts
- External library: Considered but unnecessary - simple class is sufficient

**Implementation**: Singleton class with `scheduleRefresh()`, `cancelRefresh()`, and logout callback support.

---

### Decision 4: Health Check Singleton Pattern

**Decision**: Move health check logic to non-React singleton (`healthMonitor.ts`) with React hook subscription.

**Rationale**:
- Prevents React re-rendering issues (StrictMode double-execution)
- Stable health check execution independent of component lifecycle
- Clear separation between health monitoring and UI rendering
- Easier to test and debug
- Prevents rapid flickering on login page

**Alternatives considered**:
- React component with guards: Rejected - still flickers in StrictMode
- React component with `useRef` guards: Rejected - doesn't fully solve StrictMode issues
- External library: Considered but unnecessary - simple singleton pattern works

**Implementation**: Singleton class with `start()`, `stop()`, `subscribe()`, and `getStatus()` methods. React hook subscribes to updates.

---

### Decision 5: Provider Hierarchy Simplification

**Decision**: Flatten provider tree to maximum 4 levels by combining providers and removing unused ones.

**Rationale**:
- Reduces render complexity and cascade effects
- Improves performance by reducing provider nesting
- Easier to reason about and debug
- Matches simplicity needs of admin portal (not complex SPA)

**Alternatives considered**:
- Keep all providers separate: Rejected - unnecessary complexity for admin portal
- Remove all providers: Rejected - still need Query, Auth, Toast
- Use context composition: Considered but adds complexity without clear benefit

**Implementation**: Combine QueryProvider + ToastProvider into `AppProviders`. Evaluate TelemetryProvider and FeatureFlagProvider necessity.

---

### Decision 6: System-Aware Theming

**Decision**: Implement system-aware theming (light/dark) using `prefers-color-scheme` with manual override support.

**Rationale**:
- Better user experience (respects system preference)
- Modern standard approach
- Linode-inspired design requires both themes
- localStorage override provides user control
- Consistent with modern admin portals

**Alternatives considered**:
- Light mode only: Rejected - doesn't match Linode design inspiration
- Manual toggle only: Rejected - worse UX, requires user action
- Theme library: Considered but TailwindCSS handles this natively

**Implementation**: TailwindCSS dark mode with `prefers-color-scheme`, localStorage for manual override, React hook for theme management.

---

### Decision 7: Playwright E2E Testing Strategy

**Decision**: Comprehensive Playwright E2E tests with smoke tests (< 2 min) and full integration tests, run against remote dev cluster.

**Rationale**:
- Tests real user workflows end-to-end
- Catches integration issues unit tests miss
- Smoke tests provide fast feedback in CI
- Remote cluster testing matches production environment
- Page object models improve maintainability

**Alternatives considered**:
- Unit tests only: Rejected - doesn't catch integration issues
- Cypress instead of Playwright: Considered but Playwright has better multi-browser support
- Local testing only: Rejected - doesn't match production environment

**Implementation**: Playwright with Chromium/Firefox/WebKit support, page objects, auth helpers, test fixtures, CI/CD integration.

---

### Decision 8: CLI Command Coverage in UI

**Decision**: Provide UI pages for all ai-aas-cli command groups and commands with feature parity.

**Rationale**:
- Complete web-based management interface
- Users who prefer GUI over CLI should have full access
- Ensures portal is not a subset of functionality
- Matches modern platform expectations (web + CLI)
- Same backend APIs ensure consistency

**Alternatives considered**:
- Partial CLI coverage: Rejected - incomplete management interface
- Separate UI-only endpoints: Rejected - violates API-first principle
- CLI-only for some operations: Rejected - inconsistent user experience

**Implementation**: UI pages organized by CLI command groups (Model Management, Access Control, Platform Operations, Utilities), using same backend APIs as CLI.

---

## Technology Choices

### React 18 + TypeScript
- **Decision**: Continue using React 18 with TypeScript strict mode
- **Rationale**: Existing stack, proven stability, strong typing prevents errors
- **Alternatives**: Preact (smaller but ecosystem compatibility concerns), Vue (would require rewrite)

### TanStack Router + Query
- **Decision**: Continue using @tanstack/react-router and @tanstack/react-query
- **Rationale**: Existing stack, good TypeScript support, excellent caching and state management
- **Alternatives**: React Router (less type-safe), SWR (similar but already using TanStack Query)

### TailwindCSS + shadcn/ui
- **Decision**: Use TailwindCSS with shadcn/ui components
- **Rationale**: Constitution standard, excellent theming support, accessible components
- **Alternatives**: Material-UI (heavier, less customizable), Chakra UI (similar but shadcn/ui preferred)

### Playwright
- **Decision**: Use Playwright for E2E testing
- **Rationale**: Multi-browser support, excellent debugging tools, fast execution
- **Alternatives**: Cypress (single browser focus, slower), Puppeteer (lower-level API)

---

## Best Practices Research

### React Architecture Patterns
- **Finding**: Provider hierarchy should be minimal for admin portals
- **Source**: React documentation, admin portal best practices
- **Application**: Flatten to 4 levels max, combine where possible

### HTTP Client Patterns
- **Finding**: Single HTTP client library with interceptors is standard practice
- **Source**: Axios documentation, React best practices
- **Application**: Use Axios with shared interceptors for all requests

### Testing Pyramid
- **Finding**: E2E tests should be few but comprehensive, unit tests many but fast
- **Source**: Testing best practices, Playwright documentation
- **Application**: Focus on E2E tests for critical paths, unit tests for utilities

### Theme Implementation
- **Finding**: System-aware theming with manual override is modern standard
- **Source**: TailwindCSS documentation, design system best practices
- **Application**: Use `prefers-color-scheme` with localStorage override

---

## Resolved Clarifications

All technical clarifications from the spec have been resolved:

1. ✅ **Scope of pages**: Login, Dashboard, Admin pages, Usage Dashboard, plus all CLI command groups
2. ✅ **Design system**: Linode-inspired with system-aware theming
3. ✅ **Testing strategy**: Playwright E2E with smoke + integration tests
4. ✅ **CLI coverage**: Full feature parity with all command groups
5. ✅ **Out of scope**: Backend changes, new features, mobile optimizations

No "NEEDS CLARIFICATION" items remain in Technical Context.

---

## Next Steps

Proceed to Phase 1: Design & Contracts
- Generate data-model.md with entities
- Generate API contracts in contracts/
- Generate quickstart.md developer guide
- Update agent context

