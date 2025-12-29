package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the admin-api-service
type Config struct {
	// Server settings
	Port    int
	Version string

	// Database settings
	DatabaseURL   string
	DBMaxConns    int
	DBMinConns    int
	DBConnTimeout time.Duration
	DBIdleTimeout time.Duration

	// Authentication
	APIKeyHashSalt    string
	MasterAdminAPIKey string // Master API key for admin access (required)

	// Rate limiting
	RateLimitPerMin int

	// Logging
	LogLevel string

	// Observability
	MetricsEnabled bool
	TracingEnabled bool
	OTLPEndpoint   string

	// External services
	UserOrgServiceURL string // URL of user-org-service for API key validation

	// Kubernetes settings
	Kubeconfig string // Path to kubeconfig file (empty = use in-cluster config)

	// Worker settings
	WorkerEnabled                bool
	WorkerPollInterval           time.Duration
	WorkerProgressUpdateInterval time.Duration
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		Port:              getEnvInt("PORT", 8080),
		Version:           getEnv("VERSION", "v1.0.0"),
		DatabaseURL:       getEnv("DATABASE_URL", ""),
		DBMaxConns:        getEnvInt("DB_MAX_CONNS", 50),
		DBMinConns:        getEnvInt("DB_MIN_CONNS", 10),
		DBConnTimeout:     getEnvDuration("DB_CONN_TIMEOUT", 5*time.Second),
		DBIdleTimeout:     getEnvDuration("DB_IDLE_TIMEOUT", 5*time.Minute),
		APIKeyHashSalt:    getEnv("API_KEY_HASH_SALT", ""),
		MasterAdminAPIKey: getEnv("MASTER_ADMIN_API_KEY", ""),
		RateLimitPerMin:   getEnvInt("RATE_LIMIT_PER_MIN", 100),
		LogLevel:          getEnv("LOG_LEVEL", "info"),
		MetricsEnabled:    getEnvBool("METRICS_ENABLED", true),
		TracingEnabled:    getEnvBool("TRACING_ENABLED", false),
		OTLPEndpoint:      getEnv("OTLP_ENDPOINT", ""),
		// External services
		UserOrgServiceURL: getEnv("USER_ORG_SERVICE_URL", ""),
		// Kubernetes settings
		Kubeconfig: getEnv("KUBECONFIG", ""),
		// Worker settings
		WorkerEnabled:                getEnvBool("WORKER_ENABLED", true),
		WorkerPollInterval:           getEnvDuration("WORKER_POLL_INTERVAL", 5*time.Second),
		WorkerProgressUpdateInterval: getEnvDuration("WORKER_PROGRESS_UPDATE_INTERVAL", 10*time.Second),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	if cfg.MasterAdminAPIKey == "" {
		return nil, fmt.Errorf("MASTER_ADMIN_API_KEY is required")
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
