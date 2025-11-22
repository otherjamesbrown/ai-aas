package harness

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client wraps http.Client with test-specific functionality
type Client struct {
	httpClient *http.Client
	baseURL    string
	headers    map[string]string
	logger     Logger
	skipTLS    bool // Skip TLS verification for development
}

// Logger interface for logging requests and responses
type Logger interface {
	LogRequest(method, url string, headers map[string]string, body []byte)
	LogResponse(statusCode int, headers map[string]string, body []byte, duration time.Duration)
	LogError(err error, context string)
}

// NewClient creates a new test HTTP client
func NewClient(baseURL string, timeout time.Duration) *Client {
	// Create HTTP client with TLS config that skips verification for development
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // Skip TLS verification for development
		},
	}
	
	return &Client{
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: tr,
		},
		baseURL: baseURL,
		headers: make(map[string]string),
		logger:  &defaultLogger{},
		skipTLS: true,
	}
}

// SetHeader sets a header for all subsequent requests
func (c *Client) SetHeader(key, value string) {
	c.headers[key] = value
}

// SetLogger sets a custom logger
func (c *Client) SetLogger(logger Logger) {
	c.logger = logger
}

// GET performs a GET request
func (c *Client) GET(path string) (*Response, error) {
	return c.Do("GET", path, nil)
}

// POST performs a POST request with JSON body
func (c *Client) POST(path string, body interface{}) (*Response, error) {
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
func (c *Client) PUT(path string, body interface{}) (*Response, error) {
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
func (c *Client) DELETE(path string) (*Response, error) {
	return c.Do("DELETE", path, nil)
}

// Do performs an HTTP request
func (c *Client) Do(method, path string, body []byte) (*Response, error) {
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
	
	// Handle Host header specially - set on request.Host, not as a header
	for k, v := range c.headers {
		if strings.ToLower(k) == "host" {
			req.Host = v
		} else {
			req.Header.Set(k, v)
		}
	}

	// Log request (with masked sensitive values)
	maskedHeaders := maskSensitiveHeaders(c.headers)
	c.logger.LogRequest(method, url, maskedHeaders, maskSensitiveBody(body))

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	duration := time.Since(start)

	if err != nil {
		c.logger.LogError(err, fmt.Sprintf("%s %s", method, url))
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	// Log response (convert http.Header to map[string]string)
	respHeadersMap := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			respHeadersMap[k] = v[0]
		}
	}
	maskedRespHeaders := maskSensitiveHeaders(respHeadersMap)
	c.logger.LogResponse(resp.StatusCode, maskedRespHeaders, maskSensitiveBody(respBody), duration)

	return &Response{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       respBody,
		Duration:   duration,
	}, nil
}

// Response represents an HTTP response
type Response struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
	Duration   time.Duration
}

// UnmarshalJSON unmarshals the response body as JSON
func (r *Response) UnmarshalJSON(v interface{}) error {
	return json.Unmarshal(r.Body, v)
}

// String returns the response body as a string
func (r *Response) String() string {
	return string(r.Body)
}

// maskSensitiveHeaders masks sensitive header values
func maskSensitiveHeaders(headers map[string]string) map[string]string {
	masked := make(map[string]string)
	for k, v := range headers {
		if isSensitiveHeader(k) {
			masked[k] = "***"
		} else {
			masked[k] = v
		}
	}
	return masked
}

// maskSensitiveBody masks sensitive values in request/response bodies
func maskSensitiveBody(body []byte) []byte {
	if len(body) == 0 {
		return body
	}

	// Try to parse as JSON and mask sensitive fields
	var jsonData map[string]interface{}
	if err := json.Unmarshal(body, &jsonData); err != nil {
		// Not JSON, return as-is
		return body
	}

	masked := maskSensitiveJSON(jsonData)
	maskedBytes, err := json.Marshal(masked)
	if err != nil {
		return body
	}

	return maskedBytes
}

// maskSensitiveJSON recursively masks sensitive fields in JSON
func maskSensitiveJSON(data map[string]interface{}) map[string]interface{} {
	masked := make(map[string]interface{})
	for k, v := range data {
		if isSensitiveField(k) {
			masked[k] = "***"
		} else if nested, ok := v.(map[string]interface{}); ok {
			masked[k] = maskSensitiveJSON(nested)
		} else {
			masked[k] = v
		}
	}
	return masked
}

// isSensitiveHeader checks if a header contains sensitive information
func isSensitiveHeader(key string) bool {
	sensitive := []string{"authorization", "x-api-key", "cookie", "set-cookie"}
	lower := strings.ToLower(key)
	for _, s := range sensitive {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

// isSensitiveField checks if a JSON field contains sensitive information
func isSensitiveField(key string) bool {
	sensitive := []string{"password", "token", "key", "secret", "api_key", "apikey", "authorization"}
	lower := strings.ToLower(key)
	for _, s := range sensitive {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

// defaultLogger provides basic logging
type defaultLogger struct{}

func (l *defaultLogger) LogRequest(method, url string, headers map[string]string, body []byte) {
	// Default: no-op (can be overridden)
}

func (l *defaultLogger) LogResponse(statusCode int, headers map[string]string, body []byte, duration time.Duration) {
	// Default: no-op (can be overridden)
}

func (l *defaultLogger) LogError(err error, context string) {
	// Default: no-op (can be overridden)
}

