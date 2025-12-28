package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Organization represents an organization.
type Organization struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	Plan        string    `json:"plan"`
	UserCount   int       `json:"user_count"`
	APIKeyCount int       `json:"api_key_count"`
	ModelCount  int       `json:"model_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// OrgLimits represents organization usage limits.
type OrgLimits struct {
	MaxUsers        int   `json:"max_users"`
	MaxAPIKeys      int   `json:"max_api_keys"`
	MonthlyTokens   int64 `json:"monthly_tokens"`
	RateLimitRPM    int   `json:"rate_limit_rpm"`
	RateLimitTPM    int64 `json:"rate_limit_tpm"`
}

// OrgDetails is the full organization details response.
type OrgDetails struct {
	Organization Organization `json:"organization"`
	Limits       OrgLimits    `json:"limits"`
}

// GetOrganization gets the organization details.
func (c *Client) GetOrganization(ctx context.Context, orgID string) (*OrgDetails, error) {
	path := fmt.Sprintf("/v1/orgs/%s", url.PathEscape(orgID))

	var result OrgDetails
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
