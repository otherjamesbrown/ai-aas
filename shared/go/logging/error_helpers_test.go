package logging

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ai-aas/shared-go/errors"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

func TestNewErrorLogger(t *testing.T) {
	cfg := DefaultConfig().WithServiceName("test-service")
	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer logger.Sync()

	el := NewErrorLogger(logger)
	if el == nil {
		t.Error("NewErrorLogger() returned nil")
	}
	if el.logger != logger {
		t.Error("NewErrorLogger() did not set logger correctly")
	}
}

func TestErrorLogger_LogError(t *testing.T) {
	cfg := DefaultConfig().WithServiceName("test-service")
	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer logger.Sync()

	el := NewErrorLogger(logger)

	t.Run("nil error", func(t *testing.T) {
		// Should not panic with nil error
		el.LogError(context.Background(), nil, "test message")
	})

	t.Run("standard error", func(t *testing.T) {
		err := fmt.Errorf("test error")
		el.LogError(context.Background(), err, "test message")
	})

	t.Run("shared error type", func(t *testing.T) {
		sharedErr := &errors.Error{
			Code:      "TEST_ERROR",
			Message:   "Test error message",
			Detail:    "Some detail",
			RequestID: "req-123",
		}
		el.LogError(context.Background(), sharedErr, "test message")
	})

	t.Run("shared error without detail", func(t *testing.T) {
		sharedErr := &errors.Error{
			Code:    "TEST_ERROR",
			Message: "Test error message",
		}
		el.LogError(context.Background(), sharedErr, "test message")
	})

	t.Run("with trace context", func(t *testing.T) {
		tracer := otel.Tracer("test")
		ctx, span := tracer.Start(context.Background(), "test-span")
		defer span.End()

		err := fmt.Errorf("test error")
		el.LogError(ctx, err, "test message")
	})

	t.Run("with additional fields", func(t *testing.T) {
		err := fmt.Errorf("test error")
		el.LogError(context.Background(), err, "test message",
			zap.String("custom_field", "custom_value"),
		)
	})
}

func TestErrorLogger_LogWarn(t *testing.T) {
	cfg := DefaultConfig().WithServiceName("test-service")
	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer logger.Sync()

	el := NewErrorLogger(logger)

	t.Run("basic warning", func(t *testing.T) {
		el.LogWarn(context.Background(), "warning message")
	})

	t.Run("with trace context", func(t *testing.T) {
		tracer := otel.Tracer("test")
		ctx, span := tracer.Start(context.Background(), "test-span")
		defer span.End()

		el.LogWarn(ctx, "warning message")
	})

	t.Run("with additional fields", func(t *testing.T) {
		el.LogWarn(context.Background(), "warning message",
			zap.String("custom_field", "custom_value"),
		)
	})
}

func TestErrorLogger_LogHTTPError(t *testing.T) {
	cfg := DefaultConfig().WithServiceName("test-service")
	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer logger.Sync()

	el := NewErrorLogger(logger)

	t.Run("5xx error", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/test", nil)
		req.Header.Set("User-Agent", "TestAgent/1.0")
		err := fmt.Errorf("internal error")
		el.LogHTTPError(req.Context(), req, http.StatusInternalServerError, err)
	})

	t.Run("4xx error", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/test", nil)
		req.Header.Set("User-Agent", "TestAgent/1.0")
		err := fmt.Errorf("bad request")
		el.LogHTTPError(req.Context(), req, http.StatusBadRequest, err)
	})

	t.Run("without user agent", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/test", nil)
		err := fmt.Errorf("not found")
		el.LogHTTPError(req.Context(), req, http.StatusNotFound, err)
	})

	t.Run("3xx status (should not log)", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/test", nil)
		// 3xx status codes should not log as they are not errors
		el.LogHTTPError(req.Context(), req, http.StatusMovedPermanently, nil)
	})
}

func TestErrorLogger_LogPanic(t *testing.T) {
	cfg := DefaultConfig().WithServiceName("test-service")
	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer logger.Sync()

	el := NewErrorLogger(logger)

	t.Run("basic panic", func(t *testing.T) {
		el.LogPanic(context.Background(), "panic value", []byte("stack trace"))
	})

	t.Run("with trace context", func(t *testing.T) {
		tracer := otel.Tracer("test")
		ctx, span := tracer.Start(context.Background(), "test-span")
		defer span.End()

		el.LogPanic(ctx, "panic value", []byte("stack trace"))
	})

	t.Run("with additional fields", func(t *testing.T) {
		el.LogPanic(context.Background(), "panic value", []byte("stack trace"),
			zap.String("custom_field", "custom_value"),
		)
	})
}

func TestErrorLogger_LogDatabaseError(t *testing.T) {
	cfg := DefaultConfig().WithServiceName("test-service")
	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer logger.Sync()

	el := NewErrorLogger(logger)

	t.Run("basic database error", func(t *testing.T) {
		err := fmt.Errorf("connection refused")
		el.LogDatabaseError(context.Background(), "SELECT", err)
	})

	t.Run("with additional fields", func(t *testing.T) {
		err := fmt.Errorf("query timeout")
		el.LogDatabaseError(context.Background(), "INSERT", err,
			zap.String("table", "users"),
		)
	})
}

func TestErrorLogger_LogExternalServiceError(t *testing.T) {
	cfg := DefaultConfig().WithServiceName("test-service")
	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer logger.Sync()

	el := NewErrorLogger(logger)

	t.Run("basic external service error", func(t *testing.T) {
		err := fmt.Errorf("service unavailable")
		el.LogExternalServiceError(context.Background(), "payment-service", "/api/charge", 503, err)
	})

	t.Run("with additional fields", func(t *testing.T) {
		err := fmt.Errorf("timeout")
		el.LogExternalServiceError(context.Background(), "auth-service", "/validate", 504, err,
			zap.Duration("timeout", 30),
		)
	})
}

func TestErrorLogger_LogAuthError(t *testing.T) {
	cfg := DefaultConfig().WithServiceName("test-service")
	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer logger.Sync()

	el := NewErrorLogger(logger)

	t.Run("basic auth error", func(t *testing.T) {
		el.LogAuthError(context.Background(), "jwt", "token expired")
	})

	t.Run("with additional fields", func(t *testing.T) {
		el.LogAuthError(context.Background(), "api_key", "invalid key",
			zap.String("key_prefix", "sk_test"),
		)
	})
}

func TestErrorLogger_LogValidationError(t *testing.T) {
	cfg := DefaultConfig().WithServiceName("test-service")
	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer logger.Sync()

	el := NewErrorLogger(logger)

	t.Run("basic validation error", func(t *testing.T) {
		el.LogValidationError(context.Background(), "email", "invalid format")
	})

	t.Run("with additional fields", func(t *testing.T) {
		el.LogValidationError(context.Background(), "age", "must be positive",
			zap.Int("value", -5),
		)
	})
}

func TestErrorLogger_RecoverMiddleware(t *testing.T) {
	cfg := DefaultConfig().WithServiceName("test-service")
	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer logger.Sync()

	el := NewErrorLogger(logger)

	t.Run("no panic", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})

		middleware := el.RecoverMiddleware(handler)
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("with panic", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test panic")
		})

		middleware := el.RecoverMiddleware(handler)
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		// Should not panic, should recover and return 500
		middleware.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
		}
	})
}
