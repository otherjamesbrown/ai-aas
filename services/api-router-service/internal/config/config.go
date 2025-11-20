package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
)

// Config represents the runtime configuration for the API Router Service.
type Config struct {
	ServiceName   string `envconfig:"SERVICE_NAME" default:"api-router-service"`
	HTTPPort      int    `envconfig:"HTTP_PORT" default:"8080"`
	AdminPort     int    `envconfig:"ADMIN_PORT" default:"8443"`
	Environment   string `envconfig:"ENVIRONMENT" default:"development"`
	LogLevel      string `envconfig:"LOG_LEVEL" default:"info"`

	// Telemetry
	TelemetryEndpoint string `envconfig:"OTEL_EXPORTER_OTLP_ENDPOINT" default:"localhost:4317"`
	TelemetryProtocol string `envconfig:"OTEL_EXPORTER_OTLP_PROTOCOL" default:"grpc"`
	TelemetryInsecure bool   `envconfig:"OTEL_EXPORTER_OTLP_INSECURE" default:"true"`

	// Database (PostgreSQL for model registry)
	DatabaseURL string `envconfig:"DATABASE_URL" default:"postgres://postgres:postgres@localhost:5432/ai_aas_operational?sslmode=disable"`

	// Redis
	RedisAddr     string `envconfig:"REDIS_ADDR" default:"localhost:6379"`
	RedisPassword string `envconfig:"REDIS_PASSWORD" default:""`
	RedisDB       int    `envconfig:"REDIS_DB" default:"0"`

	// Kafka
	KafkaBrokers string `envconfig:"KAFKA_BROKERS" default:"localhost:9092"`
	KafkaTopic   string `envconfig:"KAFKA_TOPIC" default:"usage.records.v1"`

	// Config Service
	ConfigServiceEndpoint string `envconfig:"CONFIG_SERVICE_ENDPOINT" default:"localhost:2379"`
	ConfigWatchEnabled     bool   `envconfig:"CONFIG_WATCH_ENABLED" default:"true"`
	ConfigCachePath        string `envconfig:"CONFIG_CACHE_PATH" default:"/tmp/api-router-config.db"`

	// Backend endpoints (comma-separated: id1:uri1,id2:uri2)
	BackendEndpoints string `envconfig:"BACKEND_ENDPOINTS" default:"mock-backend-1:http://localhost:8001/v1/completions,mock-backend-2:http://localhost:8002/v1/completions"`

	// Rate Limiting
	RateLimitRedisAddr string `envconfig:"RATE_LIMIT_REDIS_ADDR" default:"localhost:6379"`
	RateLimitDefaultRPS int    `envconfig:"RATE_LIMIT_DEFAULT_RPS" default:"100"`
	RateLimitBurstSize  int    `envconfig:"RATE_LIMIT_BURST_SIZE" default:"200"`

	// Budget Service
	BudgetServiceEndpoint string        `envconfig:"BUDGET_SERVICE_ENDPOINT" default:""`
	BudgetServiceTimeout  time.Duration `envconfig:"BUDGET_SERVICE_TIMEOUT" default:"2s"`

	// User-Org Service (for API key validation)
	UserOrgServiceURL string        `envconfig:"USER_ORG_SERVICE_URL" default:"http://localhost:8081"`
	UserOrgServiceTimeout time.Duration `envconfig:"USER_ORG_SERVICE_TIMEOUT" default:"2s"`

	// Audit/Kafka
	KafkaAuditTopic string `envconfig:"KAFKA_AUDIT_TOPIC" default:"audit.router"`

	// Health Monitoring
	HealthCheckInterval time.Duration `envconfig:"HEALTH_CHECK_INTERVAL" default:"10s"`

	// Usage Accounting
	UsageBufferDir string `envconfig:"USAGE_BUFFER_DIR" default:"/tmp/api-router-usage-buffer"`
}

// BackendEndpointConfig represents a configured backend endpoint.
type BackendEndpointConfig struct {
	ID          string
	URI         string
	ModelVariant string
	Timeout     time.Duration
}

// BackendRegistry manages backend endpoint configurations.
type BackendRegistry struct {
	backends map[string]*BackendEndpointConfig
}

// NewBackendRegistry creates a new backend registry from config.
func NewBackendRegistry(cfg *Config) *BackendRegistry {
	registry := &BackendRegistry{
		backends: make(map[string]*BackendEndpointConfig),
	}

	// Parse backend endpoints from config
	if cfg.BackendEndpoints != "" {
		entries := strings.Split(cfg.BackendEndpoints, ",")
		for _, entry := range entries {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}

			parts := strings.SplitN(entry, ":", 2)
			if len(parts) != 2 {
				continue // Skip invalid entries
			}

			backendID := strings.TrimSpace(parts[0])
			backendURI := strings.TrimSpace(parts[1])

			registry.backends[backendID] = &BackendEndpointConfig{
				ID:          backendID,
				URI:         backendURI,
				ModelVariant: "", // Will be set from routing policy
				Timeout:     30 * time.Second,
			}
		}
	}

	return registry
}

// GetBackend returns the backend configuration for the given ID.
func (r *BackendRegistry) GetBackend(backendID string) (*BackendEndpointConfig, error) {
	backend, ok := r.backends[backendID]
	if !ok {
		return nil, fmt.Errorf("backend not found: %s", backendID)
	}
	return backend, nil
}

// RegisterBackend registers or updates a backend configuration.
func (r *BackendRegistry) RegisterBackend(backendID, uri string, timeout time.Duration) {
	if r.backends == nil {
		r.backends = make(map[string]*BackendEndpointConfig)
	}
	r.backends[backendID] = &BackendEndpointConfig{
		ID:      backendID,
		URI:     uri,
		Timeout: timeout,
	}
}

// ListBackends returns all registered backend IDs.
func (r *BackendRegistry) ListBackends() []string {
	ids := make([]string, 0, len(r.backends))
	for id := range r.backends {
		ids = append(ids, id)
	}
	return ids
}

// Load reads environment variables into Config.
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

