// Package api provides the API client for the AI-as-a-Service platform.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is the API client for the AI-as-a-Service platform.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// ClientOption configures the client.
type ClientOption func(*Client)

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(c *http.Client) ClientOption {
	return func(client *Client) {
		client.httpClient = c
	}
}

// NewClient creates a new API client.
func NewClient(baseURL, apiKey string, opts ...ClientOption) *Client {
	c := &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// SetAPIKey updates the API key for authentication.
func (c *Client) SetAPIKey(key string) {
	c.apiKey = key
}

// RedeemBootstrapKeyRequest is the request to redeem a bootstrap key.
type RedeemBootstrapKeyRequest struct {
	BootstrapKey string `json:"bootstrap_key"`
}

// RedeemBootstrapKeyResponse is the response from redeeming a bootstrap key.
type RedeemBootstrapKeyResponse struct {
	APIKey      string `json:"api_key"`
	OrgID       string `json:"org_id"`
	OrgName     string `json:"org_name"`
	AdminUserID string `json:"admin_user_id"`
	AdminEmail  string `json:"admin_email"`
	ExpiresAt   string `json:"expires_at,omitempty"`
}

// RedeemBootstrapKey redeems a bootstrap key and returns credentials.
func (c *Client) RedeemBootstrapKey(ctx context.Context, key string) (*RedeemBootstrapKeyResponse, error) {
	req := RedeemBootstrapKeyRequest{BootstrapKey: key}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/bootstrap-keys/redeem", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		switch resp.StatusCode {
		case http.StatusNotFound:
			return nil, fmt.Errorf("bootstrap key not found or already used")
		case http.StatusGone:
			return nil, fmt.Errorf("bootstrap key has expired")
		case http.StatusBadRequest:
			return nil, fmt.Errorf("invalid bootstrap key format")
		default:
			return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
		}
	}

	var result RedeemBootstrapKeyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// APIError represents an error response from the API.
type APIError struct {
	StatusCode int
	Message    string
	Details    string
}

func (e *APIError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s", e.Message, e.Details)
	}
	return e.Message
}

// doRequest performs an authenticated request.
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("API request failed with status %d", resp.StatusCode),
			Details:    string(bodyBytes),
		}
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}
