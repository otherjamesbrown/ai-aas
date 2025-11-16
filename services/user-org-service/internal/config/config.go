// Package config provides environment variable-based configuration loading.
//
// Purpose:
//
//	This package defines the service configuration structure and provides
//	functions to load configuration from environment variables using envconfig.
//	All binaries (admin-api, reconciler, seed) share this configuration structure.
//
// Dependencies:
//   - github.com/kelseyhightower/envconfig: Environment variable parsing
//
// Key Responsibilities:
//   - Config struct defines all service configuration fields
//   - Load reads and validates environment variables
//   - MustLoad exits the process if configuration is invalid
//
// Requirements Reference:
//   - specs/005-user-org-service/spec.md#NFR-001 (Configuration Management)
//
// Debugging Notes:
//   - Required fields: DATABASE_URL, OAUTH_HMAC_SECRET, OAUTH_CLIENT_SECRET
//   - Defaults provided for optional fields (ports, Redis, log level)
//   - OAuthHMACSecret must be at least 32 bytes (validated by provider)
//   - Redis is optional (no-op cache used if not configured)
//
// Thread Safety:
//   - Config struct is read-only after loading (safe for concurrent read access)
//
// Error Handling:
//   - Load returns wrapped errors from envconfig.Process
//   - MustLoad writes to stderr and exits on error
package config

import (
	"fmt"
	"os"

	"github.com/kelseyhightower/envconfig"
)

// Config represents shared runtime configuration for binaries in the user-org service.
// All fields are populated from environment variables with defaults where specified.
// Required fields must be set or Load/MustLoad will return an error.
type Config struct {
	// ServiceName is emitted in logs and metrics.
	ServiceName string `envconfig:"SERVICE_NAME" default:"user-org-service"`
	// HTTPPort is the port the HTTP server listens on.
	HTTPPort int `envconfig:"HTTP_PORT" default:"8081"`
	// DatabaseURL is the Postgres connection string for the primary service database.
	DatabaseURL string `envconfig:"DATABASE_URL" required:"true"`
	// RedisAddr is the host:port of the Redis instance used for caching OAuth sessions.
	RedisAddr string `envconfig:"REDIS_ADDR" default:"localhost:6379"`
	// RedisPassword is the optional password for Redis authentication.
	RedisPassword string `envconfig:"REDIS_PASSWORD" default:""`
	// RedisDB selects the logical Redis database index.
	RedisDB int `envconfig:"REDIS_DB" default:"0"`
	// LogLevel controls zerolog global level (debug, info, warn, error).
	LogLevel string `envconfig:"LOG_LEVEL" default:"info"`
	// Environment describes the current deployment environment (dev, staging, prod, etc.).
	Environment string `envconfig:"ENVIRONMENT" default:"development"`
	// OAuthHMACSecret seeds the HMAC strategy used by Fosité.
	OAuthHMACSecret string `envconfig:"OAUTH_HMAC_SECRET" required:"true"`
	// OAuthClientID is the identifier for the primary confidential client used by first-party flows.
	OAuthClientID string `envconfig:"OAUTH_CLIENT_ID" default:"user-org-admin"`
	// OAuthClientSecret is the plaintext secret for the confidential client. It is hashed before storage.
	OAuthClientSecret string `envconfig:"OAUTH_CLIENT_SECRET" required:"true"`
	// KafkaBrokers is a comma-separated list of Kafka broker addresses (e.g., "broker1:9092,broker2:9092").
	// If empty, audit events will be logged instead of sent to Kafka.
	KafkaBrokers string `envconfig:"KAFKA_BROKERS" default:""`
	// KafkaTopic is the Kafka topic name for audit events (default: "audit.identity").
	KafkaTopic string `envconfig:"KAFKA_TOPIC" default:"audit.identity"`
	// KafkaClientID is the client ID used when connecting to Kafka.
	KafkaClientID string `envconfig:"KAFKA_CLIENT_ID" default:"user-org-service"`
	// OIDCBaseURL is the base URL for OIDC callback redirects (e.g., "https://api.example.com").
	// Used to construct callback URLs for IdP providers.
	OIDCBaseURL string `envconfig:"OIDC_BASE_URL" default:""`
	// OIDCGoogleClientID is the Google OAuth2 client ID for IdP federation.
	OIDCGoogleClientID string `envconfig:"OIDC_GOOGLE_CLIENT_ID" default:""`
	// OIDCGoogleClientSecret is the Google OAuth2 client secret for IdP federation.
	OIDCGoogleClientSecret string `envconfig:"OIDC_GOOGLE_CLIENT_SECRET" default:""`
	// OIDCGithubClientID is the GitHub OAuth2 client ID for IdP federation.
	OIDCGithubClientID string `envconfig:"OIDC_GITHUB_CLIENT_ID" default:""`
	// OIDCGithubClientSecret is the GitHub OAuth2 client secret for IdP federation.
	OIDCGithubClientSecret string `envconfig:"OIDC_GITHUB_CLIENT_SECRET" default:""`

	// Lockout configuration
	// LockoutMaxAttempts is the maximum number of failed login attempts before lockout (default: 5).
	LockoutMaxAttempts int `envconfig:"LOCKOUT_MAX_ATTEMPTS" default:"5"`
	// LockoutDurationMinutes is the duration of lockout in minutes (default: 15).
	LockoutDurationMinutes int `envconfig:"LOCKOUT_DURATION_MINUTES" default:"15"`
	// LockoutWindowMinutes is the time window for counting failed attempts in minutes (default: 15).
	LockoutWindowMinutes int `envconfig:"LOCKOUT_WINDOW_MINUTES" default:"15"`
	// RecoveryRequiresAdminApproval enables admin approval workflow for recovery requests (default: false).
	RecoveryRequiresAdminApproval bool `envconfig:"RECOVERY_REQUIRES_ADMIN_APPROVAL" default:"false"`
}

// Load reads environment variables into Config, applying defaults where necessary.
func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("config: process env: %w", err)
	}
	return &cfg, nil
}

// MustLoad returns Config or exits the process.
func MustLoad() *Config {
	cfg, err := Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return cfg
}
