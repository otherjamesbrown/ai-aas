package config

import (
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
)

// Config holds all configuration for the analytics service.
type Config struct {
	// Service identity
	ServiceName string `envconfig:"SERVICE_NAME" default:"analytics-service"`
	Environment string `envconfig:"ENVIRONMENT" default:"development"`

	// HTTP server
	HTTPPort int `envconfig:"HTTP_PORT" default:"8084"`

	// Database
	DatabaseURL string `envconfig:"DATABASE_URL" required:"true"`

	// Redis
	RedisURL string `envconfig:"REDIS_URL" default:"redis://localhost:6379"`

	// RabbitMQ
	RabbitMQURL      string `envconfig:"RABBITMQ_URL" default:"amqp://guest:guest@localhost:5672"`
	RabbitMQStream   string `envconfig:"RABBITMQ_STREAM" default:"analytics.usage.v1"`
	RabbitMQConsumer string `envconfig:"RABBITMQ_CONSUMER" default:"analytics-service"`

	// Linode Object Storage (S3-compatible)
	S3Endpoint  string `envconfig:"S3_ENDPOINT"` // Linode Object Storage endpoint (e.g., us-east-1.linodeobjects.com)
	S3AccessKey string `envconfig:"S3_ACCESS_KEY"` // Linode access key
	S3SecretKey string `envconfig:"S3_SECRET_KEY"` // Linode secret key
	S3Bucket    string `envconfig:"S3_BUCKET" default:"analytics-exports"`
	S3Region    string `envconfig:"S3_REGION" default:"us-east-1"` // Linode region

	// Observability
	TelemetryEndpoint string            `envconfig:"OTEL_EXPORTER_OTLP_ENDPOINT"`
	TelemetryProtocol string            `envconfig:"OTEL_EXPORTER_OTLP_PROTOCOL" default:"grpc"`
	TelemetryInsecure bool              `envconfig:"OTEL_EXPORTER_OTLP_INSECURE" default:"true"`
	LogLevel          string            `envconfig:"LOG_LEVEL" default:"info"`

	// Ingestion
	IngestionBatchSize    int           `envconfig:"INGESTION_BATCH_SIZE" default:"1000"`
	IngestionBatchTimeout time.Duration `envconfig:"INGESTION_BATCH_TIMEOUT" default:"5s"`
	IngestionWorkers      int           `envconfig:"INGESTION_WORKERS" default:"4"`

	// Aggregation
	AggregationWorkers int           `envconfig:"AGGREGATION_WORKERS" default:"2"`
	RollupInterval     time.Duration `envconfig:"ROLLUP_INTERVAL" default:"1h"`

	// Freshness
	FreshnessCacheTTL time.Duration `envconfig:"FRESHNESS_CACHE_TTL" default:"5m"`

	// Export Worker
	ExportWorkerInterval   time.Duration `envconfig:"EXPORT_WORKER_INTERVAL" default:"30s"`
	ExportWorkerConcurrency int          `envconfig:"EXPORT_WORKER_CONCURRENCY" default:"2"`
	ExportSignedURLTTL     time.Duration `envconfig:"EXPORT_SIGNED_URL_TTL" default:"24h"`

	// Security
	EnableRBAC bool `envconfig:"ENABLE_RBAC" default:"true"`
}

// Load loads configuration from environment variables.
func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// MustLoad loads configuration and panics on error.
func MustLoad() *Config {
	cfg, err := Load()
	if err != nil {
		panic(fmt.Sprintf("failed to load configuration: %v", err))
	}
	return cfg
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.HTTPPort <= 0 || c.HTTPPort > 65535 {
		return fmt.Errorf("HTTP_PORT must be between 1 and 65535, got %d", c.HTTPPort)
	}
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.IngestionBatchSize <= 0 {
		return fmt.Errorf("INGESTION_BATCH_SIZE must be positive, got %d", c.IngestionBatchSize)
	}
	if c.IngestionWorkers <= 0 {
		return fmt.Errorf("INGESTION_WORKERS must be positive, got %d", c.IngestionWorkers)
	}
	if c.AggregationWorkers <= 0 {
		return fmt.Errorf("AGGREGATION_WORKERS must be positive, got %d", c.AggregationWorkers)
	}
	if c.ExportWorkerConcurrency <= 0 {
		return fmt.Errorf("EXPORT_WORKER_CONCURRENCY must be positive, got %d", c.ExportWorkerConcurrency)
	}
	return nil
}

