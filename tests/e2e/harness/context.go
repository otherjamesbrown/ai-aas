package harness

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Context holds test execution context
type Context struct {
	RunID       string
	Environment string
	Config      *Config
	Client      *Client
	Fixtures    *FixtureManager
	Artifacts   *ArtifactCollector
	WorkerID    string
	mu          sync.RWMutex
}

// NewContext creates a new test context
func NewContext(config *Config) (*Context, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	runID := os.Getenv("TEST_RUN_ID")
	if runID == "" {
		runID = uuid.New().String()
	}

	workerID := os.Getenv("TEST_WORKER_ID")
	if workerID == "" {
		workerID = "worker-0"
	}

	client := NewClient(config.APIURLs.UserOrgService, config.Timeouts.RequestTimeout)
	if config.Credentials.AdminAPIKey != "" {
		client.SetHeader("Authorization", "Bearer "+config.Credentials.AdminAPIKey)
		client.SetHeader("X-API-Key", config.Credentials.AdminAPIKey)
	}
	
	// If using IP address, set Host header for ingress routing
	// Check if URL is an IP address (contains only digits and dots)
	if isIPAddress(config.APIURLs.UserOrgService) {
		client.SetHeader("Host", "api.dev.ai-aas.local")
	}

	ctx := &Context{
		RunID:       runID,
		Environment: config.Environment,
		Config:      config,
		Client:      client,
		Fixtures:    NewFixtureManager(runID, workerID),
		Artifacts:   NewArtifactCollector(config.Artifacts.OutputDir, runID),
		WorkerID:    workerID,
	}

	return ctx, nil
}

// Namespace returns a unique namespace for this test run
func (c *Context) Namespace() string {
	return fmt.Sprintf("e2e-%s-%s", c.RunID[:8], c.WorkerID)
}

// GenerateResourceName generates a unique resource name with namespace prefix
func (c *Context) GenerateResourceName(prefix string) string {
	timestamp := time.Now().Format("20060102-150405")
	return fmt.Sprintf("%s-%s-%s-%s", c.Namespace(), prefix, timestamp, uuid.New().String()[:8])
}

// Cleanup performs cleanup operations
func (c *Context) Cleanup() error {
	if !c.Config.Cleanup.Enabled {
		return nil
	}

	// Wait for cleanup delay
	if c.Config.Cleanup.DelaySeconds > 0 {
		time.Sleep(time.Duration(c.Config.Cleanup.DelaySeconds) * time.Second)
	}

	// Cleanup fixtures
	if err := c.Fixtures.Cleanup(); err != nil {
		return fmt.Errorf("cleanup fixtures: %w", err)
	}

	return nil
}

// isIPAddress checks if a URL string contains an IP address
func isIPAddress(urlStr string) bool {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return false
	}
	
	host := parsed.Hostname()
	if host == "" {
		return false
	}
	
	// Check if host is an IP address (IPv4)
	ipRegex := regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$`)
	if ipRegex.MatchString(host) {
		return true
	}
	
	// Check if host is an IP address with port
	if strings.Contains(host, ":") {
		host = strings.Split(host, ":")[0]
		return ipRegex.MatchString(host)
	}
	
	return false
}

