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

	// Sampling configures log sampling to reduce volume.
	// If nil, no sampling is applied (all logs are emitted).
	Sampling *SamplingConfig
}

// SamplingConfig controls log sampling behavior.
type SamplingConfig struct {
	// Enabled turns on sampling. If false, all logs are emitted.
	Enabled bool

	// Initial is the number of messages to log each second before sampling.
	// After this threshold, only one in every Thereafter messages is logged.
	// Defaults to 100 if not set.
	Initial int

	// Thereafter samples only one in N messages after the initial threshold.
	// For debug level, set to 100 (1-in-100).
	// For info level, set to 10 (1-in-10).
	// Warn and error levels are never sampled.
	// Defaults to 100 if not set.
	Thereafter int
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

// WithSampling sets the sampling configuration.
func (c Config) WithSampling(sampling *SamplingConfig) Config {
	c.Sampling = sampling
	return c
}

// DefaultSamplingConfig returns default sampling configuration.
// - Sample 1-in-100 for debug level
// - Sample 1-in-10 for info level
// - Never sample warn/error (handled at core level)
func DefaultSamplingConfig() *SamplingConfig {
	return &SamplingConfig{
		Enabled:    true,
		Initial:    100,
		Thereafter: 100,
	}
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

