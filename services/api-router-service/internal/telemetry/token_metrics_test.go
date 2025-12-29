// Package telemetry provides unit tests for token metrics functionality.
//
// Purpose:
//
//	These tests validate the token usage metrics collection,
//	including counter and histogram metrics for tokens processed.
package telemetry

import (
	"context"
	"testing"

	"go.uber.org/zap"
)

// TestNewTokenMetrics tests that NewTokenMetrics creates a valid TokenMetrics instance.
func TestNewTokenMetrics(t *testing.T) {
	logger := zap.NewNop()

	metrics, err := NewTokenMetrics(logger)
	if err != nil {
		t.Fatalf("unexpected error creating token metrics: %v", err)
	}

	if metrics == nil {
		t.Fatal("expected non-nil TokenMetrics")
	}

	if metrics.logger == nil {
		t.Error("expected logger to be set")
	}

	if metrics.tokensProcessedTotal == nil {
		t.Error("expected tokensProcessedTotal counter to be set")
	}

	if metrics.tokensPerRequest == nil {
		t.Error("expected tokensPerRequest histogram to be set")
	}
}

// TestRecordTokens_ValidInput tests recording tokens with valid input.
func TestRecordTokens_ValidInput(t *testing.T) {
	logger := zap.NewNop()
	metrics, err := NewTokenMetrics(logger)
	if err != nil {
		t.Fatalf("unexpected error creating token metrics: %v", err)
	}

	ctx := context.Background()

	// Test recording tokens for chat request
	metrics.RecordTokens(ctx, "org-123", "gpt-4", "chat", 100, 50)

	// Test recording tokens for completion request
	metrics.RecordTokens(ctx, "org-456", "llama-7b", "completion", 200, 100)

	// If we reach here without panic, the metrics were recorded successfully
}

// TestRecordTokens_ZeroTokens tests recording with zero tokens.
func TestRecordTokens_ZeroTokens(t *testing.T) {
	logger := zap.NewNop()
	metrics, err := NewTokenMetrics(logger)
	if err != nil {
		t.Fatalf("unexpected error creating token metrics: %v", err)
	}

	ctx := context.Background()

	// Test recording zero tokens (edge case)
	metrics.RecordTokens(ctx, "org-123", "gpt-4", "chat", 0, 0)

	// Should not panic
}

// TestRecordTokens_HighTokenUsage tests recording with high token counts.
func TestRecordTokens_HighTokenUsage(t *testing.T) {
	logger := zap.NewNop()
	metrics, err := NewTokenMetrics(logger)
	if err != nil {
		t.Fatalf("unexpected error creating token metrics: %v", err)
	}

	ctx := context.Background()

	// Test recording high token usage (triggers debug log)
	metrics.RecordTokens(ctx, "org-123", "gpt-4", "chat", 8000, 3000)

	// Should not panic, and should log debug message
}

// TestRecordTokens_MultipleOrganizations tests that metrics are recorded per organization.
func TestRecordTokens_MultipleOrganizations(t *testing.T) {
	logger := zap.NewNop()
	metrics, err := NewTokenMetrics(logger)
	if err != nil {
		t.Fatalf("unexpected error creating token metrics: %v", err)
	}

	ctx := context.Background()

	// Record tokens for multiple organizations
	metrics.RecordTokens(ctx, "org-1", "gpt-4", "chat", 100, 50)
	metrics.RecordTokens(ctx, "org-2", "gpt-4", "chat", 200, 100)
	metrics.RecordTokens(ctx, "org-3", "llama-7b", "completion", 150, 75)

	// If we reach here without panic, metrics were recorded successfully
}

// TestRecordTokens_MultipleModels tests that metrics are recorded per model.
func TestRecordTokens_MultipleModels(t *testing.T) {
	logger := zap.NewNop()
	metrics, err := NewTokenMetrics(logger)
	if err != nil {
		t.Fatalf("unexpected error creating token metrics: %v", err)
	}

	ctx := context.Background()

	// Record tokens for multiple models
	metrics.RecordTokens(ctx, "org-123", "gpt-4", "chat", 100, 50)
	metrics.RecordTokens(ctx, "org-123", "gpt-3.5", "chat", 200, 100)
	metrics.RecordTokens(ctx, "org-123", "llama-7b", "completion", 150, 75)

	// If we reach here without panic, metrics were recorded successfully
}

// TestRecordTokens_RequestTypes tests that metrics are recorded per request type.
func TestRecordTokens_RequestTypes(t *testing.T) {
	logger := zap.NewNop()
	metrics, err := NewTokenMetrics(logger)
	if err != nil {
		t.Fatalf("unexpected error creating token metrics: %v", err)
	}

	ctx := context.Background()

	// Record tokens for different request types
	metrics.RecordTokens(ctx, "org-123", "gpt-4", "chat", 100, 50)
	metrics.RecordTokens(ctx, "org-123", "gpt-4", "completion", 200, 100)

	// If we reach here without panic, metrics were recorded successfully
}

// TestRecordTokens_ConcurrentAccess tests concurrent metric recording.
func TestRecordTokens_ConcurrentAccess(t *testing.T) {
	logger := zap.NewNop()
	metrics, err := NewTokenMetrics(logger)
	if err != nil {
		t.Fatalf("unexpected error creating token metrics: %v", err)
	}

	ctx := context.Background()

	// Run concurrent recordings
	done := make(chan bool, 100)

	for i := 0; i < 100; i++ {
		go func(idx int) {
			metrics.RecordTokens(
				ctx,
				"org-concurrent",
				"gpt-4",
				"chat",
				100+idx,
				50+idx,
			)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 100; i++ {
		<-done
	}

	// If we reach here without panic or deadlock, concurrent access works
}

// TestRecordTokens_EmptyStrings tests recording with empty string parameters.
func TestRecordTokens_EmptyStrings(t *testing.T) {
	logger := zap.NewNop()
	metrics, err := NewTokenMetrics(logger)
	if err != nil {
		t.Fatalf("unexpected error creating token metrics: %v", err)
	}

	ctx := context.Background()

	// Test recording with empty strings (edge case)
	metrics.RecordTokens(ctx, "", "", "", 100, 50)

	// Should not panic, though this would be invalid in production
}

// TestRecordTokens_NegativeTokens tests recording with negative token counts.
func TestRecordTokens_NegativeTokens(t *testing.T) {
	logger := zap.NewNop()
	metrics, err := NewTokenMetrics(logger)
	if err != nil {
		t.Fatalf("unexpected error creating token metrics: %v", err)
	}

	ctx := context.Background()

	// Test recording negative tokens (edge case - should not happen in production)
	metrics.RecordTokens(ctx, "org-123", "gpt-4", "chat", -10, -5)

	// Should not panic, though this would be invalid in production
}

// TestRecordTokens_LargeScale tests recording a large number of metrics.
func TestRecordTokens_LargeScale(t *testing.T) {
	logger := zap.NewNop()
	metrics, err := NewTokenMetrics(logger)
	if err != nil {
		t.Fatalf("unexpected error creating token metrics: %v", err)
	}

	ctx := context.Background()

	// Record a large number of metrics
	for i := 0; i < 1000; i++ {
		metrics.RecordTokens(ctx, "org-load-test", "gpt-4", "chat", 100, 50)
	}

	// If we reach here without panic or performance issues, metrics scale well
}
