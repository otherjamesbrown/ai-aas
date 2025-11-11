package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// Config represents the top-level runtime configuration for shared services.
type Config struct {
	Service   ServiceConfig
	Telemetry TelemetryConfig
	Database  DatabaseConfig
}

// ServiceConfig captures generic service settings.
type ServiceConfig struct {
	Name    string
	Address string
}

// TelemetryConfig controls OpenTelemetry exporters.
type TelemetryConfig struct {
	Endpoint string
	Protocol string
	Headers  map[string]string
	Insecure bool
}

// DatabaseConfig contains connection parameters used by shared data helpers.
type DatabaseConfig struct {
	DSN             string
	MaxIdleConns    int
	MaxOpenConns    int
	ConnMaxLifetime time.Duration
}

// Load reads environment variables and returns a populated Config.
func Load(ctx context.Context) (Config, error) {
	_ = ctx // reserved for future use (Vault, remote stores, etc.)

	cfg := Config{
		Service: ServiceConfig{
			Name:    getEnv("SERVICE_NAME", "shared-service"),
			Address: getEnv("SERVICE_ADDRESS", ":8080"),
		},
		Telemetry: TelemetryConfig{
			Endpoint: getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
			Protocol: strings.ToLower(getEnv("OTEL_EXPORTER_OTLP_PROTOCOL", "grpc")),
			Headers:  parseHeaders(getEnv("OTEL_EXPORTER_OTLP_HEADERS", "")),
			Insecure: getEnvBool("OTEL_EXPORTER_OTLP_INSECURE", false),
		},
		Database: DatabaseConfig{
			DSN:             getEnv("DATABASE_DSN", ""),
			MaxIdleConns:    getEnvInt("DATABASE_MAX_IDLE_CONNS", 2),
			MaxOpenConns:    getEnvInt("DATABASE_MAX_OPEN_CONNS", 10),
			ConnMaxLifetime: getEnvDuration("DATABASE_CONN_MAX_LIFETIME", time.Minute*5),
		},
	}

	if err := validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// MustLoad calls Load and panics on error.
func MustLoad(ctx context.Context) Config {
	cfg, err := Load(ctx)
	if err != nil {
		panic(err)
	}
	return cfg
}

func validate(cfg Config) error {
	if strings.TrimSpace(cfg.Service.Name) == "" {
		return errors.New("SERVICE_NAME must be provided")
	}
	if cfg.Telemetry.Protocol != "grpc" && cfg.Telemetry.Protocol != "http" {
		return fmt.Errorf("unsupported OTLP protocol %q", cfg.Telemetry.Protocol)
	}
	return nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func parseHeaders(raw string) map[string]string {
	headers := map[string]string{}
	if raw == "" {
		return headers
	}
	pairs := strings.Split(raw, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			continue
		}
		headers[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
	}
	return headers
}

func getEnvInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		var parsed int
		if _, err := fmt.Sscanf(value, "%d", &parsed); err == nil {
			return parsed
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if value, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if value, ok := os.LookupEnv(key); ok {
		value = strings.ToLower(strings.TrimSpace(value))
		return value == "1" || value == "true" || value == "yes"
	}
	return fallback
}
