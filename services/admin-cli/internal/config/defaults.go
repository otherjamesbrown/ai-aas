package config

import (
	"github.com/spf13/viper"
)

// ApplyDefaults sets default configuration values in the provided Viper instance.
func ApplyDefaults(v *viper.Viper) {
	// API Endpoints (defaults to localhost for local development)
	v.SetDefault("api-endpoints.user-org-service", "http://localhost:8081")
	v.SetDefault("api-endpoints.analytics-service", "http://localhost:8084")

	// Database (default to local PostgreSQL for development)
	v.SetDefault("database.url", "postgres://postgres:postgres@localhost:5432/ai_aas_operational?sslmode=disable")

	// Output Settings
	v.SetDefault("defaults.output-format", "table") // table, json, csv
	v.SetDefault("defaults.verbose", false)
	v.SetDefault("defaults.quiet", false)
	v.SetDefault("defaults.confirm", false) // Require explicit --confirm for destructive ops

	// Retry Settings
	v.SetDefault("retry.max-attempts", 3)
	v.SetDefault("retry.timeout", 30) // seconds
	v.SetDefault("retry.initial-delay", 1) // seconds
	v.SetDefault("retry.max-delay", 4) // seconds

	// Timeouts
	v.SetDefault("timeouts.health-check", 5) // seconds
	v.SetDefault("timeouts.operation", 300) // seconds (5 minutes)

	// Progress Indicators
	v.SetDefault("progress.enabled", true)
	v.SetDefault("progress.min-duration", 30) // Show progress for operations >30s
}

