package usecases_test

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// TestClient wraps http.Client for UC tests
type TestClient struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

// NewTestClient creates a new test HTTP client
func NewTestClient(baseURL, apiKey string) *TestClient {
	// Create HTTP client with TLS config that skips verification for development
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // Skip TLS verification for development
		},
	}

	return &TestClient{
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: tr,
		},
		baseURL: baseURL,
		apiKey:  apiKey,
	}
}

// NewTestClientFromEnv creates a test client from environment variables
func NewTestClientFromEnv() (*TestClient, error) {
	endpoint := os.Getenv(envAPIEndpoint)
	apiKey := os.Getenv(envAPIKey)

	if endpoint == "" {
		return nil, fmt.Errorf("AI_AAS_API_ENDPOINT not set")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("AI_AAS_API_KEY not set")
	}

	return NewTestClient(endpoint, apiKey), nil
}

// TestResponse represents an HTTP response for tests
type TestResponse struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
	Duration   time.Duration
}

// UnmarshalJSON unmarshals the response body as JSON
func (r *TestResponse) UnmarshalJSON(v interface{}) error {
	return json.Unmarshal(r.Body, v)
}

// String returns the response body as a string
func (r *TestResponse) String() string {
	return string(r.Body)
}

// GET performs a GET request
func (c *TestClient) GET(path string) (*TestResponse, error) {
	return c.Do("GET", path, nil)
}

// POST performs a POST request with JSON body
func (c *TestClient) POST(path string, body interface{}) (*TestResponse, error) {
	var bodyBytes []byte
	var err error

	if body != nil {
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
	}

	return c.Do("POST", path, bodyBytes)
}

// PUT performs a PUT request with JSON body
func (c *TestClient) PUT(path string, body interface{}) (*TestResponse, error) {
	var bodyBytes []byte
	var err error

	if body != nil {
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
	}

	return c.Do("PUT", path, bodyBytes)
}

// DELETE performs a DELETE request
func (c *TestClient) DELETE(path string) (*TestResponse, error) {
	return c.Do("DELETE", path, nil)
}

// PATCH performs a PATCH request with JSON body
func (c *TestClient) PATCH(path string, body interface{}) (*TestResponse, error) {
	var bodyBytes []byte
	var err error

	if body != nil {
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
	}

	return c.Do("PATCH", path, bodyBytes)
}

// Do performs an HTTP request
func (c *TestClient) Do(method, path string, body []byte) (*TestResponse, error) {
	url := c.baseURL + path

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("X-API-Key", c.apiKey)
	}

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	duration := time.Since(start)

	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	return &TestResponse{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       respBody,
		Duration:   duration,
	}, nil
}
