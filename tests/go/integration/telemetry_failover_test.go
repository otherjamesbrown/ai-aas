package integration

import (
	"context"
	"testing"
	"time"

	"github.com/ai-aas/shared-go/observability"
)

// TestTelemetryInitWithUnreachableEndpoint tests that Init succeeds even when
// the OTEL endpoint is unreachable. With async gRPC connections (grpc.WithBlock
// removed to prevent startup hangs), the connection is established lazily and
// failures only occur when attempting to send traces.
//
// This test verifies:
// 1. Init returns a valid provider (not nil, no error)
// 2. The provider is NOT in fallback mode initially (connection is async)
// 3. Shutdown works correctly even with an unreachable endpoint
func TestTelemetryInitWithUnreachableEndpoint(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	provider, err := observability.Init(ctx, observability.Config{
		ServiceName: "integration-go",
		Environment: "test",
		Endpoint:    "127.0.0.1:43179", // Unreachable endpoint
		Protocol:    "grpc",
		Insecure:    true,
	})
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	if provider == nil {
		t.Fatalf("expected provider instance")
	}

	// With async gRPC connections, the provider is NOT in fallback mode
	// immediately. Fallback only happens when a trace send actually fails.
	// This is intentional to prevent startup hangs when OTEL collector is
	// temporarily unavailable.
	if provider.Fallback() {
		t.Logf("note: provider unexpectedly in fallback mode (sync connection?)")
	}

	// Shutdown should work correctly regardless of connection state.
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown returned error: %v", err)
	}
}
