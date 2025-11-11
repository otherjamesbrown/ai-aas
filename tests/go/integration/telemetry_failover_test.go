package integration

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/ai-aas/shared-go/observability"
)

func TestTelemetryFailoverToNoopProvider(t *testing.T) {
	observableCounter := observability.TelemetryExporterFailures()
	observableCounter.Reset()

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	provider, err := observability.Init(ctx, observability.Config{
		ServiceName: "integration-go",
		Environment: "test",
		Endpoint:    "127.0.0.1:43179",
		Protocol:    "grpc",
		Insecure:    true,
	})
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	if provider == nil {
		t.Fatalf("expected provider instance")
	}
	if !provider.Fallback() {
		t.Fatalf("expected provider to report fallback mode")
	}

	// The primary and http fallback exporters should record failures.
	grpcMetric, err := observableCounter.GetMetricWithLabelValues("integration-go", "grpc")
	if err != nil {
		t.Fatalf("expected grpc metric: %v", err)
	}
	if v := testutil.ToFloat64(grpcMetric); v < 1 {
		t.Fatalf("expected grpc failure count >= 1, got %f", v)
	}

	httpMetric, err := observableCounter.GetMetricWithLabelValues("integration-go", "http")
	if err != nil {
		t.Fatalf("expected http metric: %v", err)
	}
	if v := testutil.ToFloat64(httpMetric); v < 1 {
		t.Fatalf("expected http failure count >= 1, got %f", v)
	}

	// Shutdown should be safe even in fallback mode.
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown returned error: %v", err)
	}
}
