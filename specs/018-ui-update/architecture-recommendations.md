# UI Architecture Recommendations

This document provides recommendations for stabilizing the web portal architecture based on the issues identified in UI-ARCHITECTURE-ANALYSIS.md.

## Executive Summary

The current architecture has several reliability issues stemming from:
1. Duplicated URL computation logic across 4 files
2. Mixed HTTP client approaches (Axios, raw axios, native fetch)
3. Build-time configuration that doesn't adapt to runtime environments
4. Overly complex state management for a simple admin portal

**Recommendation**: Simplify and centralize. An admin portal should prioritize reliability over flexibility.

---

## Priority 1: Centralize URL Configuration

### Problem

The nip.io URL detection pattern is duplicated in 4 separate files:
- `/src/lib/http/client.ts`
- `/src/providers/AuthProvider.tsx`
- `/src/features/admin/api/apiKeys.ts`
- `/src/components/ServiceHealthCheck.tsx`

This creates maintenance burden and inconsistency risk.

### Solution: Single URL Configuration Module

Create a dedicated configuration module that computes URLs once at application startup:

```typescript
// /src/config/api.ts

interface ApiConfig {
  baseUrl: string;        // Without /api suffix (e.g., https://api.example.com)
  apiUrl: string;         // With /api suffix (e.g., https://api.example.com/api)
  isNipIo: boolean;       // Whether running via nip.io
  environment: 'local' | 'development' | 'production';
}

function computeApiConfig(): ApiConfig {
  const hostname = window.location.hostname;
  const nipioMatch = hostname.match(/^portal\.(.+\.nip\.io)$/);

  if (nipioMatch) {
    const baseUrl = `https://api.${nipioMatch[1]}`;
    return {
      baseUrl,
      apiUrl: `${baseUrl}/api`,
      isNipIo: true,
      environment: 'development',
    };
  }

  const envBaseUrl = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080/api';
  const baseUrl = envBaseUrl.replace(/\/api\/?$/, '');

  return {
    baseUrl,
    apiUrl: envBaseUrl,
    isNipIo: false,
    environment: hostname === 'localhost' ? 'local' : 'production',
  };
}

// Compute once at module load - this is intentional
export const apiConfig = Object.freeze(computeApiConfig());
```

**Benefits:**
- Single source of truth
- Computed once at startup
- Immutable after initialization
- Clear distinction between `baseUrl` and `apiUrl`

---

## Priority 2: Standardize HTTP Client Usage

### Problem

Three HTTP mechanisms are in use:
1. `httpClient` (Axios with interceptors) - most API calls
2. Raw `axios` (no interceptors) - API keys client
3. Native `fetch()` - Auth and health checks

This causes:
- Inconsistent header injection (CSRF, Correlation ID)
- Inconsistent error handling (401 redirect)
- Inconsistent timeout handling

### Solution: Single HTTP Client with Explicit Modes

Extend the existing httpClient to handle all use cases:

```typescript
// /src/lib/http/client.ts

import axios, { AxiosInstance, AxiosRequestConfig } from 'axios';
import { apiConfig } from '@/config/api';

// Standard authenticated client
export const httpClient: AxiosInstance = axios.create({
  baseURL: apiConfig.apiUrl,
  timeout: 30000,
  headers: { 'Content-Type': 'application/json' },
});

// Unauthenticated client for login/health checks
export const publicClient: AxiosInstance = axios.create({
  baseURL: apiConfig.baseUrl,
  timeout: 30000,
  headers: { 'Content-Type': 'application/json' },
});

// Shared request interceptor for correlation IDs
const addCorrelationId = (config: AxiosRequestConfig) => {
  config.headers = config.headers || {};
  config.headers['X-Correlation-ID'] = crypto.randomUUID();
  return config;
};

// Add to both clients
httpClient.interceptors.request.use(addCorrelationId);
publicClient.interceptors.request.use(addCorrelationId);

// Auth interceptor only for httpClient
httpClient.interceptors.request.use((config) => {
  const token = sessionStorage.getItem('auth_token');
  if (token) {
    config.headers = config.headers || {};
    config.headers['Authorization'] = `Bearer ${token}`;
  }
  return config;
});

// 401 handling only for httpClient
httpClient.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      sessionStorage.removeItem('auth_token');
      sessionStorage.removeItem('refresh_token');
      sessionStorage.removeItem('user');
      window.location.href = '/auth/login';
    }
    return Promise.reject(error);
  }
);
```

**Migration Path:**
1. Update `AuthProvider.tsx` to use `publicClient` instead of native `fetch()`
2. Update `ServiceHealthCheck.tsx` to use `publicClient`
3. Update `apiKeys.ts` to use `httpClient` (it needs auth)
4. Remove all native `fetch()` calls from application code

---

## Priority 3: Fix ServiceHealthCheck Re-rendering

### Problem

The component re-renders rapidly despite guards (`useRef`, `useMemo`, `isCheckingRef`).

### Root Cause Analysis

Likely causes:
1. **React StrictMode** double-invokes effects in development
2. **Parent component re-renders** causing child re-renders
3. **State updates in parent** (LoginPage) triggering cascading updates

### Solution: Extract to Standalone Component with Isolation

```typescript
// /src/components/ServiceHealthCheck.tsx

import { useState, useEffect, useRef } from 'react';
import { publicClient } from '@/lib/http/client';

interface ServiceHealth {
  name: string;
  status: 'healthy' | 'unhealthy' | 'checking';
  latency?: number;
}

const HEALTH_CHECK_INTERVAL = 30000; // 30 seconds

export function ServiceHealthCheck() {
  const [services, setServices] = useState<ServiceHealth[]>([]);
  const [lastChecked, setLastChecked] = useState<Date | null>(null);
  const [expanded, setExpanded] = useState(false);

  // Single ref to track if we've started
  const initialized = useRef(false);

  useEffect(() => {
    // Prevent StrictMode double-execution
    if (initialized.current) return;
    initialized.current = true;

    let mounted = true;
    let intervalId: NodeJS.Timeout | null = null;

    const checkHealth = async () => {
      if (!mounted) return;

      const checks: ServiceHealth[] = [
        { name: 'API Gateway', status: 'checking' },
        { name: 'CORS', status: 'checking' },
      ];

      // Update UI to show checking state
      setServices([...checks]);

      // Perform health check
      try {
        const start = Date.now();
        await publicClient.get('/v1/status/healthz');
        const latency = Date.now() - start;

        if (mounted) {
          checks[0] = { name: 'API Gateway', status: 'healthy', latency };
        }
      } catch {
        if (mounted) {
          checks[0] = { name: 'API Gateway', status: 'unhealthy' };
        }
      }

      // CORS check
      try {
        await publicClient.options('/v1/status/healthz');
        if (mounted) {
          checks[1] = { name: 'CORS', status: 'healthy' };
        }
      } catch {
        if (mounted) {
          checks[1] = { name: 'CORS', status: 'unhealthy' };
        }
      }

      if (mounted) {
        setServices([...checks]);
        setLastChecked(new Date());
      }
    };

    // Initial check
    checkHealth();

    // Set up interval
    intervalId = setInterval(checkHealth, HEALTH_CHECK_INTERVAL);

    return () => {
      mounted = false;
      if (intervalId) clearInterval(intervalId);
    };
  }, []); // Empty deps - runs once

  // Render logic...
}
```

**Key Changes:**
1. Use `initialized.current` to prevent StrictMode double-execution
2. Use `publicClient` instead of native `fetch()`
3. Simpler state management
4. Clear cleanup logic

### Alternative: Move Health Check Outside React

For maximum stability, consider moving health checks to a non-React singleton:

```typescript
// /src/services/healthMonitor.ts

class HealthMonitor {
  private status: Map<string, 'healthy' | 'unhealthy'> = new Map();
  private listeners: Set<() => void> = new Set();
  private intervalId: NodeJS.Timeout | null = null;

  start() {
    if (this.intervalId) return;
    this.check();
    this.intervalId = setInterval(() => this.check(), 30000);
  }

  stop() {
    if (this.intervalId) {
      clearInterval(this.intervalId);
      this.intervalId = null;
    }
  }

  subscribe(listener: () => void) {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  getStatus() {
    return Object.fromEntries(this.status);
  }

  private async check() {
    // Perform checks and update this.status
    // Notify listeners
    this.listeners.forEach(fn => fn());
  }
}

export const healthMonitor = new HealthMonitor();
```

Then use a simple React hook to subscribe:

```typescript
function useHealthStatus() {
  const [status, setStatus] = useState(healthMonitor.getStatus());

  useEffect(() => {
    return healthMonitor.subscribe(() => {
      setStatus(healthMonitor.getStatus());
    });
  }, []);

  return status;
}
```

---

## Priority 4: Simplify Provider Hierarchy

### Current Structure (6 levels deep)

```
React.StrictMode
└── ErrorBoundary
    └── TelemetryProvider
        └── AuthProvider
            └── FeatureFlagProviderWrapper
                └── QueryProvider
                    └── ToastProvider
                        └── RouterProvider
```

### Problem

- Deep nesting increases render complexity
- FeatureFlagProvider depends on AuthProvider, creating coupling
- Changes in auth state can cascade through the tree

### Recommendation: Flatten Where Possible

For an admin portal, consider whether all these providers are necessary:

```
React.StrictMode
└── ErrorBoundary
    └── AppProviders (combines QueryProvider + ToastProvider)
        └── AuthProvider
            └── RouterProvider
```

**Specific Changes:**

1. **Remove TelemetryProvider** if not actively using telemetry
2. **Remove FeatureFlagProvider** if feature flags are not critical for admin portal
   - Or fetch flags lazily per-page instead of globally
3. **Combine QueryProvider + ToastProvider** into single `AppProviders` component

```typescript
// /src/providers/AppProviders.tsx

export function AppProviders({ children }: { children: React.ReactNode }) {
  return (
    <QueryClientProvider client={queryClient}>
      <ToastProvider>
        {children}
      </ToastProvider>
    </QueryClientProvider>
  );
}
```

---

## Priority 5: Runtime Configuration

### Problem

`VITE_*` environment variables are baked in at build time. The same Docker image cannot be used across environments without the nip.io workaround.

### Solution: Runtime Configuration Endpoint

**Option A: Config endpoint from API**

```typescript
// Fetch config at startup before React renders
async function loadConfig(): Promise<AppConfig> {
  // Try runtime config first
  try {
    const response = await fetch('/config.json');
    if (response.ok) {
      return await response.json();
    }
  } catch {
    // Fall through to computed config
  }

  // Fall back to computed config
  return computeApiConfig();
}

// In main.tsx
loadConfig().then(config => {
  window.__APP_CONFIG__ = config;
  ReactDOM.createRoot(document.getElementById('root')!).render(<App />);
});
```

**Option B: Inject config at container startup**

In Kubernetes, mount a ConfigMap as `/usr/share/nginx/html/config.json`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: portal-config
data:
  config.json: |
    {
      "apiUrl": "https://api.dev.ai-aas.local/api",
      "baseUrl": "https://api.dev.ai-aas.local"
    }
```

---

## Priority 6: Token Refresh Improvements

### Problem

Token refresh uses `setTimeout` with recursive pattern and potential closure issues.

### Solution: Use a Token Manager Class

```typescript
// /src/services/tokenManager.ts

class TokenManager {
  private refreshTimeout: NodeJS.Timeout | null = null;
  private onLogout: (() => void) | null = null;

  setLogoutCallback(callback: () => void) {
    this.onLogout = callback;
  }

  scheduleRefresh(expiresIn: number) {
    this.cancelRefresh();

    // Refresh 60 seconds before expiry
    const refreshIn = Math.max(0, (expiresIn - 60) * 1000);

    this.refreshTimeout = setTimeout(() => {
      this.doRefresh();
    }, refreshIn);
  }

  cancelRefresh() {
    if (this.refreshTimeout) {
      clearTimeout(this.refreshTimeout);
      this.refreshTimeout = null;
    }
  }

  private async doRefresh() {
    const refreshToken = sessionStorage.getItem('refresh_token');
    if (!refreshToken) {
      this.onLogout?.();
      return;
    }

    try {
      const response = await publicClient.post('/v1/auth/refresh', {
        refresh_token: refreshToken,
      });

      const { access_token, refresh_token, expires_in } = response.data;

      sessionStorage.setItem('auth_token', access_token);
      sessionStorage.setItem('refresh_token', refresh_token);

      this.scheduleRefresh(expires_in);
    } catch {
      this.onLogout?.();
    }
  }
}

export const tokenManager = new TokenManager();
```

---

## Implementation Order

### Phase 1: Foundation (Day 1-2)
1. Create `/src/config/api.ts` with centralized URL logic
2. Update `httpClient` to use new config
3. Create `publicClient` for unauthenticated requests

### Phase 2: Migration (Day 2-3)
1. Update `AuthProvider.tsx` to use `publicClient`
2. Update `apiKeys.ts` to use `httpClient`
3. Update `ServiceHealthCheck.tsx` to use `publicClient`
4. Remove duplicated URL logic from all files

### Phase 3: Stability (Day 3-4)
1. Fix ServiceHealthCheck re-rendering with `initialized.current` pattern
2. Simplify provider hierarchy
3. Implement TokenManager class

### Phase 4: Polish (Day 4-5)
1. Add runtime configuration support
2. Remove unused providers
3. Test across all access patterns (localhost, nip.io, local domain)

---

## Summary of Changes

| File | Change |
|------|--------|
| `/src/config/api.ts` | **NEW** - Centralized URL configuration |
| `/src/lib/http/client.ts` | Add `publicClient`, use centralized config |
| `/src/providers/AuthProvider.tsx` | Use `publicClient`, remove URL logic |
| `/src/components/ServiceHealthCheck.tsx` | Use `publicClient`, fix re-rendering |
| `/src/features/admin/api/apiKeys.ts` | Use `httpClient`, remove URL logic |
| `/src/services/tokenManager.ts` | **NEW** - Token refresh management |
| `/src/providers/AppProviders.tsx` | **NEW** - Combined provider wrapper |

---

## Testing Checklist

After implementation, verify:

- [ ] Login works via `localhost:5173`
- [ ] Login works via `portal.172.232.58.222.nip.io`
- [ ] Login works via `portal.dev.ai-aas.local`
- [ ] ServiceHealthCheck does not flicker/re-render rapidly
- [ ] Health checks complete within 5 seconds
- [ ] Token refresh works after login
- [ ] 401 errors redirect to login
- [ ] All API calls include `X-Correlation-ID` header
- [ ] CSRF token is included when available

---

## Alternative: Consider Simpler Stack

If the current React architecture continues to cause issues, consider whether the complexity is warranted for an admin portal.

**Simpler alternatives:**

1. **React without TanStack Query** - Use simple `useEffect` + `fetch` for admin CRUD
2. **Preact** - Smaller, compatible with React ecosystem, fewer edge cases
3. **Server-rendered with HTMX** - For very simple admin UIs, eliminates SPA complexity entirely

An admin portal's primary requirement is reliability, not cutting-edge UX. Choose the simplest architecture that meets the needs.
