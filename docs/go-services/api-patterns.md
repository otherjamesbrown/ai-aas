# API Patterns

---
last_updated: 2025-12-08
document_type: guide
---

## Overview

This document defines the REST API conventions used across all Go services in the AI-AAS platform.

## URL Structure

### Versioning

All APIs are versioned under `/v1/`:

```
/v1/resource
/v1/resource/{id}
/v1/resource/{id}/subresource
```

### Naming Conventions

- Use plural nouns for resources: `/v1/organizations` not `/v1/organization`
- Use kebab-case for multi-word resources: `/v1/routing-policies`
- Use path parameters for identifiers: `/v1/models/{name}`

## HTTP Methods

| Method | Purpose | Idempotent |
|--------|---------|------------|
| GET | Retrieve resource(s) | Yes |
| POST | Create new resource | No |
| PATCH | Partial update | Yes |
| PUT | Full replacement | Yes |
| DELETE | Remove resource | Yes |

## Response Formats

### Success Response

```json
{
  "data": { ... },
  "meta": {
    "request_id": "uuid",
    "timestamp": "2025-01-01T00:00:00Z"
  }
}
```

### List Response

```json
{
  "data": [ ... ],
  "meta": {
    "total": 100,
    "page": 1,
    "per_page": 20,
    "request_id": "uuid"
  }
}
```

### Error Response

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Human readable message",
    "details": { ... }
  },
  "meta": {
    "request_id": "uuid"
  }
}
```

## HTTP Status Codes

| Code | Meaning | Use When |
|------|---------|----------|
| 200 | OK | Successful GET, PATCH, PUT |
| 201 | Created | Successful POST creating resource |
| 204 | No Content | Successful DELETE |
| 400 | Bad Request | Invalid input, validation errors |
| 401 | Unauthorized | Missing or invalid authentication |
| 403 | Forbidden | Valid auth but insufficient permissions |
| 404 | Not Found | Resource doesn't exist |
| 409 | Conflict | Resource already exists, state conflict |
| 422 | Unprocessable Entity | Valid JSON but semantic errors |
| 429 | Too Many Requests | Rate limited |
| 500 | Internal Server Error | Unexpected server errors |
| 503 | Service Unavailable | Service temporarily unavailable |

## Authentication

All authenticated endpoints require the `X-API-Key` header:

```
X-API-Key: your-api-key-here
```

## Pagination

Use query parameters for pagination:

```
GET /v1/models?page=1&per_page=20
```

Default `per_page` is 20, maximum is 100.

## Filtering

Use query parameters for filtering:

```
GET /v1/models?status=active&owner=org-123
```

## Health Endpoints

Every service must implement:

| Endpoint | Purpose | Auth Required |
|----------|---------|---------------|
| `/health` or `/healthz` | Liveness probe | No |
| `/ready` or `/readyz` | Readiness probe | No |
| `/metrics` | Prometheus metrics | No |

## Implementation Example

```go
// Handler registration
func RegisterRoutes(r *mux.Router, h *Handlers) {
    api := r.PathPrefix("/v1").Subrouter()

    // Models
    api.HandleFunc("/models", h.ListModels).Methods("GET")
    api.HandleFunc("/models", h.CreateModel).Methods("POST")
    api.HandleFunc("/models/{name}", h.GetModel).Methods("GET")
    api.HandleFunc("/models/{name}", h.UpdateModel).Methods("PATCH")
    api.HandleFunc("/models/{name}", h.DeleteModel).Methods("DELETE")
}

// Response helper
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(map[string]interface{}{
        "data": data,
        "meta": map[string]interface{}{
            "request_id": middleware.GetRequestID(r.Context()),
            "timestamp":  time.Now().UTC().Format(time.RFC3339),
        },
    })
}
```

## Related Documents

- [error-handling.md](error-handling.md) - Error response patterns
- Service READMEs for endpoint-specific documentation
