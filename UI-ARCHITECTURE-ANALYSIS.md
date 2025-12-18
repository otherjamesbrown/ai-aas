# UI Architecture Analysis

This document provides a factual overview of the web portal architecture, current issues, and URL configurations.

## Current Issues

### 1. Rapid Re-rendering on Login Page

**Symptoms:**
- The ServiceHealthCheck component on the login page refreshes rapidly (multiple times per second)
- This occurs despite implementing guards: `useRef`, `useMemo`, `isCheckingRef`, `mountedRef`

**Attempted Fixes:**
- Added `useMemo` for `baseUrl` computation
- Added `useRef` pattern for `checkHealth` function
- Added `isCheckingRef` to prevent concurrent health checks
- Added `mountedRef` to prevent state updates after unmount
- Empty dependency array on the main `useEffect`

**Current State:** Issue persists after all fixes.

### 2. API URL Resolution

**Symptoms:**
- When accessing via `portal.dev.otherjamesbrown.com`, API calls must target `api.dev.otherjamesbrown.com`
- Build-time environment variable `VITE_API_BASE_URL` is set to `https://api.dev.otherjamesbrown.com/api`
- The `.local` domain does not resolve from external browsers

**Current Workaround:** Dynamic URL detection added to multiple files to detect nip.io hostnames.

---

## URL Configuration

### Environment Variables

| Variable | Purpose | Default Value |
|----------|---------|---------------|
| `VITE_API_BASE_URL` | Base URL for API calls | `http://localhost:8080/api` |
| `VITE_OAUTH_ISSUER_URL` | OAuth2/OIDC issuer URL | - |
| `VITE_OAUTH_CLIENT_ID` | OAuth2 client ID | - |
| `VITE_OAUTH_REDIRECT_URI` | OAuth2 callback URL | - |
| `VITE_FEATURE_FLAGS_API_URL` | Feature flags endpoint | - |

### URL Computation Locations

The API base URL is computed in **four separate locations**:

#### 1. HTTP Client (`/src/lib/http/client.ts`)

```typescript
const getApiBaseUrl = () => {
  const hostname = window.location.hostname;
  const nipioMatch = hostname.match(/^portal\.(.+\.nip\.io)$/);
  if (nipioMatch) {
    return `https://api.${nipioMatch[1]}/api`;  // Includes /api suffix
  }
  return import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080/api';
};
```

- Computed **once** at module load time
- Returns URL **with** `/api` suffix for nip.io
- Used by most API clients via `httpClient`

#### 2. AuthProvider (`/src/providers/AuthProvider.tsx`)

```typescript
const getBaseUrl = () => {
  const hostname = window.location.hostname;
  const nipioMatch = hostname.match(/^portal\.(.+\.nip\.io)$/);
  if (nipioMatch) {
    return `https://api.${nipioMatch[1]}`;  // No /api suffix
  }
  const apiBaseUrl = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080/api';
  return apiBaseUrl.replace(/\/api\/?$/, '');  // Strips /api suffix
};
```

- Computed **each time** `loginWithPassword()` is called
- Returns URL **without** `/api` suffix
- Appends `/v1/auth/login` for login endpoint

#### 3. API Keys Client (`/src/features/admin/api/apiKeys.ts`)

```typescript
const getBaseUrl = () => {
  const hostname = window.location.hostname;
  const nipioMatch = hostname.match(/^portal\.(.+\.nip\.io)$/);
  if (nipioMatch) {
    return `https://api.${nipioMatch[1]}`;  // No /api suffix
  }
  const apiBaseUrl = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080/api';
  return apiBaseUrl.replace(/\/api\/?$/, '');
};
```

- Computed **once** at module load time
- Returns URL **without** `/api` suffix
- Uses raw `axios` instead of `httpClient`

#### 4. ServiceHealthCheck (`/src/components/ServiceHealthCheck.tsx`)

```typescript
const baseUrl = useMemo(() => {
  const hostname = window.location.hostname;
  const nipioMatch = hostname.match(/^portal\.(.+\.nip\.io)$/);
  if (nipioMatch) {
    return `https://api.${nipioMatch[1]}`;  // No /api suffix
  }
  const apiBaseUrl = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080/api';
  return apiBaseUrl.replace(/\/api\/?$/, '');
}, []);
```

- Computed **once** per component mount (memoized)
- Returns URL **without** `/api` suffix
- Uses raw `fetch()` for health checks

### Access Patterns

| Access Method | Portal URL | Expected API URL |
|--------------|------------|------------------|
| nip.io | `https://portal.dev.otherjamesbrown.com` | `https://api.dev.otherjamesbrown.com` |
| Local domain | `https://portal.dev.otherjamesbrown.com` | `https://api.dev.otherjamesbrown.com` |
| Localhost | `http://localhost:5173` | `http://localhost:8080` |

---

## UI Architecture

### Application Bootstrap

**Entry Point:** `/src/main.tsx`

**Provider Hierarchy (outermost to innermost):**

```
React.StrictMode
└── ErrorBoundary
    └── TelemetryProvider
        └── AuthProvider
            └── FeatureFlagProviderWrapper
                └── QueryProvider
                    └── ToastProvider
                        └── RouterProvider
                            └── Layout
                                └── Route Components
```

### Key Providers

#### AuthProvider (`/src/providers/AuthProvider.tsx`)

**Responsibilities:**
- OAuth2/OIDC authentication flow
- Password-based authentication
- Token storage (sessionStorage)
- Token refresh scheduling
- User state management

**State:**
- `user: User | null`
- `isLoading: boolean`
- `refreshTimer: NodeJS.Timeout | null`

**Storage:**
- `sessionStorage.auth_token` - Access token
- `sessionStorage.refresh_token` - Refresh token
- `sessionStorage.user` - Serialized user object

#### FeatureFlagProvider (`/src/providers/FeatureFlagProvider.tsx`)

**Responsibilities:**
- Fetch feature flags from API
- Periodic refresh (5 minute interval)
- Provide `isEnabled(flagName)` function

**Dependencies:**
- Requires `useAuth()` from AuthProvider
- Fetches flags with `user_id` and `organization_id` params

#### QueryProvider (`/src/lib/query/index.tsx`)

**Configuration:**
- Retry: 2 attempts for queries, 1 for mutations
- Stale time: 5 minutes
- **No automatic refetch on window focus**

### HTTP Client (`/src/lib/http/client.ts`)

**Axios Instance Configuration:**
- Base URL: Computed at module load
- Timeout: 30 seconds
- Content-Type: `application/json`

**Request Interceptor Adds:**
- `Authorization: Bearer {token}` from sessionStorage
- `X-CSRF-Token` from meta tag (if present)
- `X-Correlation-ID` random UUID

**Response Interceptor:**
- On 401: Clears token, redirects to `/auth/login`

### Routing (`/src/app/AppRouter.tsx`)

**Router:** TanStack React Router v1.45.0

**Routes:**
| Path | Component | Auth Required |
|------|-----------|---------------|
| `/` | HomePage | No |
| `/auth/login` | LoginPage | No |
| `/admin/*` | Admin pages | Yes |
| `/usage` | UsageDashboardPage | Yes |
| `/support/console` | SupportConsolePage | Yes (support role) |

**Lazy Loading:** All page components use `React.lazy()` with Suspense fallback.

---

## Login Page Structure

**File:** `/src/app/pages/LoginPage.tsx`

### Component Hierarchy

```
LoginPage
├── State
│   ├── loginMethod: 'oauth' | 'password'
│   ├── email, password, orgId: string
│   └── isLoggingIn: boolean
├── Auth Hook (useAuth)
│   ├── login() - OAuth flow
│   └── loginWithPassword() - Password flow
├── Form UI
│   ├── Method selector tabs
│   ├── Email/Password inputs (password method)
│   └── OAuth button (oauth method)
└── ServiceHealthCheck (health status display)
```

### Authentication Flows

#### Password Login Flow

1. User enters email/password in LoginPage
2. Calls `AuthProvider.loginWithPassword()`
3. Computes API base URL (nip.io detection)
4. POST to `${baseUrl}/v1/auth/login` using **native fetch()**
5. On success: Store tokens in sessionStorage
6. GET `${baseUrl}/v1/auth/userinfo` using **native fetch()**
7. Store user in sessionStorage and React state
8. Schedule token refresh
9. Redirect to home or original destination

#### OAuth Login Flow

1. User clicks OAuth button
2. Calls `AuthProvider.login()`
3. Generates CSRF state token
4. Redirects to `${oauthIssuerUrl}/v1/auth/oidc/{provider}/login`
5. User authenticates with OAuth provider
6. Callback to `/auth/callback` with code + state
7. Exchange code for tokens
8. Continue from step 5 of password flow

---

## API Clients

### Using httpClient (Axios)

| Client | File | Endpoints |
|--------|------|-----------|
| organizationApi | `/src/features/admin/api/organization.ts` | `/organizations/me` |
| membersApi | `/src/features/admin/api/members.ts` | `/organizations/me/members/*` |
| budgetsApi | `/src/features/admin/api/budgets.ts` | `/organizations/me/budget/*` |
| auditApi | `/src/features/admin/api/audit.ts` | `/organizations/me/audit/*` |
| usageApi | `/src/features/usage/api.ts` | `/organizations/me/usage/*` |
| impersonationApi | `/src/features/support/api/impersonation.ts` | `/support/impersonations/*` |

### Using Raw Axios (Not httpClient)

| Client | File | Reason |
|--------|------|--------|
| apiKeysApi | `/src/features/admin/api/apiKeys.ts` | Manual token injection |

### Using Native fetch()

| Location | File | Reason |
|----------|------|--------|
| loginWithPassword | `/src/providers/AuthProvider.tsx` | Custom error handling |
| fetchUserInfo | `/src/providers/AuthProvider.tsx` | Part of login flow |
| ServiceHealthCheck | `/src/components/ServiceHealthCheck.tsx` | Health check endpoints |

---

## ServiceHealthCheck Component

**File:** `/src/components/ServiceHealthCheck.tsx`

### Purpose

Displays health status of backend services on the login page with traffic light indicators.

### Health Checks Performed

1. **API Gateway** - GET `/v1/status/healthz`
2. **Component Status** - GET `/v1/status/readyz` (parses components)
3. **CORS Preflight** - OPTIONS `/v1/status/healthz`
4. **Header Test** - GET `/v1/status/healthz` with `X-Correlation-ID` header

### Current Implementation

```typescript
// Guards
const isCheckingRef = useRef(false);
const mountedRef = useRef(true);

// Memoized base URL
const baseUrl = useMemo(() => { /* nip.io detection */ }, []);

// Health check function
const checkHealth = useCallback(async () => {
  if (isCheckingRef.current || !mountedRef.current) return;
  isCheckingRef.current = true;
  // ... perform checks ...
  if (mountedRef.current) {
    setServices(newServices);
    setLastChecked(new Date());
  }
  isCheckingRef.current = false;
}, [baseUrl]);

// Ref to latest checkHealth
const checkHealthRef = useRef(checkHealth);
checkHealthRef.current = checkHealth;

// Effect runs once on mount
useEffect(() => {
  mountedRef.current = true;
  checkHealthRef.current();
  const interval = setInterval(() => checkHealthRef.current(), 30000);
  return () => {
    mountedRef.current = false;
    clearInterval(interval);
  };
}, []);
```

### State

- `services: ServiceHealth[]` - Array of service statuses
- `expanded: boolean` - Panel expansion state
- `lastChecked: Date | null` - Last check timestamp

---

## Potential Architecture Concerns

### 1. Duplicated URL Logic

The nip.io URL detection pattern is duplicated in 4 files. Changes require updates in all locations.

### 2. Mixed HTTP Clients

Three different HTTP mechanisms are used:
- `httpClient` (Axios with interceptors)
- Raw `axios` (no interceptors)
- Native `fetch()` (no interceptors)

This means:
- Inconsistent header injection (CSRF, Correlation ID)
- Inconsistent error handling (401 redirect)
- Inconsistent timeout handling

### 3. Build-time vs Runtime URL Configuration

`VITE_*` environment variables are injected at build time, not runtime. This means:
- The Docker image contains hardcoded URLs
- Different environments require different builds or runtime detection

### 4. Provider Dependency Chain

FeatureFlagProviderWrapper depends on AuthProvider. If auth state changes, feature flags may refetch, potentially causing cascading updates.

### 5. Token Refresh Scheduling

Token refresh uses `setTimeout` with a recursive pattern. The callback function depends on `refreshTimer` state, which could cause closure issues.

### 6. React StrictMode

The application runs in `React.StrictMode` which intentionally double-invokes certain functions in development to detect side effects. This could contribute to unexpected behavior during debugging.

---

## File References

| Purpose | Path |
|---------|------|
| App entry | `/web/portal/src/main.tsx` |
| Router | `/web/portal/src/app/AppRouter.tsx` |
| Layout | `/web/portal/src/app/Layout.tsx` |
| Login page | `/web/portal/src/app/pages/LoginPage.tsx` |
| Auth provider | `/web/portal/src/providers/AuthProvider.tsx` |
| Feature flags | `/web/portal/src/providers/FeatureFlagProvider.tsx` |
| HTTP client | `/web/portal/src/lib/http/client.ts` |
| Health check | `/web/portal/src/components/ServiceHealthCheck.tsx` |
| API keys client | `/web/portal/src/features/admin/api/apiKeys.ts` |
| Env types | `/web/portal/src/vite-env.d.ts` |
