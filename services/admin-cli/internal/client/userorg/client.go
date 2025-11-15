// Package userorg provides the API client for user-org-service.
//
// Purpose:
//
//	REST client implementation for consuming user-org-service APIs. Handles
//	authentication, request/response formatting, and error handling with retry logic.
//
// Dependencies:
//   - net/http: HTTP client
//   - internal/client/retry: Retry logic with exponential backoff
//   - internal/client/userorg/types: Request/response types
//
// Requirements Reference:
//   - specs/009-admin-cli/spec.md#FR-008 (consume existing service APIs)
//   - specs/009-admin-cli/plan.md#client/userorg
//
package userorg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/client"
)

// Client provides access to user-org-service APIs.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	retryCfg   client.RetryConfig
}

// NewClient creates a new user-org-service API client.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		retryCfg:   client.DefaultRetryConfig(),
	}
}

// Bootstrap creates the first admin account (bootstrap operation).
// This creates an org, invites a user, and generates an API key for that user.
func (c *Client) Bootstrap(ctx context.Context, req BootstrapRequest) (*BootstrapResponse, error) {
	// Step 1: Create organization
	orgName := req.OrgName
	if orgName == "" {
		orgName = "Platform Admin Organization"
	}
	orgSlug := req.OrgSlug
	if orgSlug == "" {
		orgSlug = "platform-admin"
	}

	orgReq := CreateOrgRequest{
		Name: orgName,
		Slug: orgSlug,
		Metadata: map[string]interface{}{
			"bootstrap": true,
		},
	}

	org, err := c.CreateOrg(ctx, orgReq)
	if err != nil {
		return nil, fmt.Errorf("create organization: %w", err)
	}

	// Step 2: Invite user (creates user with invited status)
	inviteReq := InviteUserRequest{
		Email:          req.Email,
		ExpiresInHours: 72, // 3 days default
	}

	inviteResp, err := c.InviteUser(ctx, org.OrgID, inviteReq)
	if err != nil {
		return nil, fmt.Errorf("invite user: %w", err)
	}

	// Step 3: Create API key for the user
	apiKeyReq := IssueAPIKeyRequest{
		Scopes: []string{"admin"}, // Admin scope for bootstrap user
	}

	apiKeyResp, err := c.IssueUserAPIKey(ctx, org.OrgID, inviteResp.UserID, apiKeyReq)
	if err != nil {
		return nil, fmt.Errorf("create API key: %w", err)
	}

	return &BootstrapResponse{
		AdminID: inviteResp.UserID,
		OrgID:   org.OrgID,
		APIKey:  apiKeyResp.Secret,
		Email:   req.Email,
		OrgName: org.Name,
	}, nil
}

// CheckExistingAdmin checks if an admin account already exists by checking if any orgs exist.
// If orgs exist, we assume an admin has been created.
func (c *Client) CheckExistingAdmin(ctx context.Context) (bool, error) {
	orgs, err := c.ListOrgs(ctx)
	if err != nil {
		return false, fmt.Errorf("check existing admin: %w", err)
	}

	return len(orgs) > 0, nil
}

// CreateOrg creates a new organization.
func (c *Client) CreateOrg(ctx context.Context, req CreateOrgRequest) (*OrganizationResponse, error) {
	url := fmt.Sprintf("%s/v1/orgs", c.baseURL)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}

	resp, err := client.DoWithRetry(ctx, c.httpClient, httpReq, c.retryCfg)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes := make([]byte, 1024)
		n, _ := resp.Body.Read(bodyBytes)
		return nil, fmt.Errorf("create org failed: status %d, body: %s", resp.StatusCode, string(bodyBytes[:n]))
	}

	var result OrganizationResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// ListOrgs lists organizations.
func (c *Client) ListOrgs(ctx context.Context) ([]OrganizationResponse, error) {
	url := fmt.Sprintf("%s/v1/orgs", c.baseURL)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}

	resp, err := client.DoWithRetry(ctx, c.httpClient, httpReq, c.retryCfg)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list orgs failed: status %d", resp.StatusCode)
	}

	var result []OrganizationResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result, nil
}

// InviteUser invites a user to an organization.
func (c *Client) InviteUser(ctx context.Context, orgID string, req InviteUserRequest) (*UserResponse, error) {
	url := fmt.Sprintf("%s/v1/orgs/%s/invites", c.baseURL, orgID)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}

	resp, err := client.DoWithRetry(ctx, c.httpClient, httpReq, c.retryCfg)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusCreated {
		bodyBytes := make([]byte, 1024)
		n, _ := resp.Body.Read(bodyBytes)
		return nil, fmt.Errorf("invite user failed: status %d, body: %s", resp.StatusCode, string(bodyBytes[:n]))
	}

	// Parse invite response (InviteID is actually the user ID)
	var inviteResp InviteResponse
	if err := json.NewDecoder(resp.Body).Decode(&inviteResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// The InviteID in the response is actually the user ID
	// Get user details to return full user response
	user, err := c.GetUser(ctx, orgID, inviteResp.InviteID)
	if err != nil {
		// If we can't get user yet, return partial response from invite
		// This can happen if the user was just created and isn't immediately queryable
		return &UserResponse{
			UserID:  inviteResp.InviteID,
			Email:   req.Email,
			Status:  "invited",
			OrgID:   orgID,
		}, nil
	}

	return user, nil
}

// GetUser gets a user by ID in an organization.
func (c *Client) GetUser(ctx context.Context, orgID, userID string) (*UserResponse, error) {
	url := fmt.Sprintf("%s/v1/orgs/%s/users/%s", c.baseURL, orgID, userID)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}

	resp, err := client.DoWithRetry(ctx, c.httpClient, httpReq, c.retryCfg)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get user failed: status %d", resp.StatusCode)
	}

	var result UserResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// GetUserByEmail gets a user by email in an organization.
func (c *Client) GetUserByEmail(ctx context.Context, orgID, email string) (*UserResponse, error) {
	// List users and find by email
	users, err := c.ListUsers(ctx, orgID)
	if err != nil {
		return nil, err
	}

	for _, user := range users {
		if user.Email == email {
			return &user, nil
		}
	}

	return nil, fmt.Errorf("user not found")
}

// ListUsers lists users in an organization.
func (c *Client) ListUsers(ctx context.Context, orgID string) ([]UserResponse, error) {
	url := fmt.Sprintf("%s/v1/orgs/%s/users", c.baseURL, orgID)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}

	resp, err := client.DoWithRetry(ctx, c.httpClient, httpReq, c.retryCfg)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list users failed: status %d", resp.StatusCode)
	}

	var result []UserResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result, nil
}

// IssueUserAPIKey creates an API key for a user.
func (c *Client) IssueUserAPIKey(ctx context.Context, orgID, userID string, req IssueAPIKeyRequest) (*IssuedAPIKeyResponse, error) {
	url := fmt.Sprintf("%s/v1/orgs/%s/users/%s/api-keys", c.baseURL, orgID, userID)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}

	resp, err := client.DoWithRetry(ctx, c.httpClient, httpReq, c.retryCfg)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("issue API key failed: status %d", resp.StatusCode)
	}

	var result IssuedAPIKeyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// RotateAPIKey rotates an API key.
func (c *Client) RotateAPIKey(ctx context.Context, orgID, apiKeyID string) (*RotateAPIKeyResponse, error) {
	url := fmt.Sprintf("%s/v1/orgs/%s/api-keys/%s/rotate", c.baseURL, orgID, apiKeyID)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}

	resp, err := client.DoWithRetry(ctx, c.httpClient, httpReq, c.retryCfg)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes := make([]byte, 1024)
		n, _ := resp.Body.Read(bodyBytes)
		return nil, fmt.Errorf("rotate API key failed: status %d, body: %s", resp.StatusCode, string(bodyBytes[:n]))
	}

	var result IssuedAPIKeyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Convert IssuedAPIKeyResponse to RotateAPIKeyResponse (same structure)
	return &RotateAPIKeyResponse{
		APIKeyID:    result.APIKeyID,
		Secret:      result.Secret,
		Fingerprint: result.Fingerprint,
		Status:      result.Status,
		ExpiresAt:   result.ExpiresAt,
	}, nil
}

// ListAPIKeys lists API keys for an organization.
func (c *Client) ListAPIKeys(ctx context.Context, orgID string) ([]APIKeyResponse, error) {
	url := fmt.Sprintf("%s/v1/orgs/%s/api-keys", c.baseURL, orgID)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}

	resp, err := client.DoWithRetry(ctx, c.httpClient, httpReq, c.retryCfg)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list API keys failed: status %d", resp.StatusCode)
	}

	var result []APIKeyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result, nil
}

// GetOrg gets an organization by ID or slug.
func (c *Client) GetOrg(ctx context.Context, orgID string) (*OrganizationResponse, error) {
	url := fmt.Sprintf("%s/v1/orgs/%s", c.baseURL, orgID)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}

	resp, err := client.DoWithRetry(ctx, c.httpClient, httpReq, c.retryCfg)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes := make([]byte, 1024)
		n, _ := resp.Body.Read(bodyBytes)
		return nil, fmt.Errorf("get org failed: status %d, body: %s", resp.StatusCode, string(bodyBytes[:n]))
	}

	var result OrganizationResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// UpdateOrg updates an organization.
func (c *Client) UpdateOrg(ctx context.Context, orgID string, req UpdateOrgRequest) (*OrganizationResponse, error) {
	url := fmt.Sprintf("%s/v1/orgs/%s", c.baseURL, orgID)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "PATCH", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}

	resp, err := client.DoWithRetry(ctx, c.httpClient, httpReq, c.retryCfg)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes := make([]byte, 1024)
		n, _ := resp.Body.Read(bodyBytes)
		return nil, fmt.Errorf("update org failed: status %d, body: %s", resp.StatusCode, string(bodyBytes[:n]))
	}

	var result OrganizationResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// DeleteOrg deletes an organization (soft delete).
func (c *Client) DeleteOrg(ctx context.Context, orgID string) error {
	url := fmt.Sprintf("%s/v1/orgs/%s", c.baseURL, orgID)

	httpReq, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}

	resp, err := client.DoWithRetry(ctx, c.httpClient, httpReq, c.retryCfg)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		bodyBytes := make([]byte, 1024)
		n, _ := resp.Body.Read(bodyBytes)
		return fmt.Errorf("delete org failed: status %d, body: %s", resp.StatusCode, string(bodyBytes[:n]))
	}

	return nil
}

// UpdateUser updates a user.
func (c *Client) UpdateUser(ctx context.Context, orgID, userID string, req UpdateUserRequest) (*UserResponse, error) {
	url := fmt.Sprintf("%s/v1/orgs/%s/users/%s", c.baseURL, orgID, userID)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "PATCH", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}

	resp, err := client.DoWithRetry(ctx, c.httpClient, httpReq, c.retryCfg)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes := make([]byte, 1024)
		n, _ := resp.Body.Read(bodyBytes)
		return nil, fmt.Errorf("update user failed: status %d, body: %s", resp.StatusCode, string(bodyBytes[:n]))
	}

	var result UserResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// DeleteUser deletes a user (soft delete).
func (c *Client) DeleteUser(ctx context.Context, orgID, userID string) error {
	url := fmt.Sprintf("%s/v1/orgs/%s/users/%s", c.baseURL, orgID, userID)

	httpReq, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}

	resp, err := client.DoWithRetry(ctx, c.httpClient, httpReq, c.retryCfg)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		bodyBytes := make([]byte, 1024)
		n, _ := resp.Body.Read(bodyBytes)
		return fmt.Errorf("delete user failed: status %d, body: %s", resp.StatusCode, string(bodyBytes[:n]))
	}

	return nil
}

// DeleteAPIKey deletes an API key.
func (c *Client) DeleteAPIKey(ctx context.Context, orgID, apiKeyID string) error {
	url := fmt.Sprintf("%s/v1/orgs/%s/api-keys/%s", c.baseURL, orgID, apiKeyID)

	httpReq, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}

	resp, err := client.DoWithRetry(ctx, c.httpClient, httpReq, c.retryCfg)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		bodyBytes := make([]byte, 1024)
		n, _ := resp.Body.Read(bodyBytes)
		return fmt.Errorf("delete API key failed: status %d, body: %s", resp.StatusCode, string(bodyBytes[:n]))
	}

	return nil
}

