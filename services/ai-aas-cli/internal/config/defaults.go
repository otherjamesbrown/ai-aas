package config

import (
	"os"

	"github.com/spf13/viper"
)

// getBaseDomain returns the base domain from environment or defaults to development.
// This supports the centralized domain configuration pattern.
// Set AI_AAS_BASE_DOMAIN environment variable to override (e.g., "prod.otherjamesbrown.com").
func getBaseDomain() string {
	if domain := os.Getenv("AI_AAS_BASE_DOMAIN"); domain != "" {
		return domain
	}
	return "dev.otherjamesbrown.com"
}

// ApplyDefaults sets default configuration values in the provided Viper instance.
//
// Domain Configuration:
// ---------------------
// Default endpoints are computed from AI_AAS_BASE_DOMAIN environment variable.
// See configs/environments/*.yaml for canonical domain configuration per environment.
//
// Environment Variable Overrides:
// - AI_AAS_BASE_DOMAIN: Base domain for all services (default: dev.otherjamesbrown.com)
// - AI_AAS_API_ENDPOINTS_USER_ORG_SERVICE: User-Org service endpoint
// - AI_AAS_API_ENDPOINTS_ANALYTICS_SERVICE: Analytics service endpoint
// - AI_AAS_API_ENDPOINTS_INFERENCE_SERVICE: API Router endpoint
//
// Or use a config file at ~/.ai-aas/config.yaml
func ApplyDefaults(v *viper.Viper) {
	baseDomain := getBaseDomain()

	// API Endpoints (computed from base domain for easy environment switching)
	// Users can override these with environment variables or config file
	v.SetDefault("api-endpoints.user-org-service", "https://user-org."+baseDomain)
	v.SetDefault("api-endpoints.analytics-service", "https://analytics."+baseDomain)
	v.SetDefault("api-endpoints.config-service", "localhost:2379")             // etcd gRPC endpoint (requires port-forward)
	v.SetDefault("api-endpoints.inference-service", "https://api."+baseDomain) // API Router for inference

	// TLS Configuration
	v.SetDefault("tls.ca-cert-file", "") // Empty by default, use system certs

	// Database (default to local PostgreSQL for development)
	v.SetDefault("database.url", "postgres://postgres:postgres@localhost:5432/ai_aas_operational?sslmode=disable")

	// Output Settings
	v.SetDefault("defaults.output-format", "table") // table, json, csv
	v.SetDefault("defaults.verbose", false)
	v.SetDefault("defaults.quiet", false)
	v.SetDefault("defaults.confirm", false) // Require explicit --confirm for destructive ops

	// Retry Settings
	v.SetDefault("retry.max-attempts", 3)
	v.SetDefault("retry.timeout", 30)      // seconds
	v.SetDefault("retry.initial-delay", 1) // seconds
	v.SetDefault("retry.max-delay", 4)     // seconds

	// Timeouts
	v.SetDefault("timeouts.health-check", 5) // seconds
	v.SetDefault("timeouts.operation", 300)  // seconds (5 minutes)

	// Progress Indicators
	v.SetDefault("progress.enabled", true)
	v.SetDefault("progress.min-duration", 30) // Show progress for operations >30s
}
