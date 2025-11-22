package harness

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds test configuration
type Config struct {
	Environment string
	APIURLs     APIURLs
	Credentials Credentials
	Timeouts    Timeouts
	Retries     Retries
	Parallel    Parallel
	Cleanup     Cleanup
	Artifacts   Artifacts
}

// APIURLs contains service API URLs
type APIURLs struct {
	UserOrgService  string
	APIRouterService string
	AnalyticsService string
}

// Credentials contains test credentials
type Credentials struct {
	AdminAPIKey    string
	TestUserEmail  string
	TestUserPassword string
}

// Timeouts contains timeout configurations
type Timeouts struct {
	RequestTimeout time.Duration
	TestTimeout   time.Duration
	CleanupTimeout time.Duration
}

// Retries contains retry configurations
type Retries struct {
	MaxRetries      int
	RetryDelay      time.Duration
	BackoffMultiplier float64
}

// Parallel contains parallel execution configurations
type Parallel struct {
	Enabled bool
	Workers int
}

// Cleanup contains cleanup configurations
type Cleanup struct {
	Enabled     bool
	DelaySeconds int
}

// Artifacts contains artifact collection configurations
type Artifacts struct {
	Enabled            bool
	OutputDir          string
	IncludeRequestBody bool
	IncludeResponseBody bool
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	env := getEnv("TEST_ENV", "local")

	config := &Config{
		Environment: env,
		APIURLs: APIURLs{
			UserOrgService:   getEnv("USER_ORG_SERVICE_URL", "http://localhost:8081"),
			APIRouterService: getEnv("API_ROUTER_SERVICE_URL", "http://localhost:8082"),
			AnalyticsService: getEnv("ANALYTICS_SERVICE_URL", "http://localhost:8083"),
		},
		Credentials: Credentials{
			AdminAPIKey:     getEnv("ADMIN_API_KEY", ""),
			TestUserEmail:   getEnv("TEST_USER_EMAIL", ""),
			TestUserPassword: getEnv("TEST_USER_PASSWORD", ""),
		},
		Timeouts: Timeouts{
			RequestTimeout: getDurationEnv("REQUEST_TIMEOUT_MS", 30*time.Second),
			TestTimeout:   getDurationEnv("TEST_TIMEOUT_MS", 5*time.Minute),
			CleanupTimeout: getDurationEnv("CLEANUP_TIMEOUT_MS", 5*time.Minute),
		},
		Retries: Retries{
			MaxRetries:       getIntEnv("MAX_RETRIES", 3),
			RetryDelay:       getDurationEnv("RETRY_DELAY_MS", 1*time.Second),
			BackoffMultiplier: getFloatEnv("BACKOFF_MULTIPLIER", 2.0),
		},
		Parallel: Parallel{
			Enabled: getBoolEnv("PARALLEL_ENABLED", false),
			Workers: getIntEnv("PARALLEL_WORKERS", 1),
		},
		Cleanup: Cleanup{
			Enabled:     getBoolEnv("ENABLE_CLEANUP", true),
			DelaySeconds: getIntEnv("CLEANUP_DELAY_SECONDS", 60),
		},
		Artifacts: Artifacts{
			Enabled:            getBoolEnv("ARTIFACTS_ENABLED", true),
			OutputDir:          getEnv("ARTIFACTS_DIR", "./artifacts"),
			IncludeRequestBody: getBoolEnv("INCLUDE_REQUEST_BODIES", true),
			IncludeResponseBody: getBoolEnv("INCLUDE_RESPONSE_BODIES", true),
		},
	}

	return config, nil
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getIntEnv gets an integer environment variable with a default value
func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getFloatEnv gets a float environment variable with a default value
func getFloatEnv(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

// getBoolEnv gets a boolean environment variable with a default value
func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

// getDurationEnv gets a duration environment variable (in milliseconds) with a default value
func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return time.Duration(intValue) * time.Millisecond
		}
	}
	return defaultValue
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.APIURLs.UserOrgService == "" {
		return fmt.Errorf("USER_ORG_SERVICE_URL is required")
	}
	if c.APIURLs.APIRouterService == "" {
		return fmt.Errorf("API_ROUTER_SERVICE_URL is required")
	}
	if c.Parallel.Workers < 1 {
		return fmt.Errorf("PARALLEL_WORKERS must be >= 1")
	}
	if c.Retries.MaxRetries < 0 {
		return fmt.Errorf("MAX_RETRIES must be >= 0")
	}
	return nil
}

