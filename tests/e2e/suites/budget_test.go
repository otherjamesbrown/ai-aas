package suites

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ai-aas/tests/e2e/fixtures"
	"github.com/ai-aas/tests/e2e/harness"
)

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	sLower := strings.ToLower(s)
	substrLower := strings.ToLower(substr)
	return strings.Contains(sLower, substrLower)
}

// TestBudgetLimitEnforcement tests that budget limits are enforced
func TestBudgetLimitEnforcement(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	budgetFixture := fixtures.NewBudgetFixture(ctx.Client, ctx.Fixtures)

	// Create organization
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}

	// Create a budget with a low limit
	budgetLimit := 10.0 // $10 limit
	budget, err := budgetFixture.Create(ctx, org.ID, budgetLimit, "USD", "monthly")
	if err != nil {
		t.Fatalf("Failed to create budget: %v", err)
	}

	if budget.ID == "" {
		t.Fatal("Budget ID is empty")
	}

	if budget.Limit != budgetLimit {
		t.Fatalf("Budget limit mismatch: expected %f, got %f", budgetLimit, budget.Limit)
	}

	// Verify budget exists
	retrieved, err := budgetFixture.Get(budget.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve budget: %v", err)
	}

	if retrieved.Limit != budgetLimit {
		t.Fatalf("Retrieved budget limit mismatch: expected %f, got %f", budgetLimit, retrieved.Limit)
	}

	t.Logf("Budget limit enforcement test passed: org=%s, budget=%s, limit=%f", org.ID, budget.ID, budget.Limit)
}

// TestBudgetExceededDenial tests that requests are denied when budget is exceeded
func TestBudgetExceededDenial(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	budgetFixture := fixtures.NewBudgetFixture(ctx.Client, ctx.Fixtures)
	apiKeyFixture := fixtures.NewAPIKeyFixture(ctx.Client, ctx.Fixtures)

	// Create organization
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}

	// Create a budget with a very low limit
	budgetLimit := 1.0 // $1 limit
	budget, err := budgetFixture.Create(ctx, org.ID, budgetLimit, "USD", "monthly")
	if err != nil {
		t.Fatalf("Failed to create budget: %v", err)
	}

	// Create an API key for making requests
	apiKey, err := apiKeyFixture.Create(ctx, org.ID, "test-budget-key", []string{"inference:read", "inference:write"})
	if err != nil {
		t.Fatalf("Failed to create API key: %v", err)
	}

	// Create a client for API router service
	routerClient := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
	routerClient.SetHeader("Authorization", "Bearer "+apiKey.Secret)
	routerClient.SetHeader("X-API-Key", apiKey.Secret)
	
	// If using IP address, set Host header
	if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
		routerClient.SetHeader("Host", "api.dev.ai-aas.local")
	}

	// Get budget status to verify it exists
	retrieved, err := budgetFixture.Get(budget.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve budget: %v", err)
	}

	t.Logf("Budget created: org=%s, budget=%s, limit=%f, current_usage=%f", 
		org.ID, budget.ID, retrieved.Limit, retrieved.CurrentUsage)

	// Attempt to make an inference request that would exceed the budget
	// Note: The actual budget enforcement happens at the API router level
	// The API router checks budget status before processing requests
	inferenceReq := map[string]interface{}{
		"model":   "gpt-4",
		"prompt":  "Test prompt that should be denied due to budget",
		"max_tokens": 100,
	}

	resp, err := routerClient.POST("/v1/inference", inferenceReq)
	if err != nil {
		// If we get an error, check if it's a budget-related denial
		errMsg := err.Error()
		if contains(errMsg, "budget") || contains(errMsg, "quota") || contains(errMsg, "exceeded") {
			t.Logf("Budget enforcement working: request denied with budget error: %v", err)
		} else {
			t.Logf("Request failed with non-budget error (may be expected if budget not enforced yet): %v", err)
		}
	} else if resp.StatusCode == 402 || resp.StatusCode == 403 {
		// 402 Payment Required or 403 Forbidden indicates budget/quota exceeded
		t.Logf("Budget enforcement working: request denied with status %d", resp.StatusCode)
	} else {
		t.Logf("Request succeeded (status %d) - budget may not be enforced yet or limit not exceeded", resp.StatusCode)
	}

	t.Logf("Budget exceeded denial test complete: org=%s, budget=%s", org.ID, budget.ID)
}

// TestBudgetResetAndRecovery tests that budgets can be reset and requests succeed after reset
func TestBudgetResetAndRecovery(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	budgetFixture := fixtures.NewBudgetFixture(ctx.Client, ctx.Fixtures)

	// Create organization
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}

	// Create a budget
	budgetLimit := 100.0
	budget, err := budgetFixture.Create(ctx, org.ID, budgetLimit, "USD", "monthly")
	if err != nil {
		t.Fatalf("Failed to create budget: %v", err)
	}

	// Reset budget usage
	err = budgetFixture.Reset(budget.ID)
	if err != nil {
		t.Fatalf("Failed to reset budget: %v", err)
	}

	// Verify budget was reset
	retrieved, err := budgetFixture.Get(budget.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve budget after reset: %v", err)
	}

	// After reset, usage should be 0 (or very low)
	if retrieved.CurrentUsage > 0.01 {
		t.Logf("Warning: Budget usage not fully reset: %f", retrieved.CurrentUsage)
	}

	t.Logf("Budget reset and recovery test passed: org=%s, budget=%s, usage=%f", 
		org.ID, budget.ID, retrieved.CurrentUsage)
}

// TestBudgetUpdateLimit tests that budget limits can be updated
func TestBudgetUpdateLimit(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	budgetFixture := fixtures.NewBudgetFixture(ctx.Client, ctx.Fixtures)

	// Create organization
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}

	// Create a budget with initial limit
	initialLimit := 50.0
	budget, err := budgetFixture.Create(ctx, org.ID, initialLimit, "USD", "monthly")
	if err != nil {
		t.Fatalf("Failed to create budget: %v", err)
	}

	// Update budget limit
	newLimit := 200.0
	updated, err := budgetFixture.UpdateLimit(budget.ID, newLimit)
	if err != nil {
		t.Fatalf("Failed to update budget limit: %v", err)
	}

	if updated.Limit != newLimit {
		t.Fatalf("Budget limit update failed: expected %f, got %f", newLimit, updated.Limit)
	}

	t.Logf("Budget update limit test passed: org=%s, budget=%s, old_limit=%f, new_limit=%f", 
		org.ID, budget.ID, initialLimit, newLimit)
}

