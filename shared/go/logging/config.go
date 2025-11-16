package logging

import (
	"os"
	"strings"
)

// Config controls logger initialization.
type Config struct {
	// ServiceName identifies the service emitting logs (required).
	ServiceName string

	// Environment is the deployment environment (development, staging, production).
	Environment string

	// LogLevel controls verbosity (debug, info, warn, error).
	// Defaults to "info" if empty or invalid.
	LogLevel string

	// OutputPath is the log output destination (stdout, stderr, or file path).
	// Defaults to "stdout".
	OutputPath string
}

// DefaultConfig returns a config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		ServiceName: "unknown",
		Environment: getEnvOrDefault("ENVIRONMENT", "development"),
		LogLevel:    getEnvOrDefault("LOG_LEVEL", "info"),
		OutputPath:  "stdout",
	}
}

// WithServiceName sets the service name.
func (c Config) WithServiceName(name string) Config {
	c.ServiceName = name
	return c
}

// WithEnvironment sets the environment.
func (c Config) WithEnvironment(env string) Config {
	c.Environment = env
	return c
}

// WithLogLevel sets the log level.
func (c Config) WithLogLevel(level string) Config {
	c.LogLevel = level
	return c
}

// WithOutputPath sets the output path.
func (c Config) WithOutputPath(path string) Config {
	c.OutputPath = path
	return c
}

// getEnvOrDefault returns the environment variable value or default.
func getEnvOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

// IsDevelopment returns true if environment is development.
func (c Config) IsDevelopment() bool {
	return strings.ToLower(c.Environment) == "development"
}

// IsProduction returns true if environment is production.
func (c Config) IsProduction() bool {
	return strings.ToLower(c.Environment) == "production"
}

