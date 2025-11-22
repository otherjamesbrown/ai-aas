package fixtures

import (
	"fmt"
	"time"

	"github.com/ai-aas/tests/e2e/harness"
)

// Budget represents a test budget fixture
type Budget struct {
	ID            string            `json:"id"`
	OrganizationID string           `json:"organization_id"`
	Limit         float64           `json:"limit"`
	Currency      string            `json:"currency"`
	Period        string            `json:"period"`
	CurrentUsage  float64           `json:"current_usage"`
	CreatedAt     time.Time         `json:"created_at"`
	Metadata      map[string]string `json:"metadata"`
}

// BudgetFixture provides methods for creating and managing budget fixtures
type BudgetFixture struct {
	client *harness.Client
	fm     *harness.FixtureManager
}

// NewBudgetFixture creates a new budget fixture manager
func NewBudgetFixture(client *harness.Client, fm *harness.FixtureManager) *BudgetFixture {
	return &BudgetFixture{
		client: client,
		fm:     fm,
	}
}

// Create creates a new budget for testing
func (bf *BudgetFixture) Create(ctx *harness.Context, orgID string, limit float64, currency string, period string) (*Budget, error) {
	if currency == "" {
		currency = "USD"
	}
	if period == "" {
		period = "monthly"
	}

	reqBody := map[string]interface{}{
		"organization_id": orgID,
		"limit":           limit,
		"currency":        currency,
		"period":          period,
	}

	resp, err := bf.client.POST("/v1/budgets", reqBody)
	if err != nil {
		return nil, fmt.Errorf("create budget: %w", err)
	}

	if resp.StatusCode != 201 {
		return nil, fmt.Errorf("create budget failed: status %d, body: %s", resp.StatusCode, resp.String())
	}

	var budget Budget
	if err := resp.UnmarshalJSON(&budget); err != nil {
		return nil, fmt.Errorf("unmarshal budget: %w", err)
	}

	// Register for cleanup
	bf.fm.Register("budget", budget.ID, map[string]string{
		"organization_id": orgID,
		"test_run_id": ctx.RunID,
	})

	return &budget, nil
}

// Get retrieves a budget by ID
func (bf *BudgetFixture) Get(id string) (*Budget, error) {
	resp, err := bf.client.GET(fmt.Sprintf("/v1/budgets/%s", id))
	if err != nil {
		return nil, fmt.Errorf("get budget: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("get budget failed: status %d, body: %s", resp.StatusCode, resp.String())
	}

	var budget Budget
	if err := resp.UnmarshalJSON(&budget); err != nil {
		return nil, fmt.Errorf("unmarshal budget: %w", err)
	}

	return &budget, nil
}

// UpdateLimit updates a budget's limit
func (bf *BudgetFixture) UpdateLimit(id string, newLimit float64) (*Budget, error) {
	reqBody := map[string]interface{}{
		"limit": newLimit,
	}

	resp, err := bf.client.PUT(fmt.Sprintf("/v1/budgets/%s", id), reqBody)
	if err != nil {
		return nil, fmt.Errorf("update budget: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("update budget failed: status %d, body: %s", resp.StatusCode, resp.String())
	}

	var budget Budget
	if err := resp.UnmarshalJSON(&budget); err != nil {
		return nil, fmt.Errorf("unmarshal budget: %w", err)
	}

	return &budget, nil
}

// Reset resets a budget's usage to zero
func (bf *BudgetFixture) Reset(id string) error {
	reqBody := map[string]interface{}{
		"reset_usage": true,
	}

	resp, err := bf.client.PUT(fmt.Sprintf("/v1/budgets/%s", id), reqBody)
	if err != nil {
		return fmt.Errorf("reset budget: %w", err)
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("reset budget failed: status %d, body: %s", resp.StatusCode, resp.String())
	}

	return nil
}

// Delete deletes a budget
func (bf *BudgetFixture) Delete(id string) error {
	resp, err := bf.client.DELETE(fmt.Sprintf("/v1/budgets/%s", id))
	if err != nil {
		return fmt.Errorf("delete budget: %w", err)
	}

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return fmt.Errorf("delete budget failed: status %d, body: %s", resp.StatusCode, resp.String())
	}

	return nil
}

