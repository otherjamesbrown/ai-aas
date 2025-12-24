package main

import (
	"context"
	"testing"
	"time"
)

func TestCheckComponent(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	// Test unknown component
	status := checkComponent(ctx, "unknown-component")
	if status.State != "unknown" {
		t.Errorf("Expected state 'unknown', got '%s'", status.State)
	}

	// Test with timeout context
	fastCtx, fastCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer fastCancel()

	// Test that checkComponent returns valid structure even without services running
	// In CI, services won't be available, so we just verify the function doesn't panic
	// and returns sensible values
	status = checkComponent(fastCtx, "postgres")
	if status.Name != "postgres" {
		t.Errorf("Expected name 'postgres', got '%s'", status.Name)
	}
	// Latency should be non-negative (0 is valid for failed connections)
	if status.LatencyMs < 0 {
		t.Error("Latency should be non-negative")
	}
}

func TestCheckLocalComponents(t *testing.T) {
	ctx := context.Background()

	// Test all components
	components := checkLocalComponents(ctx, "")
	if len(components) == 0 {
		t.Error("Should return at least one component")
	}

	// Test filtered component
	filtered := checkLocalComponents(ctx, "postgres")
	if len(filtered) != 1 {
		t.Errorf("Expected 1 component, got %d", len(filtered))
	}
	if filtered[0].Name != "postgres" {
		t.Errorf("Expected component 'postgres', got '%s'", filtered[0].Name)
	}
}

func TestComponentStatusFields(t *testing.T) {
	status := ComponentStatus{
		Name:      "test",
		State:     "healthy",
		LatencyMs: 100,
		Message:   "test message",
		Endpoint:  "localhost:8080",
	}

	if status.Name != "test" {
		t.Error("Name should be set correctly")
	}
	if status.State != "healthy" {
		t.Error("State should be set correctly")
	}
	if status.LatencyMs != 100 {
		t.Error("LatencyMs should be set correctly")
	}
}

func TestStatusOutput(t *testing.T) {
	output := StatusOutput{
		Timestamp:  "2025-01-01T00:00:00Z",
		Mode:       "local",
		Components: []ComponentStatus{},
		Overall:    "healthy",
	}

	if output.Mode != "local" {
		t.Error("Mode should be set correctly")
	}
	if output.Overall != "healthy" {
		t.Error("Overall should be set correctly")
	}
}

// Note: Actual health check tests (checkPostgres, checkRedis, etc.) would require
// either mock servers or integration test setup with actual services running.
// These are left as integration tests rather than unit tests.
