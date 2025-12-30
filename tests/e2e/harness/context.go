package harness

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Context holds test execution context
type Context struct {
	RunID       string
	Environment string
	Config      *Config
	Client      *Client      // User-org-service client (orgs, users, api-keys)
	AdminClient *Client      // Admin API client (engines, budgets, bootstrap-keys)
	RedisClient *redis.Client // Redis client for cleanup operations
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

	// Create user-org-service client
	client := NewClient(config.APIURLs.UserOrgService, config.Timeouts.RequestTimeout)
	if config.Credentials.AdminAPIKey != "" {
		client.SetHeader("Authorization", "Bearer "+config.Credentials.AdminAPIKey)
		client.SetHeader("X-API-Key", config.Credentials.AdminAPIKey)
	}

	// If using IP address, set Host header for ingress routing
	if isIPAddress(config.APIURLs.UserOrgService) {
		client.SetHeader("Host", "user-org.dev.otherjamesbrown.com")
	}

	// Create admin-api-service client
	adminClient := NewClient(config.APIURLs.AdminAPIService, config.Timeouts.RequestTimeout)
	if config.Credentials.AdminAPIKey != "" {
		adminClient.SetHeader("Authorization", "Bearer "+config.Credentials.AdminAPIKey)
		adminClient.SetHeader("X-API-Key", config.Credentials.AdminAPIKey)
	}

	// If using IP address, set Host header for ingress routing
	if isIPAddress(config.APIURLs.AdminAPIService) {
		adminClient.SetHeader("Host", "admin-api.dev.otherjamesbrown.com")
	}

	// Create Redis client for cleanup operations
	redisClient := redis.NewClient(&redis.Options{
		Addr: config.Redis.Addr,
		DB:   config.Redis.DB,
	})

	// Test Redis connection (non-blocking - log if unavailable but don't fail)
	testCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := redisClient.Ping(testCtx).Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Redis not available at %s (token cleanup will be skipped): %v\n", config.Redis.Addr, err)
	}

	ctx := &Context{
		RunID:       runID,
		Environment: config.Environment,
		Config:      config,
		Client:      client,
		AdminClient: adminClient,
		RedisClient: redisClient,
		Fixtures:    NewFixtureManager(runID, workerID),
		Artifacts:   NewArtifactCollector(config.Artifacts.OutputDir, runID),
		WorkerID:    workerID,
	}

	// Clean up Redis token counters before tests start
	if err := ctx.CleanupRedisTokenCounters(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to cleanup Redis token counters: %v\n", err)
		// Don't fail - tests can still run, they just might hit quota limits
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

// CleanupRedisTokenCounters removes all rate limit token counters from Redis
// This ensures tests start with fresh quota state
func (c *Context) CleanupRedisTokenCounters() error {
	if c.RedisClient == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Find all token counter keys
	keys, err := c.RedisClient.Keys(ctx, "ratelimit:tokens:*").Result()
	if err != nil {
		return fmt.Errorf("failed to list token keys: %w", err)
	}

	if len(keys) == 0 {
		return nil // No keys to delete
	}

	// Delete all token counter keys
	if err := c.RedisClient.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("failed to delete %d token keys: %w", len(keys), err)
	}

	fmt.Fprintf(os.Stderr, "Cleaned up %d Redis token counter keys\n", len(keys))
	return nil
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

	// Close Redis connection
	if c.RedisClient != nil {
		if err := c.RedisClient.Close(); err != nil {
			return fmt.Errorf("close redis client: %w", err)
		}
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

