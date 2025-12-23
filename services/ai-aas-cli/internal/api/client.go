// Package api provides the HTTP client for communicating with the Admin API.
package api

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client provides methods for calling the Admin API
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// ClientOption configures the client
type ClientOption func(*Client)

// WithTimeout sets the HTTP client timeout
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithInsecureSkipVerify disables TLS certificate verification
func WithInsecureSkipVerify() ClientOption {
	return func(c *Client) {
		// Get the existing transport or use DefaultTransport
		customTransport, ok := c.httpClient.Transport.(*http.Transport)
		if !ok || customTransport == nil {
			customTransport = http.DefaultTransport.(*http.Transport)
		}

		// Clone the transport to avoid modifying a shared instance
		// This preserves connection pooling, HTTP/2, and proxy settings
		transport := customTransport.Clone()
		if transport.TLSClientConfig == nil {
			transport.TLSClientConfig = &tls.Config{}
		}
		transport.TLSClientConfig.InsecureSkipVerify = true //nolint:gosec // User-requested for dev environments
		c.httpClient.Transport = transport
	}
}

// NewClient creates a new Admin API client
func NewClient(baseURL, apiKey string, opts ...ClientOption) *Client {
	c := &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 90 * time.Second, // Increased from 30s for GPU inference validation
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Request performs an HTTP request to the Admin API
func (c *Client) Request(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	// Build URL - parse path to handle query strings correctly
	pathURL, err := url.Parse(path)
	if err != nil {
		return fmt.Errorf("parse path: %w", err)
	}

	baseURL, err := url.Parse(c.baseURL)
	if err != nil {
		return fmt.Errorf("parse base URL: %w", err)
	}

	// Resolve the path against the base URL
	u := baseURL.ResolveReference(pathURL)

	// Prepare body
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, u.String(), bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	// Check for errors
	if resp.StatusCode >= 400 {
		var apiErr APIError
		if err := json.Unmarshal(respBody, &apiErr); err != nil {
			return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
		}
		return &apiErr
	}

	// Decode result if provided
	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}

// Get performs a GET request
func (c *Client) Get(ctx context.Context, path string, result interface{}) error {
	return c.Request(ctx, http.MethodGet, path, nil, result)
}

// Post performs a POST request
func (c *Client) Post(ctx context.Context, path string, body interface{}, result interface{}) error {
	return c.Request(ctx, http.MethodPost, path, body, result)
}

// Put performs a PUT request
func (c *Client) Put(ctx context.Context, path string, body interface{}, result interface{}) error {
	return c.Request(ctx, http.MethodPut, path, body, result)
}

// Delete performs a DELETE request
func (c *Client) Delete(ctx context.Context, path string) error {
	return c.Request(ctx, http.MethodDelete, path, nil, nil)
}

// APIError represents an error response from the API
type APIError struct {
	StatusCode int    `json:"-"`
	Message    string `json:"error"`
	Details    string `json:"details,omitempty"`
}

// Error implements the error interface
func (e *APIError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s", e.Message, e.Details)
	}
	return e.Message
}

// IsNotFound returns true if the error is a 404
func (e *APIError) IsNotFound() bool {
	return e.StatusCode == http.StatusNotFound
}

// IsUnauthorized returns true if the error is a 401
func (e *APIError) IsUnauthorized() bool {
	return e.StatusCode == http.StatusUnauthorized
}

// HealthCheck performs a health check against the API
func (c *Client) HealthCheck(ctx context.Context) error {
	return c.Get(ctx, "/healthz", nil)
}

// Ping tests connectivity to the API
func (c *Client) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return c.HealthCheck(ctx)
}

// NewHTTPClient creates a standalone HTTP client with optional TLS skip verification
func NewHTTPClient(insecureSkipVerify bool) *http.Client {
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	if insecureSkipVerify {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		if transport.TLSClientConfig == nil {
			transport.TLSClientConfig = &tls.Config{}
		}
		transport.TLSClientConfig.InsecureSkipVerify = true //nolint:gosec // User-requested for dev environments
		client.Transport = transport
	}

	return client
}

