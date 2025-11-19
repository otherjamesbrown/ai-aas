// Package limiter provides unit tests for budget client functionality.
//
// Purpose:
//   These tests validate the budget service client implementation,
//   including stub mode and HTTP integration.
//
package limiter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"
)

// TestBudgetClient_CheckBudget_StubMode tests budget checking in stub mode.
func TestBudgetClient_CheckBudget_StubMode(t *testing.T) {
	logger := zap.NewNop()
	client := NewBudgetClient("", 2*time.Second, logger)

	ctx := context.Background()
	status, err := client.CheckBudget(ctx, "test-org-1")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !status.Allowed {
		t.Error("expected budget to be allowed in stub mode")
	}
	if status.CurrentUsage != 0.0 {
		t.Errorf("expected current usage 0.0, got %f", status.CurrentUsage)
	}
	if status.Limit != 10000.0 {
		t.Errorf("expected limit 10000.0, got %f", status.Limit)
	}
	if status.QuotaType != "budget" {
		t.Errorf("expected quota type 'budget', got %s", status.QuotaType)
	}
}

// TestBudgetClient_CheckBudgetWithKey_ExhaustedBudget tests budget exhaustion scenario.
func TestBudgetClient_CheckBudgetWithKey_ExhaustedBudget(t *testing.T) {
	logger := zap.NewNop()
	client := NewBudgetClient("", 2*time.Second, logger)

	ctx := context.Background()
	status, err := client.CheckBudgetWithKey(ctx, "test-org-1", "dev-exhausted-budget-key")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.Allowed {
		t.Error("expected budget to be denied")
	}
	if status.CurrentUsage != 10000.0 {
		t.Errorf("expected current usage 10000.0, got %f", status.CurrentUsage)
	}
	if status.Limit != 10000.0 {
		t.Errorf("expected limit 10000.0, got %f", status.Limit)
	}
	if status.QuotaType != "budget" {
		t.Errorf("expected quota type 'budget', got %s", status.QuotaType)
	}
	if status.Reason == "" {
		t.Error("expected reason to be set")
	}
}

// TestBudgetClient_CheckBudgetWithKey_ExhaustedQuota tests quota exhaustion scenario.
func TestBudgetClient_CheckBudgetWithKey_ExhaustedQuota(t *testing.T) {
	logger := zap.NewNop()
	client := NewBudgetClient("", 2*time.Second, logger)

	ctx := context.Background()
	status, err := client.CheckBudgetWithKey(ctx, "test-org-1", "dev-exhausted-quota-key")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.Allowed {
		t.Error("expected quota to be denied")
	}
	if status.QuotaType != "daily_quota" {
		t.Errorf("expected quota type 'daily_quota', got %s", status.QuotaType)
	}
	if status.Reason == "" {
		t.Error("expected reason to be set")
	}
}

// TestBudgetClient_CheckBudget_HTTP_Success tests successful HTTP budget check.
func TestBudgetClient_CheckBudget_HTTP_Success(t *testing.T) {
	// Create mock budget service
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if r.URL.Path != "/v1/budgets/test-org-1/check" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		response := budgetServiceResponse{
			Allowed:      true,
			CurrentUsage: 5000.0,
			Limit:        10000.0,
			QuotaType:    "budget",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	logger := zap.NewNop()
	client := NewBudgetClient(mockServer.URL, 2*time.Second, logger)

	ctx := context.Background()
	status, err := client.CheckBudget(ctx, "test-org-1")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !status.Allowed {
		t.Error("expected budget to be allowed")
	}
	if status.CurrentUsage != 5000.0 {
		t.Errorf("expected current usage 5000.0, got %f", status.CurrentUsage)
	}
	if status.Limit != 10000.0 {
		t.Errorf("expected limit 10000.0, got %f", status.Limit)
	}
}

// TestBudgetClient_CheckBudget_HTTP_Denied tests denied HTTP budget check.
func TestBudgetClient_CheckBudget_HTTP_Denied(t *testing.T) {
	// Create mock budget service that denies request
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := budgetServiceResponse{
			Allowed:      false,
			CurrentUsage: 10000.0,
			Limit:        10000.0,
			QuotaType:    "budget",
			Reason:       "Monthly budget exhausted",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	logger := zap.NewNop()
	client := NewBudgetClient(mockServer.URL, 2*time.Second, logger)

	ctx := context.Background()
	status, err := client.CheckBudget(ctx, "test-org-1")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.Allowed {
		t.Error("expected budget to be denied")
	}
	if status.Reason == "" {
		t.Error("expected reason to be set")
	}
}

// TestBudgetClient_CheckBudget_HTTP_Unavailable tests graceful degradation when budget service is unavailable.
func TestBudgetClient_CheckBudget_HTTP_Unavailable(t *testing.T) {
	logger := zap.NewNop()
	// Use invalid endpoint to simulate unavailable service
	client := NewBudgetClient("http://localhost:99999", 100*time.Millisecond, logger)

	ctx := context.Background()
	status, err := client.CheckBudget(ctx, "test-org-1")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should gracefully degrade and allow request
	if !status.Allowed {
		t.Error("expected request to be allowed when budget service unavailable")
	}
}

// TestBudgetClient_CheckBudget_HTTP_ErrorResponse tests error response handling.
func TestBudgetClient_CheckBudget_HTTP_ErrorResponse(t *testing.T) {
	// Create mock budget service that returns error status
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mockServer.Close()

	logger := zap.NewNop()
	client := NewBudgetClient(mockServer.URL, 2*time.Second, logger)

	ctx := context.Background()
	status, err := client.CheckBudget(ctx, "test-org-1")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should gracefully degrade and allow request
	if !status.Allowed {
		t.Error("expected request to be allowed when budget service returns error")
	}
}

// TestBudgetClient_CheckBudget_HTTP_InvalidJSON tests invalid JSON response handling.
func TestBudgetClient_CheckBudget_HTTP_InvalidJSON(t *testing.T) {
	// Create mock budget service that returns invalid JSON
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("invalid json"))
	}))
	defer mockServer.Close()

	logger := zap.NewNop()
	client := NewBudgetClient(mockServer.URL, 2*time.Second, logger)

	ctx := context.Background()
	_, err := client.CheckBudget(ctx, "test-org-1")

	if err == nil {
		t.Error("expected error when JSON is invalid")
	}
}

// TestBudgetClient_CheckBudgetWithKey_DefaultAllowed tests default allowed behavior.
func TestBudgetClient_CheckBudgetWithKey_DefaultAllowed(t *testing.T) {
	logger := zap.NewNop()
	client := NewBudgetClient("", 2*time.Second, logger)

	ctx := context.Background()
	status, err := client.CheckBudgetWithKey(ctx, "test-org-1", "normal-api-key")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !status.Allowed {
		t.Error("expected budget to be allowed for normal API key")
	}
}

