package fixtures

import (
	"fmt"
	"time"

	"github.com/ai-aas/tests/e2e/harness"
)

// TokenRateLimitPolicy represents a test token rate-limit policy fixture
type TokenRateLimitPolicy struct {
	ID          string  `json:"id"`
	OrgID       *string `json:"org_id,omitempty"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	Limit1h     *int64  `json:"limit_1h,omitempty"`
	Limit24h    *int64  `json:"limit_24h,omitempty"`
	Limit7d     *int64  `json:"limit_7d,omitempty"`
	IsBuiltin   bool    `json:"is_builtin"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// EffectiveTokenPolicy represents a user's effective token policy
type EffectiveTokenPolicy struct {
	Policy TokenRateLimitPolicy `json:"policy"`
	Source string               `json:"source"` // "override" or "inherited"
}

// TokenUsage represents token usage for a user
type TokenUsage struct {
	UserID     string             `json:"user_id,omitempty"`
	OrgID      string             `json:"org_id,omitempty"`
	PolicyID   string             `json:"policy_id,omitempty"`
	PolicyName string             `json:"policy_name"`
	Windows    []TokenUsageWindow `json:"windows"`
}

// TokenUsageWindow represents a usage window
type TokenUsageWindow struct {
	Window     string  `json:"window"` // "1h", "24h", "7d"
	Limit      int64   `json:"limit"`
	Used       int64   `json:"used"`
	Remaining  int64   `json:"remaining"`
	Percentage float64 `json:"percentage"`
	ResetsAt   string  `json:"resets_at"`
}

// TokenPolicyFixture provides methods for creating and managing token policy fixtures
type TokenPolicyFixture struct {
	client *harness.Client
	fm     *harness.FixtureManager
}

// NewTokenPolicyFixture creates a new token policy fixture manager
func NewTokenPolicyFixture(client *harness.Client, fm *harness.FixtureManager) *TokenPolicyFixture {
	return &TokenPolicyFixture{
		client: client,
		fm:     fm,
	}
}

// Create creates a new token rate-limit policy for testing
func (tf *TokenPolicyFixture) Create(ctx *harness.Context, orgID string, name string, limit1h, limit24h, limit7d *int64) (*TokenRateLimitPolicy, error) {
	reqBody := map[string]interface{}{
		"name": name,
	}
	if limit1h != nil {
		reqBody["limit_1h"] = *limit1h
	}
	if limit24h != nil {
		reqBody["limit_24h"] = *limit24h
	}
	if limit7d != nil {
		reqBody["limit_7d"] = *limit7d
	}

	resp, err := tf.client.POST(fmt.Sprintf("/v1/orgs/%s/token-policies", orgID), reqBody)
	if err != nil {
		return nil, fmt.Errorf("create token policy: %w", err)
	}

	if resp.StatusCode != 201 {
		return nil, fmt.Errorf("create token policy failed: status %d, body: %s", resp.StatusCode, resp.String())
	}

	var policy TokenRateLimitPolicy
	if err := resp.UnmarshalJSON(&policy); err != nil {
		return nil, fmt.Errorf("unmarshal token policy: %w", err)
	}

	// Register for cleanup
	tf.fm.Register("token_policy", policy.ID, map[string]string{
		"organization_id": orgID,
		"test_run_id":     ctx.RunID,
	}, func() error {
		return tf.Delete(orgID, policy.ID)
	})

	return &policy, nil
}

// CreateWithLimits creates a token policy with specific window limits
func (tf *TokenPolicyFixture) CreateWithLimits(ctx *harness.Context, orgID string, name string, limits map[string]int64) (*TokenRateLimitPolicy, error) {
	var limit1h, limit24h, limit7d *int64
	if v, ok := limits["1h"]; ok {
		limit1h = &v
	}
	if v, ok := limits["24h"]; ok {
		limit24h = &v
	}
	if v, ok := limits["7d"]; ok {
		limit7d = &v
	}
	return tf.Create(ctx, orgID, name, limit1h, limit24h, limit7d)
}

// Get retrieves a token policy by ID
func (tf *TokenPolicyFixture) Get(orgID, policyID string) (*TokenRateLimitPolicy, error) {
	resp, err := tf.client.GET(fmt.Sprintf("/v1/orgs/%s/token-policies/%s", orgID, policyID))
	if err != nil {
		return nil, fmt.Errorf("get token policy: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("get token policy failed: status %d, body: %s", resp.StatusCode, resp.String())
	}

	var policy TokenRateLimitPolicy
	if err := resp.UnmarshalJSON(&policy); err != nil {
		return nil, fmt.Errorf("unmarshal token policy: %w", err)
	}

	return &policy, nil
}

// List retrieves all token policies for an org
func (tf *TokenPolicyFixture) List(orgID string) ([]TokenRateLimitPolicy, error) {
	resp, err := tf.client.GET(fmt.Sprintf("/v1/orgs/%s/token-policies", orgID))
	if err != nil {
		return nil, fmt.Errorf("list token policies: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("list token policies failed: status %d, body: %s", resp.StatusCode, resp.String())
	}

	var result struct {
		Policies []TokenRateLimitPolicy `json:"policies"`
	}
	if err := resp.UnmarshalJSON(&result); err != nil {
		return nil, fmt.Errorf("unmarshal token policies: %w", err)
	}

	return result.Policies, nil
}

// Update updates a token policy
func (tf *TokenPolicyFixture) Update(orgID, policyID string, name *string, limit1h, limit24h, limit7d *int64) (*TokenRateLimitPolicy, error) {
	reqBody := make(map[string]interface{})
	if name != nil {
		reqBody["name"] = *name
	}
	if limit1h != nil {
		reqBody["limit_1h"] = *limit1h
	}
	if limit24h != nil {
		reqBody["limit_24h"] = *limit24h
	}
	if limit7d != nil {
		reqBody["limit_7d"] = *limit7d
	}

	resp, err := tf.client.PUT(fmt.Sprintf("/v1/orgs/%s/token-policies/%s", orgID, policyID), reqBody)
	if err != nil {
		return nil, fmt.Errorf("update token policy: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("update token policy failed: status %d, body: %s", resp.StatusCode, resp.String())
	}

	var policy TokenRateLimitPolicy
	if err := resp.UnmarshalJSON(&policy); err != nil {
		return nil, fmt.Errorf("unmarshal token policy: %w", err)
	}

	return &policy, nil
}

// Delete deletes a token policy
func (tf *TokenPolicyFixture) Delete(orgID, policyID string) error {
	resp, err := tf.client.DELETE(fmt.Sprintf("/v1/orgs/%s/token-policies/%s", orgID, policyID))
	if err != nil {
		return fmt.Errorf("delete token policy: %w", err)
	}

	if resp.StatusCode != 204 && resp.StatusCode != 200 {
		return fmt.Errorf("delete token policy failed: status %d, body: %s", resp.StatusCode, resp.String())
	}

	return nil
}

// GetOrgDefault retrieves the org's default token policy
func (tf *TokenPolicyFixture) GetOrgDefault(orgID string) (*TokenRateLimitPolicy, error) {
	resp, err := tf.client.GET(fmt.Sprintf("/v1/orgs/%s/token-policy", orgID))
	if err != nil {
		return nil, fmt.Errorf("get org default token policy: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("get org default token policy failed: status %d, body: %s", resp.StatusCode, resp.String())
	}

	var policy TokenRateLimitPolicy
	if err := resp.UnmarshalJSON(&policy); err != nil {
		return nil, fmt.Errorf("unmarshal token policy: %w", err)
	}

	return &policy, nil
}

// SetOrgDefault sets the org's default token policy
func (tf *TokenPolicyFixture) SetOrgDefault(orgID, policyID string) (*TokenRateLimitPolicy, error) {
	reqBody := map[string]interface{}{
		"policy_id": policyID,
	}

	resp, err := tf.client.PUT(fmt.Sprintf("/v1/orgs/%s/token-policy", orgID), reqBody)
	if err != nil {
		return nil, fmt.Errorf("set org default token policy: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("set org default token policy failed: status %d, body: %s", resp.StatusCode, resp.String())
	}

	var policy TokenRateLimitPolicy
	if err := resp.UnmarshalJSON(&policy); err != nil {
		return nil, fmt.Errorf("unmarshal token policy: %w", err)
	}

	return &policy, nil
}

// GetUserPolicy retrieves a user's effective token policy
func (tf *TokenPolicyFixture) GetUserPolicy(orgID, userID string) (*EffectiveTokenPolicy, error) {
	resp, err := tf.client.GET(fmt.Sprintf("/v1/orgs/%s/users/%s/token-policy", orgID, userID))
	if err != nil {
		return nil, fmt.Errorf("get user token policy: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("get user token policy failed: status %d, body: %s", resp.StatusCode, resp.String())
	}

	var effective EffectiveTokenPolicy
	if err := resp.UnmarshalJSON(&effective); err != nil {
		return nil, fmt.Errorf("unmarshal effective token policy: %w", err)
	}

	return &effective, nil
}

// SetUserPolicy sets a user's token policy override
func (tf *TokenPolicyFixture) SetUserPolicy(orgID, userID, policyID string) (*EffectiveTokenPolicy, error) {
	reqBody := map[string]interface{}{
		"policy_id": policyID,
	}

	resp, err := tf.client.PUT(fmt.Sprintf("/v1/orgs/%s/users/%s/token-policy", orgID, userID), reqBody)
	if err != nil {
		return nil, fmt.Errorf("set user token policy: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("set user token policy failed: status %d, body: %s", resp.StatusCode, resp.String())
	}

	var effective EffectiveTokenPolicy
	if err := resp.UnmarshalJSON(&effective); err != nil {
		return nil, fmt.Errorf("unmarshal effective token policy: %w", err)
	}

	return &effective, nil
}

// ClearUserPolicy clears a user's token policy override (inherits org default)
func (tf *TokenPolicyFixture) ClearUserPolicy(orgID, userID string) (*EffectiveTokenPolicy, error) {
	resp, err := tf.client.DELETE(fmt.Sprintf("/v1/orgs/%s/users/%s/token-policy", orgID, userID))
	if err != nil {
		return nil, fmt.Errorf("clear user token policy: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("clear user token policy failed: status %d, body: %s", resp.StatusCode, resp.String())
	}

	var effective EffectiveTokenPolicy
	if err := resp.UnmarshalJSON(&effective); err != nil {
		return nil, fmt.Errorf("unmarshal effective token policy: %w", err)
	}

	return &effective, nil
}

// TokenUsageFixture provides methods for querying token usage
type TokenUsageFixture struct {
	client *harness.Client
}

// NewTokenUsageFixture creates a new token usage fixture manager
func NewTokenUsageFixture(client *harness.Client) *TokenUsageFixture {
	return &TokenUsageFixture{
		client: client,
	}
}

// GetOwnUsage gets the authenticated user's token usage
func (tf *TokenUsageFixture) GetOwnUsage() (*TokenUsage, error) {
	resp, err := tf.client.GET("/v1/usage/tokens")
	if err != nil {
		return nil, fmt.Errorf("get own token usage: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("get own token usage failed: status %d, body: %s", resp.StatusCode, resp.String())
	}

	var usage TokenUsage
	if err := resp.UnmarshalJSON(&usage); err != nil {
		return nil, fmt.Errorf("unmarshal token usage: %w", err)
	}

	return &usage, nil
}

// GetUserUsage gets a user's token usage (requires admin privileges)
func (tf *TokenUsageFixture) GetUserUsage(orgID, userID string) (*TokenUsage, error) {
	resp, err := tf.client.GET(fmt.Sprintf("/v1/orgs/%s/users/%s/usage/tokens", orgID, userID))
	if err != nil {
		return nil, fmt.Errorf("get user token usage: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("get user token usage failed: status %d, body: %s", resp.StatusCode, resp.String())
	}

	var usage TokenUsage
	if err := resp.UnmarshalJSON(&usage); err != nil {
		return nil, fmt.Errorf("unmarshal token usage: %w", err)
	}

	return &usage, nil
}

// WaitForUsageUpdate waits for usage to reflect a change (eventual consistency helper)
func (tf *TokenUsageFixture) WaitForUsageUpdate(orgID, userID string, expectedUsed int64, window string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		usage, err := tf.GetUserUsage(orgID, userID)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		for _, w := range usage.Windows {
			if w.Window == window && w.Used >= expectedUsed {
				return nil
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for usage update: expected at least %d tokens in %s window", expectedUsed, window)
}
