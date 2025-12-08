# Testing Guide

---
last_updated: 2025-12-08
document_type: guide
---

## Overview

This document defines testing patterns for Go services in the AI-AAS platform.

## Test Organization

```
services/<name>/
├── internal/
│   ├── handlers/
│   │   ├── models.go
│   │   └── models_test.go      # Unit tests next to code
│   └── service/
│       ├── models.go
│       └── models_test.go
└── tests/
    └── integration/
        └── models_test.go       # Integration tests
```

## Running Tests

```bash
# Run all tests for a service
make test SERVICE=api-router-service

# Or directly with go test
go test ./services/api-router-service/...

# With race detection
go test -race ./services/api-router-service/...

# With coverage
go test -cover ./services/api-router-service/...

# Verbose output
go test -v ./services/api-router-service/...
```

## Unit Tests

### Table-Driven Tests

Use table-driven tests for comprehensive coverage:

```go
func TestValidateModel(t *testing.T) {
    tests := []struct {
        name    string
        model   *Model
        wantErr bool
        errCode string
    }{
        {
            name:    "valid model",
            model:   &Model{Name: "gpt-4", Endpoint: "http://localhost:8000"},
            wantErr: false,
        },
        {
            name:    "missing name",
            model:   &Model{Endpoint: "http://localhost:8000"},
            wantErr: true,
            errCode: "VALIDATION_ERROR",
        },
        {
            name:    "invalid endpoint",
            model:   &Model{Name: "gpt-4", Endpoint: "not-a-url"},
            wantErr: true,
            errCode: "VALIDATION_ERROR",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateModel(tt.model)
            if (err != nil) != tt.wantErr {
                t.Errorf("ValidateModel() error = %v, wantErr %v", err, tt.wantErr)
            }
            if tt.wantErr && tt.errCode != "" {
                var apiErr *APIError
                if errors.As(err, &apiErr) && apiErr.Code != tt.errCode {
                    t.Errorf("ValidateModel() error code = %v, want %v", apiErr.Code, tt.errCode)
                }
            }
        })
    }
}
```

### Mocking Dependencies

Use interfaces for dependencies:

```go
// Define interface
type ModelRepository interface {
    Get(ctx context.Context, name string) (*Model, error)
    Create(ctx context.Context, model *Model) error
    List(ctx context.Context, opts ListOptions) ([]*Model, error)
}

// Mock implementation
type MockModelRepository struct {
    GetFunc    func(ctx context.Context, name string) (*Model, error)
    CreateFunc func(ctx context.Context, model *Model) error
    ListFunc   func(ctx context.Context, opts ListOptions) ([]*Model, error)
}

func (m *MockModelRepository) Get(ctx context.Context, name string) (*Model, error) {
    return m.GetFunc(ctx, name)
}

// Usage in tests
func TestServiceGetModel(t *testing.T) {
    mockRepo := &MockModelRepository{
        GetFunc: func(ctx context.Context, name string) (*Model, error) {
            if name == "existing" {
                return &Model{Name: "existing"}, nil
            }
            return nil, &NotFoundError{Resource: "model", ID: name}
        },
    }

    svc := NewService(mockRepo)

    t.Run("existing model", func(t *testing.T) {
        model, err := svc.GetModel(context.Background(), "existing")
        if err != nil {
            t.Fatalf("unexpected error: %v", err)
        }
        if model.Name != "existing" {
            t.Errorf("got name %q, want %q", model.Name, "existing")
        }
    })

    t.Run("not found", func(t *testing.T) {
        _, err := svc.GetModel(context.Background(), "missing")
        if err == nil {
            t.Fatal("expected error, got nil")
        }
        var notFound *NotFoundError
        if !errors.As(err, &notFound) {
            t.Errorf("expected NotFoundError, got %T", err)
        }
    })
}
```

## HTTP Handler Tests

Use `httptest` for handler tests:

```go
func TestGetModelHandler(t *testing.T) {
    // Setup
    mockService := &MockModelService{
        GetFunc: func(ctx context.Context, name string) (*Model, error) {
            if name == "gpt-4" {
                return &Model{Name: "gpt-4", Status: "active"}, nil
            }
            return nil, &NotFoundError{Resource: "model", ID: name}
        },
    }
    handler := NewHandler(mockService)

    tests := []struct {
        name       string
        modelName  string
        wantStatus int
        wantBody   string
    }{
        {
            name:       "existing model",
            modelName:  "gpt-4",
            wantStatus: http.StatusOK,
            wantBody:   `"name":"gpt-4"`,
        },
        {
            name:       "not found",
            modelName:  "missing",
            wantStatus: http.StatusNotFound,
            wantBody:   `"code":"NOT_FOUND"`,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            req := httptest.NewRequest("GET", "/v1/models/"+tt.modelName, nil)
            rec := httptest.NewRecorder()

            // If using gorilla/mux, set path vars
            req = mux.SetURLVars(req, map[string]string{"name": tt.modelName})

            handler.GetModel(rec, req)

            if rec.Code != tt.wantStatus {
                t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
            }
            if !strings.Contains(rec.Body.String(), tt.wantBody) {
                t.Errorf("body = %s, want to contain %s", rec.Body.String(), tt.wantBody)
            }
        })
    }
}
```

## Integration Tests

Integration tests use real dependencies (database, etc.):

```go
// +build integration

func TestModelRepositoryIntegration(t *testing.T) {
    // Skip if not running integration tests
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    // Setup test database
    db := setupTestDB(t)
    defer db.Close()

    repo := NewModelRepository(db)
    ctx := context.Background()

    t.Run("create and get", func(t *testing.T) {
        model := &Model{Name: "test-model", Endpoint: "http://localhost:8000"}

        err := repo.Create(ctx, model)
        if err != nil {
            t.Fatalf("Create failed: %v", err)
        }

        got, err := repo.Get(ctx, model.Name)
        if err != nil {
            t.Fatalf("Get failed: %v", err)
        }

        if got.Name != model.Name {
            t.Errorf("Name = %q, want %q", got.Name, model.Name)
        }
    })
}
```

## Test Utilities

### Test Fixtures

```go
// testdata/fixtures.go
func NewTestModel(overrides ...func(*Model)) *Model {
    m := &Model{
        Name:     "test-model",
        Endpoint: "http://localhost:8000",
        Status:   "active",
    }
    for _, override := range overrides {
        override(m)
    }
    return m
}

// Usage
model := NewTestModel(func(m *Model) {
    m.Status = "inactive"
})
```

### Context with Timeout

```go
func TestWithTimeout(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    // Use ctx in test...
}
```

## Coverage Requirements

- Aim for >80% coverage on business logic
- 100% coverage on error paths
- Don't chase coverage on trivial code (getters, simple constructors)

## Related Documents

- [error-handling.md](error-handling.md) - Testing error conditions
- [api-patterns.md](api-patterns.md) - API contracts to test against
