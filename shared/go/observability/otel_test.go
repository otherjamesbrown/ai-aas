package observability

import (
	"context"
	"testing"
	"time"
)

func TestInitValidation(t *testing.T) {
	if _, err := Init(context.Background(), Config{}); err == nil {
		t.Fatalf("expected error when endpoint missing")
	}
}

func TestBuildClientUnsupported(t *testing.T) {
	if _, err := Init(context.Background(), Config{
		ServiceName: "svc",
		Endpoint:    "collector:4317",
		Protocol:    "ws",
	}); err == nil {
		t.Fatalf("expected unsupported protocol error")
	}
}

func TestInitHTTPProvider(t *testing.T) {
	ctx := context.Background()
	provider, err := Init(ctx, Config{
		ServiceName: "svc",
		Environment: "test",
		Endpoint:    "collector:4318",
		Protocol:    "http",
		Headers: map[string]string{
			"authorization": "Bearer value",
		},
		Insecure: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Cleanup(func() {
		if shutdownErr := provider.Shutdown(ctx); shutdownErr != nil {
			t.Fatalf("shutdown error: %v", shutdownErr)
		}
	})
}

func TestBuildClientVariants(t *testing.T) {
	ctx := context.Background()
	if _, err := buildClient(ctx, Config{
		Endpoint: "collector:4317",
		Protocol: "grpc",
		Headers:  map[string]string{"x-test": "value"},
		Insecure: true,
	}); err != nil {
		t.Fatalf("unexpected grpc client error: %v", err)
	}

	if _, err := buildClient(ctx, Config{
		Endpoint: "collector:4318",
		Protocol: "http",
		Insecure: true,
	}); err != nil {
		t.Fatalf("unexpected http client error: %v", err)
	}
}

func TestMustInitPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic when configuration invalid")
		}
	}()
	MustInit(context.Background(), Config{})
}

func TestProviderShutdownNoop(t *testing.T) {
	var provider *Provider
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatalf("expected nil provider shutdown to be no-op: %v", err)
	}

	provider = &Provider{}
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatalf("expected zero provider shutdown to be no-op: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	active, err := Init(context.Background(), Config{
		ServiceName: "svc",
		Environment: "dev",
		Endpoint:    "collector:4318",
		Protocol:    "http",
		Insecure:    true,
	})
	if err != nil {
		t.Fatalf("unexpected init error: %v", err)
	}
	if err := active.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}
}
