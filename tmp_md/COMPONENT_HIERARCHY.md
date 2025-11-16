# Component Hierarchy & Request Flow

## Current Architecture (Local Dev)

### Request Flow: UI → Backend Services

```
┌─────────────────┐
│  Web Portal UI  │
│  (Port 5173)    │
│  localhost      │
└────────┬────────┘
         │ HTTP Request
         │ POST /v1/auth/login
         │ VITE_API_BASE_URL
         ▼
┌─────────────────────────┐
│   Current Config        │
│  VITE_API_BASE_URL =    │
│  http://localhost:8081/v1│
└────────┬────────────────┘
         │
         │ ⚠️ BYPASSING API ROUTER (Temporary)
         │
         ▼
┌──────────────────────┐
│  user-org-service    │
│  (Port 8081)         │
│  localhost           │
└────────┬─────────────┘
         │
         │ Database Query
         ▼
┌──────────────────────┐
│   PostgreSQL         │
│   (Port 5433)        │
│   localhost          │
└──────────────────────┘
```

## Intended Architecture (Production)

### Request Flow: UI → API Router → Backend Services

```
┌─────────────────┐
│  Web Portal UI  │
│  (Port 5173)    │
│  localhost      │
└────────┬────────┘
         │ HTTP Request
         │ POST /api/auth/login
         │ VITE_API_BASE_URL
         │ = http://localhost:8080/api
         ▼
┌──────────────────────┐
│   API Router Service │
│   (Port 8080)        │
│   localhost          │
│                      │
│  - Auth Middleware   │
│  - Rate Limiting     │
│  - Budget Enforcement│
│  - Request Routing   │
└────────┬─────────────┘
         │
         │ Forward to user-org-service
         │ POST /v1/auth/login
         │ (with auth context)
         ▼
┌──────────────────────┐
│  user-org-service    │
│  (Port 8081)         │
│  localhost           │
└────────┬─────────────┘
         │
         │ Database Query
         ▼
┌──────────────────────┐
│   PostgreSQL         │
│   (Port 5433)        │
│   localhost          │
└──────────────────────┘
```

## Component Details

### 1. Web Portal UI (`web/portal`)

**Entry Point**: `web/portal/src/lib/http/client.ts`

```typescript
const apiBaseUrl = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080/api';
export const httpClient = new HttpClient(apiBaseUrl);
```

**Configuration**:
- **Default**: `http://localhost:8080/api` (API Router)
- **Current Local Dev**: `http://localhost:8081/v1` (Direct to user-org-service)
- **Production**: `http://api-router-service:8080/api` (via Kubernetes service)

**HTTP Client Features**:
- Base URL configuration via `VITE_API_BASE_URL`
- Auto-injects `Authorization: Bearer <token>` header
- Auto-injects CSRF token from meta tag
- Auto-injects `X-Correlation-ID` header
- Handles 401 errors (token refresh/redirect to login)

**Request Flow from UI**:
1. User submits login form (`LoginPage.tsx`)
2. `AuthProvider.tsx` calls `loginWithPassword()`
3. Uses `httpClient.post('/auth/login', ...)` from `client.ts`
4. Request goes to `VITE_API_BASE_URL + '/auth/login'`

### 2. API Router Service (`services/api-router-service`)

**Purpose**: 
- Single entry point for all API requests
- Authentication/Authorization middleware
- Rate limiting and budget enforcement
- Request routing to backend services
- Audit logging

**Routes**:
- `/api/*` - Public API routes (inference, auth proxying)
- `/admin/*` - Admin API routes (configuration management)
- `/healthz`, `/readyz` - Health checks
- `/metrics` - Prometheus metrics

**Middleware Stack** (in order):
1. RequestID, RealIP, Logger, Recoverer, Timeout
2. BodyBuffer (buffers request body for HMAC verification)
3. AuthContext (validates API keys via user-org-service)
4. RateLimit (Redis-based rate limiting)
5. Budget (budget enforcement)

**Auth Proxying**:
- API Router validates API keys via user-org-service
- For web portal auth, API Router should proxy auth requests to user-org-service
- Currently not configured (API router is not running/failing)

### 3. user-org-service (`services/user-org-service`)

**Purpose**:
- User and organization management
- Authentication (password, OAuth, OIDC)
- Authorization (RBAC, API keys)
- Session management

**Routes**:
- `/v1/auth/*` - Authentication endpoints
  - `POST /v1/auth/login` - Password login
  - `POST /v1/auth/refresh` - Token refresh
  - `POST /v1/auth/logout` - Logout
  - `POST /v1/auth/validate-api-key` - API key validation (for API router)
- `/v1/orgs/*` - Organization management
- `/v1/orgs/{orgId}/users/*` - User management
- `/healthz`, `/readyz` - Health checks
- `/metrics` - Prometheus metrics

**Current Status**:
- ✅ Running on port 8081
- ✅ CORS enabled for `localhost:5173`
- ✅ Login endpoint working
- ✅ Database connected (PostgreSQL on port 5433)

## Configuration Files

### Environment Profile: `configs/environments/local-dev.yaml`

```yaml
web_portal:
  environment_variables:
    - name: VITE_API_BASE_URL
      value: "http://localhost:8081/v1"  # ⚠️ Temporary: Direct to user-org-service
      # Should be: "http://localhost:8080/api" (via API router)

api_router_service:
  host: localhost
  port: 8080
  backend_endpoints: "mock-backend-1:http://localhost:8000/v1/completions"
  # ⚠️ Not currently routing auth requests to user-org-service

user_org_service:
  host: localhost
  port: 8081
  endpoints:
    health: "http://localhost:8081/healthz"
    api: "http://localhost:8081/v1"
```

### UI HTTP Client: `web/portal/src/lib/http/client.ts`

```typescript
// Default (if VITE_API_BASE_URL not set):
const apiBaseUrl = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080/api';

// Current local dev (from environment profile):
// VITE_API_BASE_URL = http://localhost:8081/v1

// All requests go through this client:
httpClient.post('/auth/login', {...}) 
// → http://localhost:8081/v1/auth/login (current)
// → http://localhost:8080/api/auth/login (intended)
```

## Request Examples

### Current (Bypassing API Router)

**Login Request**:
```http
POST http://localhost:8081/v1/auth/login HTTP/1.1
Host: localhost:8081
Origin: http://localhost:5173
Content-Type: application/json

{
  "email": "admin@example.com",
  "password": "nubipwdkryfmtaho123!"
}
```

**Response**:
```http
HTTP/1.1 200 OK
Access-Control-Allow-Origin: http://localhost:5173
Access-Control-Allow-Credentials: true
Content-Type: application/json

{
  "access_token": "ory_at_...",
  "expires_in": 3600,
  "token_type": "bearer"
}
```

### Intended (Via API Router)

**Login Request** (should be):
```http
POST http://localhost:8080/api/auth/login HTTP/1.1
Host: localhost:8080
Origin: http://localhost:5173
Content-Type: application/json

{
  "email": "admin@example.com",
  "password": "nubipwdkryfmtaho123!"
}
```

**API Router Processing**:
1. Receives request at `/api/auth/login`
2. Recognizes it's an auth request (not inference)
3. Proxies to `user-org-service:8081/v1/auth/login`
4. Returns response with CORS headers

## Current Issues

### ⚠️ API Router Not Running

**Problem**: API router service is not running or failing to start.

**Workaround**: UI is configured to bypass API router and call user-org-service directly.

**Why**: 
- API router likely requires Kafka/Config Service dependencies
- Need to investigate why API router isn't starting

### ⚠️ Missing Auth Routing in API Router

**Problem**: API router doesn't have routes configured to proxy auth requests to user-org-service.

**Needed**: API router should have routes like:
- `POST /api/auth/login` → proxies to `user-org-service/v1/auth/login`
- `POST /api/auth/refresh` → proxies to `user-org-service/v1/auth/refresh`
- `POST /api/auth/logout` → proxies to `user-org-service/v1/auth/logout`

## Summary

### Current Flow (Local Dev)
**UI → user-org-service → PostgreSQL**

### Intended Flow (Production)
**UI → API Router → user-org-service → PostgreSQL**

### Not Everything Goes Through API Router
- **Currently**: Nothing goes through API router (bypassed for local dev)
- **Intended**: All API requests should go through API router
- **API Router Purpose**: 
  - Single entry point
  - Authentication/authorization
  - Rate limiting
  - Budget enforcement
  - Request routing
  - Audit logging

