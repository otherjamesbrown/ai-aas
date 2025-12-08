# Error Handling

---
last_updated: 2025-12-08
document_type: guide
---

## Overview

This document defines error handling patterns for Go services in the AI-AAS platform.

## Error Wrapping

Always wrap errors with context using `fmt.Errorf`:

```go
// GOOD: Adds context
if err != nil {
    return fmt.Errorf("failed to create organization %s: %w", orgID, err)
}

// BAD: Loses context
if err != nil {
    return err
}
```

## Structured Errors

Use custom error types for domain errors:

```go
// Define error types
type NotFoundError struct {
    Resource string
    ID       string
}

func (e *NotFoundError) Error() string {
    return fmt.Sprintf("%s not found: %s", e.Resource, e.ID)
}

// Usage
func GetModel(ctx context.Context, name string) (*Model, error) {
    model, err := db.FindModel(ctx, name)
    if err == sql.ErrNoRows {
        return nil, &NotFoundError{Resource: "model", ID: name}
    }
    if err != nil {
        return nil, fmt.Errorf("database error finding model %s: %w", name, err)
    }
    return model, nil
}
```

## Error Codes

Use consistent error codes across services:

| Code | HTTP Status | Meaning |
|------|-------------|---------|
| `VALIDATION_ERROR` | 400 | Input validation failed |
| `UNAUTHORIZED` | 401 | Authentication required |
| `FORBIDDEN` | 403 | Permission denied |
| `NOT_FOUND` | 404 | Resource not found |
| `CONFLICT` | 409 | Resource conflict |
| `RATE_LIMITED` | 429 | Too many requests |
| `INTERNAL_ERROR` | 500 | Unexpected error |
| `SERVICE_UNAVAILABLE` | 503 | Dependency unavailable |

## HTTP Error Responses

Use a consistent error response helper:

```go
type APIError struct {
    Code    string      `json:"code"`
    Message string      `json:"message"`
    Details interface{} `json:"details,omitempty"`
}

func respondError(w http.ResponseWriter, r *http.Request, status int, code, message string, details interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(map[string]interface{}{
        "error": APIError{
            Code:    code,
            Message: message,
            Details: details,
        },
        "meta": map[string]interface{}{
            "request_id": middleware.GetRequestID(r.Context()),
        },
    })
}

// Usage in handlers
func (h *Handler) GetModel(w http.ResponseWriter, r *http.Request) {
    name := mux.Vars(r)["name"]

    model, err := h.service.GetModel(r.Context(), name)
    if err != nil {
        var notFound *NotFoundError
        if errors.As(err, &notFound) {
            respondError(w, r, http.StatusNotFound, "NOT_FOUND", err.Error(), nil)
            return
        }
        log.Error("failed to get model", "error", err, "name", name)
        respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error", nil)
        return
    }

    respondJSON(w, http.StatusOK, model)
}
```

## Logging Errors

Log errors with structured context:

```go
// GOOD: Structured logging with context
log.Error("failed to process request",
    "error", err,
    "org_id", orgID,
    "user_id", userID,
    "request_id", requestID,
)

// BAD: Unstructured logging
log.Printf("Error: %v", err)
```

## Error Propagation

### Service Layer

Return wrapped errors, let handlers decide HTTP status:

```go
func (s *Service) CreateOrg(ctx context.Context, org *Organization) error {
    if err := s.validate(org); err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }

    if err := s.repo.Create(ctx, org); err != nil {
        return fmt.Errorf("failed to save organization: %w", err)
    }

    return nil
}
```

### Handler Layer

Map service errors to HTTP responses:

```go
func (h *Handler) CreateOrg(w http.ResponseWriter, r *http.Request) {
    // ... parse request ...

    if err := h.service.CreateOrg(r.Context(), org); err != nil {
        var validationErr *ValidationError
        if errors.As(err, &validationErr) {
            respondError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), validationErr.Fields)
            return
        }

        log.Error("failed to create organization", "error", err)
        respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create organization", nil)
        return
    }

    respondJSON(w, http.StatusCreated, org)
}
```

## Panic Recovery

Always use panic recovery middleware:

```go
func RecoveryMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if err := recover(); err != nil {
                log.Error("panic recovered",
                    "error", err,
                    "stack", string(debug.Stack()),
                    "request_id", middleware.GetRequestID(r.Context()),
                )
                respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error", nil)
            }
        }()
        next.ServeHTTP(w, r)
    })
}
```

## Related Documents

- [api-patterns.md](api-patterns.md) - API response formats
- [testing-guide.md](testing-guide.md) - Testing error conditions
