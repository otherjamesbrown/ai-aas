package observability

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestInitValidation(t *testing.T) {
	if _, err := Init(context.Background(), Config{}); err == nil {
		t.Fatalf("expected error when endpoint missing")
	}
}

func TestInitUnsupportedProtocolFallsBack(t *testing.T) {
	TelemetryExporterFailures().Reset()

	provider, err := Init(context.Background(), Config{
		ServiceName: "svc",
		Endpoint:    "collector:4317",
		Protocol:    "ws",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !provider.Fallback() {
		t.Fatalf("expected fallback provider")
	}

	wsMetric, err := TelemetryExporterFailures().GetMetricWithLabelValues("svc", "ws")
	if err != nil {
		t.Fatalf("expected ws metric: %v", err)
	}
	if v := testutil.ToFloat64(wsMetric); v < 1 {
		t.Fatalf("expected ws failure count >= 1, got %f", v)
	}

	degradedMetric, err := TelemetryExporterFailures().GetMetricWithLabelValues("svc", "degraded")
	if err != nil {
		t.Fatalf("expected degraded metric: %v", err)
	}
	if v := testutil.ToFloat64(degradedMetric); v < 1 {
		t.Fatalf("expected degraded failure count >= 1, got %f", v)
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
