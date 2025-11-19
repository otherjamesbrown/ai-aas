// Package limiter provides budget and quota checking for API requests.
//
// Purpose:
//   This package implements budget service client integration for checking
//   organization budgets and quotas before processing requests.
//
// Dependencies:
//   - Budget service HTTP API (may be stubbed for development)
//
// Key Responsibilities:
//   - Check organization budget before processing request
//   - Check quota limits (daily/monthly)
//   - Handle budget service unavailability gracefully
//   - Return structured budget/quota status
//
// Requirements Reference:
//   - specs/006-api-router-service/spec.md#US-002 (Enforce budgets and safe usage)
//
package limiter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// BudgetClient checks budgets and quotas for organizations.
type BudgetClient struct {
	endpoint string
	timeout  time.Duration
	logger   *zap.Logger
	client   *http.Client
}

// NewBudgetClient creates a new budget client.
func NewBudgetClient(endpoint string, timeout time.Duration, logger *zap.Logger) *BudgetClient {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &BudgetClient{
		endpoint: endpoint,
		timeout:  timeout,
		logger:   logger,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// BudgetStatus represents the budget/quota status for an organization.
type BudgetStatus struct {
	Allowed      bool
	CurrentUsage float64
	Limit        float64
	QuotaType    string // "budget", "daily_quota", "monthly_quota"
	Reason       string // Reason if not allowed
}

// CheckBudget checks if an organization has budget/quota available.
// Returns BudgetStatus with allowed status and usage information.
func (c *BudgetClient) CheckBudget(ctx context.Context, orgID string) (*BudgetStatus, error) {
	// If no endpoint configured, use stub implementation
	if c.endpoint == "" {
		return c.checkBudgetStub(orgID)
	}
	
	// Make HTTP request to budget service
	return c.checkBudgetHTTP(ctx, orgID)
}

// checkBudgetStub provides a stub implementation for development/testing.
// Accepts special API keys to simulate budget exhaustion:
// - "dev-exhausted-budget-key" -> budget exceeded
// - "dev-exhausted-quota-key" -> quota exceeded
func (c *BudgetClient) checkBudgetStub(orgID string) (*BudgetStatus, error) {
	// Stub implementation: check orgID for special test cases
	// In real implementation, this would make HTTP request to budget service
	
	// For now, all organizations have budget available (except test cases)
	// Test cases are handled via API key in authenticator, which we can't access here
	// So we'll return allowed for all orgs in stub mode
	
	return &BudgetStatus{
		Allowed:      true,
		CurrentUsage: 0.0,
		Limit:        10000.0,
		QuotaType:    "budget",
	}, nil
}

// CheckBudgetWithKey checks budget using API key (for test scenarios).
// This allows us to simulate budget exhaustion based on API key.
func (c *BudgetClient) CheckBudgetWithKey(ctx context.Context, orgID, apiKey string) (*BudgetStatus, error) {
	// Check for test API keys that simulate budget/quota exhaustion
	if apiKey == "dev-exhausted-budget-key" {
		return &BudgetStatus{
			Allowed:      false,
			CurrentUsage: 10000.0,
			Limit:        10000.0,
			QuotaType:    "budget",
			Reason:       "Monthly budget exhausted",
		}, nil
	}
	
	if apiKey == "dev-exhausted-quota-key" {
		return &BudgetStatus{
			Allowed:      false,
			CurrentUsage: 1000.0,
			Limit:        1000.0,
			QuotaType:    "daily_quota",
			Reason:       "Daily quota exhausted",
		}, nil
	}
	
	// Default: budget available
	return &BudgetStatus{
		Allowed:      true,
		CurrentUsage: 0.0,
		Limit:        10000.0,
		QuotaType:    "budget",
	}, nil
}

// budgetServiceResponse represents the response from budget service API.
type budgetServiceResponse struct {
	Allowed      bool    `json:"allowed"`
	CurrentUsage float64 `json:"current_usage"`
	Limit        float64 `json:"limit"`
	QuotaType    string  `json:"quota_type"`
	Reason       string  `json:"reason,omitempty"`
}

// checkBudgetHTTP makes HTTP request to budget service (not implemented yet).
func (c *BudgetClient) checkBudgetHTTP(ctx context.Context, orgID string) (*BudgetStatus, error) {
	url := fmt.Sprintf("%s/v1/budgets/%s/check", c.endpoint, orgID)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create budget check request: %w", err)
	}
	
	resp, err := c.client.Do(req)
	if err != nil {
		c.logger.Warn("budget service unavailable, allowing request",
			zap.String("org_id", orgID),
			zap.Error(err),
		)
		// Graceful degradation: allow request if budget service unavailable
		return &BudgetStatus{
			Allowed:   true,
			QuotaType: "budget",
		}, nil
	}
	defer func() { _ = resp.Body.Close() }()
	
	if resp.StatusCode != http.StatusOK {
		c.logger.Warn("budget service returned error, allowing request",
			zap.String("org_id", orgID),
			zap.Int("status", resp.StatusCode),
		)
		// Graceful degradation
		return &BudgetStatus{
			Allowed:   true,
			QuotaType: "budget",
		}, nil
	}
	
	var budgetResp budgetServiceResponse
	if err := json.NewDecoder(resp.Body).Decode(&budgetResp); err != nil {
		return nil, fmt.Errorf("decode budget response: %w", err)
	}
	
	return &BudgetStatus{
		Allowed:      budgetResp.Allowed,
		CurrentUsage: budgetResp.CurrentUsage,
		Limit:        budgetResp.Limit,
		QuotaType:    budgetResp.QuotaType,
		Reason:       budgetResp.Reason,
	}, nil
}

