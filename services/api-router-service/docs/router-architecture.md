# Router Architecture Guide

## Overview

The API Router Service uses a **two-tier router architecture** to handle chi router's middleware ordering constraint while keeping health endpoints accessible without authentication.

## Architecture Pattern

### Main Router (`router`)
- **Purpose**: Base middleware and public endpoints
- **Middleware**: RequestID, RealIP, Logger, Recoverer, Timeout
- **Routes**:
  - `/v1/status/healthz` - Liveness probe (no auth)
  - `/v1/status/readyz` - Readiness probe (no auth)
  - `/metrics` - Prometheus metrics (no auth)

### Sub-Router (`appRouter`)
- **Purpose**: Authenticated application routes
- **Middleware Chain** (order is critical):
  1. `BodyBufferMiddleware` - Buffers request body for HMAC verification
  2. `AuthContextMiddleware` - Validates API key and sets auth context
  3. `RateLimitMiddleware` - Enforces rate limits per org/key
  4. `BudgetMiddleware` - Enforces budget/quota limits
- **Routes**: All routes registered via `RegisterRoutes()` methods

## Why This Pattern?

### Chi Router Constraint
chi router requires **ALL middleware to be registered BEFORE ANY routes**. This means:
- ❌ You cannot register routes, then add middleware
- ✅ You must register all middleware first, then all routes

### The Problem We Solved
We needed:
1. Health endpoints accessible without authentication (for Kubernetes probes)
2. Application routes with full middleware chain (auth, rate limit, budget)
3. All middleware registered before routes (chi requirement)

### The Solution
By using a sub-router:
- Health endpoints are registered on main router **before** sub-router is created
- Sub-router has all middleware registered **before** routes are added
- Sub-router is mounted on main router, so routes work at their original paths

## Middleware Order Requirements

The middleware order is **critical** and must not be changed without understanding dependencies:

### 1. BodyBufferMiddleware (First)
- **Why first**: Buffers request body for reuse
- **Used by**: HMAC verification, model extraction, request parsing
- **Dependencies**: None

### 2. AuthContextMiddleware (Second)
- **Why second**: Needs buffered body for HMAC signature verification
- **Sets**: Auth context (user, org, API key) for downstream middleware
- **Dependencies**: BodyBufferMiddleware

### 3. RateLimitMiddleware (Third)
- **Why third**: Needs auth context to identify user/org for rate limiting
- **Checks**: Rate limits per organization or API key
- **Dependencies**: AuthContextMiddleware

### 4. BudgetMiddleware (Fourth)
- **Why fourth**: Needs auth context to identify user/org for budget checks
- **Checks**: Budget/quota limits after rate limit passes
- **Dependencies**: AuthContextMiddleware

## Adding New Routes

### Public Endpoints (No Authentication)
```go
// Register on main router BEFORE mounting appRouter
router.Get("/public/endpoint", handler)
```

### Authenticated Endpoints
```go
// Register on appRouter (will go through all middleware)
appRouter.Post("/v1/your-endpoint", handler)
```

### Admin Endpoints
```go
// Register on appRouter (already has auth middleware)
adminHandler.RegisterRoutes(appRouter)
```

## Common Pitfalls

### ❌ Don't: Register routes before middleware
```go
// This will panic!
router.Get("/route", handler)
router.Use(middleware) // ERROR: middleware must come before routes
```

### ❌ Don't: Register health endpoints on sub-router
```go
// This would require authentication for health checks
appRouter.Get("/v1/status/healthz", handler) // BAD - breaks Kubernetes probes
```

### ❌ Don't: Change middleware order
```go
// This breaks HMAC verification
appRouter.Use(authMiddleware) // Needs body buffer first!
appRouter.Use(bodyBufferMiddleware)
```

### ✅ Do: Follow the pattern
```go
// 1. Register public routes on main router
router.Get("/v1/status/healthz", handler)

// 2. Create sub-router and register middleware
appRouter := chi.NewRouter()
appRouter.Use(bodyBufferMiddleware)
appRouter.Use(authMiddleware)
// ... other middleware

// 3. Register authenticated routes on sub-router
appRouter.Post("/v1/inference", handler)

// 4. Mount sub-router
router.Mount("/", appRouter)
```

## Testing Considerations

When writing integration tests, you must replicate the middleware setup:

```go
router := chi.NewRouter()
tracer := otel.Tracer("test")
router.Use(public.BodyBufferMiddleware(64 * 1024))
router.Use(public.AuthContextMiddleware(authenticator, logger, tracer))
// ... add other middleware as needed
handler.RegisterRoutes(router)
```

See `test/integration/` for examples.

## References

- [chi router documentation](https://github.com/go-chi/chi)
- Issue fixed: PR #20 - Middleware registration order
- Related code: `cmd/router/main.go` lines 114-328

