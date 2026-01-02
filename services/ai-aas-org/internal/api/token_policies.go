// Package api provides the API client for the AI-as-a-Service platform.
package api

import (
	"context"
	"fmt"
	"net/url"
)

// TokenRateLimitPolicy represents a token rate-limit policy.
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

// CreateTokenPolicyRequest represents a request to create a token policy.
type CreateTokenPolicyRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	Limit1h     *int64  `json:"limit_1h,omitempty"`
	Limit24h    *int64  `json:"limit_24h,omitempty"`
	Limit7d     *int64  `json:"limit_7d,omitempty"`
}

// UpdateTokenPolicyRequest represents a request to update a token policy.
type UpdateTokenPolicyRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Limit1h     *int64  `json:"limit_1h,omitempty"`
	Limit24h    *int64  `json:"limit_24h,omitempty"`
	Limit7d     *int64  `json:"limit_7d,omitempty"`
}

// SetTokenPolicyRequest represents a request to set a policy.
type SetTokenPolicyRequest struct {
	PolicyID string `json:"policy_id"`
}

// EffectiveTokenPolicy represents a user's effective token policy.
type EffectiveTokenPolicy struct {
	Policy TokenRateLimitPolicy `json:"policy"`
	Source string               `json:"source"` // "override" or "inherited"
}

// TokenUsage represents token usage.
type TokenUsage struct {
	UserID     string             `json:"user_id,omitempty"`
	OrgID      string             `json:"org_id,omitempty"`
	PolicyID   string             `json:"policy_id,omitempty"`
	PolicyName string             `json:"policy_name"`
	Windows    []TokenUsageWindow `json:"windows"`
}

// TokenUsageWindow represents a usage window.
type TokenUsageWindow struct {
	Window     string  `json:"window"` // "1h", "24h", "7d"
	Limit      int64   `json:"limit"`
	Used       int64   `json:"used"`
	Remaining  int64   `json:"remaining"`
	Percentage float64 `json:"percentage"`
	ResetsAt   string  `json:"resets_at"`
}

// tokenPolicyListResponse wraps the list response.
type tokenPolicyListResponse struct {
	Policies []TokenRateLimitPolicy `json:"policies"`
}

// CreateTokenPolicy creates a new token rate-limit policy.
func (c *Client) CreateTokenPolicy(ctx context.Context, orgID string, req *CreateTokenPolicyRequest) (*TokenRateLimitPolicy, error) {
	path := fmt.Sprintf("/v1/orgs/%s/token-policies", url.PathEscape(orgID))
	var result TokenRateLimitPolicy
	if err := c.doRequest(ctx, "POST", path, req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListTokenPolicies lists token rate-limit policies for an organization.
func (c *Client) ListTokenPolicies(ctx context.Context, orgID string) ([]TokenRateLimitPolicy, error) {
	path := fmt.Sprintf("/v1/orgs/%s/token-policies", url.PathEscape(orgID))
	var result tokenPolicyListResponse
	if err := c.doRequest(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return result.Policies, nil
}

// GetTokenPolicy gets a token rate-limit policy by ID or name.
func (c *Client) GetTokenPolicy(ctx context.Context, orgID, policyID string) (*TokenRateLimitPolicy, error) {
	path := fmt.Sprintf("/v1/orgs/%s/token-policies/%s", url.PathEscape(orgID), url.PathEscape(policyID))
	var result TokenRateLimitPolicy
	if err := c.doRequest(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateTokenPolicy updates a token rate-limit policy.
func (c *Client) UpdateTokenPolicy(ctx context.Context, orgID, policyID string, req *UpdateTokenPolicyRequest) (*TokenRateLimitPolicy, error) {
	path := fmt.Sprintf("/v1/orgs/%s/token-policies/%s", url.PathEscape(orgID), url.PathEscape(policyID))
	var result TokenRateLimitPolicy
	if err := c.doRequest(ctx, "PUT", path, req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteTokenPolicy deletes a token rate-limit policy.
func (c *Client) DeleteTokenPolicy(ctx context.Context, orgID, policyID string) error {
	path := fmt.Sprintf("/v1/orgs/%s/token-policies/%s", url.PathEscape(orgID), url.PathEscape(policyID))
	return c.doRequest(ctx, "DELETE", path, nil, nil)
}

// GetOrgDefaultTokenPolicy gets the default token policy for an organization.
func (c *Client) GetOrgDefaultTokenPolicy(ctx context.Context, orgID string) (*TokenRateLimitPolicy, error) {
	path := fmt.Sprintf("/v1/orgs/%s/token-policy", url.PathEscape(orgID))
	var result TokenRateLimitPolicy
	if err := c.doRequest(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SetOrgDefaultTokenPolicy sets the default token policy for an organization.
func (c *Client) SetOrgDefaultTokenPolicy(ctx context.Context, orgID, policyID string) (*TokenRateLimitPolicy, error) {
	path := fmt.Sprintf("/v1/orgs/%s/token-policy", url.PathEscape(orgID))
	req := SetTokenPolicyRequest{PolicyID: policyID}
	var result TokenRateLimitPolicy
	if err := c.doRequest(ctx, "PUT", path, &req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetUserTokenPolicy gets the effective token policy for a user.
func (c *Client) GetUserTokenPolicy(ctx context.Context, orgID, userID string) (*EffectiveTokenPolicy, error) {
	path := fmt.Sprintf("/v1/orgs/%s/users/%s/token-policy", url.PathEscape(orgID), url.PathEscape(userID))
	var result EffectiveTokenPolicy
	if err := c.doRequest(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SetUserTokenPolicy sets a token policy override for a user.
func (c *Client) SetUserTokenPolicy(ctx context.Context, orgID, userID, policyID string) (*EffectiveTokenPolicy, error) {
	path := fmt.Sprintf("/v1/orgs/%s/users/%s/token-policy", url.PathEscape(orgID), url.PathEscape(userID))
	req := SetTokenPolicyRequest{PolicyID: policyID}
	var result EffectiveTokenPolicy
	if err := c.doRequest(ctx, "PUT", path, &req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ClearUserTokenPolicy clears a user's token policy override.
func (c *Client) ClearUserTokenPolicy(ctx context.Context, orgID, userID string) (*EffectiveTokenPolicy, error) {
	path := fmt.Sprintf("/v1/orgs/%s/users/%s/token-policy", url.PathEscape(orgID), url.PathEscape(userID))
	var result EffectiveTokenPolicy
	if err := c.doRequest(ctx, "DELETE", path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetUserTokenUsage gets token usage for a specific user.
func (c *Client) GetUserTokenUsage(ctx context.Context, orgID, userID string) (*TokenUsage, error) {
	path := fmt.Sprintf("/v1/orgs/%s/users/%s/usage/tokens", url.PathEscape(orgID), url.PathEscape(userID))
	var result TokenUsage
	if err := c.doRequest(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
