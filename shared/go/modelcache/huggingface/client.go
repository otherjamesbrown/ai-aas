// Package huggingface provides a client for interacting with the HuggingFace Hub API.
package huggingface

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	// DefaultBaseURL is the default HuggingFace API URL
	DefaultBaseURL = "https://huggingface.co/api"
	// DefaultTimeout is the default HTTP client timeout
	DefaultTimeout = 30 * time.Second
)

// Client provides methods for interacting with the HuggingFace Hub API
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// ClientOption configures the client
type ClientOption func(*Client)

// WithToken sets the HuggingFace token for authentication
func WithToken(token string) ClientOption {
	return func(c *Client) {
		c.token = token
	}
}

// WithBaseURL sets a custom base URL
func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) {
		c.baseURL = baseURL
	}
}

// WithTimeout sets the HTTP client timeout
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = client
	}
}

// NewClient creates a new HuggingFace Hub client
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		baseURL: DefaultBaseURL,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// ModelInfo contains information about a HuggingFace model
type ModelInfo struct {
	ID           string    `json:"id"`           // Full model ID (org/name)
	ModelID      string    `json:"modelId"`      // Same as ID
	Author       string    `json:"author"`       // Organization/user
	SHA          string    `json:"sha"`          // Current commit SHA
	LastModified string    `json:"lastModified"` // ISO8601 timestamp
	Private      bool      `json:"private"`      // Is model private
	Gated        bool      `json:"gated"`        // Requires license acceptance
	Disabled     bool      `json:"disabled"`     // Is model disabled
	Downloads    int64     `json:"downloads"`    // Download count
	Likes        int64     `json:"likes"`        // Like count
	Tags         []string  `json:"tags"`         // Model tags
	PipelineTag  string    `json:"pipeline_tag"` // Pipeline type
	LibraryName  string    `json:"library_name"` // Library (transformers, etc.)
	CardData     CardData  `json:"cardData"`     // Model card data
	Siblings     []Sibling `json:"siblings"`     // File list
}

// CardData contains model card metadata
type CardData struct {
	License     interface{} `json:"license"`      // License string or array
	LicenseLink string      `json:"license_link"` // URL to license
	Language    interface{} `json:"language"`     // Language(s)
	Tags        []string    `json:"tags"`         // Tags
}

// Sibling represents a file in the model repository
type Sibling struct {
	RFilename string   `json:"rfilename"` // Relative filename
	Size      int64    `json:"size"`      // File size in bytes
	BlobID    string   `json:"blobId"`    // Blob identifier
	LFS       *LFSInfo `json:"lfs"`       // LFS info if applicable
}

// LFSInfo contains LFS-specific file information
type LFSInfo struct {
	Size        int64  `json:"size"`
	SHA256      string `json:"sha256"`
	PointerSize int64  `json:"pointerSize"`
}

// GetModel retrieves information about a model from the HuggingFace Hub
func (c *Client) GetModel(ctx context.Context, modelID string) (*ModelInfo, error) {
	// Don't escape modelID - the slash in "org/model" is intentional for the HuggingFace API path
	endpoint := fmt.Sprintf("%s/models/%s", c.baseURL, modelID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrModelNotFound
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrUnauthorized
	}

	if resp.StatusCode == http.StatusForbidden {
		return nil, ErrForbidden
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var info ModelInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &info, nil
}

// ModelExists checks if a model exists on the HuggingFace Hub
func (c *Client) ModelExists(ctx context.Context, modelID string) (bool, error) {
	_, err := c.GetModel(ctx, modelID)
	if err == ErrModelNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// GetModelRevision retrieves information about a specific revision
func (c *Client) GetModelRevision(ctx context.Context, modelID, revision string) (*ModelInfo, error) {
	// Don't escape modelID - the slash in "org/model" is intentional for the HuggingFace API path
	endpoint := fmt.Sprintf("%s/models/%s/revision/%s", c.baseURL, modelID, url.PathEscape(revision))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrRevisionNotFound
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var info ModelInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &info, nil
}

// WhoAmI returns information about the authenticated user
func (c *Client) WhoAmI(ctx context.Context) (*UserInfo, error) {
	if c.token == "" {
		return nil, ErrNoToken
	}

	endpoint := fmt.Sprintf("%s/whoami-v2", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrUnauthorized
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var info UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &info, nil
}

// UserInfo contains information about a HuggingFace user
type UserInfo struct {
	Type          string    `json:"type"`     // "user" or "org"
	ID            string    `json:"id"`       // User ID
	Name          string    `json:"name"`     // Username
	Fullname      string    `json:"fullname"` // Full name
	Email         string    `json:"email"`    // Email (if available)
	EmailVerified bool      `json:"emailVerified"`
	Plan          string    `json:"plan"` // Subscription plan
	CanPay        bool      `json:"canPay"`
	IsPro         bool      `json:"isPro"`
	Orgs          []OrgInfo `json:"orgs"` // Organizations
}

// OrgInfo contains organization information
type OrgInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"roleInOrg"`
}

// setHeaders sets common headers for API requests
func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}

// ValidateToken validates the HuggingFace token
func (c *Client) ValidateToken(ctx context.Context) (*UserInfo, error) {
	return c.WhoAmI(ctx)
}

// RepoFile represents a file in a HuggingFace repository
type RepoFile struct {
	Type string   `json:"type"` // "file" or "directory"
	Path string   `json:"path"` // Relative path in repo
	Size int64    `json:"size"` // File size in bytes
	OID  string   `json:"oid"`  // Git OID
	LFS  *LFSInfo `json:"lfs,omitempty"` // LFS info if applicable
}

// ListFiles returns the list of files in a model repository
func (c *Client) ListFiles(ctx context.Context, modelID, revision string) ([]RepoFile, error) {
	if revision == "" {
		revision = "main"
	}

	// Don't escape modelID - the slash in "org/model" is intentional for the HuggingFace API path
	endpoint := fmt.Sprintf("%s/models/%s/tree/%s", c.baseURL, modelID, url.PathEscape(revision))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrModelNotFound
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrUnauthorized
	}

	if resp.StatusCode == http.StatusForbidden {
		return nil, ErrForbidden
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var files []RepoFile
	if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return files, nil
}
