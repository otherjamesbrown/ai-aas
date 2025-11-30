// Model access client methods for user-org-service API.
//
// Purpose:
//   Client methods for user-level model access control API (Spec 022).
//
// Requirements Reference:
//   - specs/022-user-model-access-control/spec.md

package userorg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/client"
)

// ModelAccessResponse represents a user's model access configuration.
type ModelAccessResponse struct {
	UserID        string              `json:"userId"`
	OrgID         string              `json:"orgId"`
	AccessMode    string              `json:"accessMode"`
	GrantedModels []ModelGrantResponse `json:"grantedModels"`
}

// ModelGrantResponse represents a model grant.
type ModelGrantResponse struct {
	GrantID   string  `json:"grantId"`
	ModelName string  `json:"modelName"`
	GrantedAt string  `json:"grantedAt"`
	GrantedBy *string `json:"grantedBy,omitempty"`
	ExpiresAt *string `json:"expiresAt,omitempty"`
}

// SetAccessModeRequest represents request to set access mode.
type SetAccessModeRequest struct {
	AccessMode string `json:"accessMode"`
}

// GrantModelRequest represents request to grant model access.
type GrantModelRequest struct {
	ModelName string  `json:"modelName"`
	ExpiresAt *string `json:"expiresAt,omitempty"`
}

// BulkGrantRequest represents request to grant multiple models.
type BulkGrantRequest struct {
	Models []string `json:"models"`
}

// BulkGrantResponse represents response from granting multiple models.
type BulkGrantResponse struct {
	GrantedCount int      `json:"grantedCount"`
	Models       []string `json:"models"`
	GrantedAt    string   `json:"grantedAt"`
}

// ModelGrantsListResponse represents list of grants.
type ModelGrantsListResponse struct {
	Grants []ModelGrantResponse `json:"grants"`
}

// GetUserModelAccess retrieves model access configuration for a user.
func (c *Client) GetUserModelAccess(ctx context.Context, orgID, userID string) (*ModelAccessResponse, error) {
	url := fmt.Sprintf("%s/v1/orgs/%s/users/%s/model-access", c.baseURL, orgID, userID)

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
		return nil, fmt.Errorf("get model access failed: status %d, body: %s", resp.StatusCode, string(bodyBytes[:n]))
	}

	var result ModelAccessResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// SetUserAccessMode sets the access mode for a user.
func (c *Client) SetUserAccessMode(ctx context.Context, orgID, userID, mode string) (*ModelAccessResponse, error) {
	url := fmt.Sprintf("%s/v1/orgs/%s/users/%s/model-access/mode", c.baseURL, orgID, userID)

	reqBody := SetAccessModeRequest{AccessMode: mode}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(body))
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
		return nil, fmt.Errorf("set access mode failed: status %d, body: %s", resp.StatusCode, string(bodyBytes[:n]))
	}

	var result ModelAccessResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// ListUserModelGrants lists all model grants for a user.
func (c *Client) ListUserModelGrants(ctx context.Context, orgID, userID string) ([]ModelGrantResponse, error) {
	url := fmt.Sprintf("%s/v1/orgs/%s/users/%s/model-access/grants", c.baseURL, orgID, userID)

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
		return nil, fmt.Errorf("list grants failed: status %d, body: %s", resp.StatusCode, string(bodyBytes[:n]))
	}

	var result ModelGrantsListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result.Grants, nil
}

// GrantModelAccess grants access to a specific model for a user.
func (c *Client) GrantModelAccess(ctx context.Context, orgID, userID, modelName string, expiresAt *string) (*ModelGrantResponse, error) {
	url := fmt.Sprintf("%s/v1/orgs/%s/users/%s/model-access/grants", c.baseURL, orgID, userID)

	reqBody := GrantModelRequest{
		ModelName: modelName,
		ExpiresAt: expiresAt,
	}
	body, err := json.Marshal(reqBody)
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
		return nil, fmt.Errorf("grant model access failed: status %d, body: %s", resp.StatusCode, string(bodyBytes[:n]))
	}

	var result ModelGrantResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// RevokeModelAccess revokes access to a specific model for a user.
func (c *Client) RevokeModelAccess(ctx context.Context, orgID, userID, modelName string) error {
	url := fmt.Sprintf("%s/v1/orgs/%s/users/%s/model-access/grants/%s", c.baseURL, orgID, userID, modelName)

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

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		bodyBytes := make([]byte, 1024)
		n, _ := resp.Body.Read(bodyBytes)
		return fmt.Errorf("revoke model access failed: status %d, body: %s", resp.StatusCode, string(bodyBytes[:n]))
	}

	return nil
}

// GrantAllCurrentModels grants all provided models to a user.
func (c *Client) GrantAllCurrentModels(ctx context.Context, orgID, userID string, models []string) (*BulkGrantResponse, error) {
	url := fmt.Sprintf("%s/v1/orgs/%s/users/%s/model-access/grants/all-current", c.baseURL, orgID, userID)

	reqBody := BulkGrantRequest{Models: models}
	body, err := json.Marshal(reqBody)
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

	if resp.StatusCode != http.StatusOK {
		bodyBytes := make([]byte, 1024)
		n, _ := resp.Body.Read(bodyBytes)
		return nil, fmt.Errorf("grant all models failed: status %d, body: %s", resp.StatusCode, string(bodyBytes[:n]))
	}

	var result BulkGrantResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}
