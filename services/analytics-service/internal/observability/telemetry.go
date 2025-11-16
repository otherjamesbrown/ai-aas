// Package observability provides OpenTelemetry and structured logging initialization.
//
// Purpose:
//   This package wires together OpenTelemetry tracing and structured logging
//   using the shared observability library and zap logger. It provides a unified
//   interface for telemetry initialization and shutdown.
//
// Dependencies:
//   - github.com/otherjamesbrown/ai-aas/shared/go/observability: OpenTelemetry setup
//   - go.uber.org/zap: Structured logging
//
// Key Responsibilities:
//   - Initialize OpenTelemetry tracer provider
//   - Configure zap logger with structured output
//   - Provide shutdown hooks for graceful teardown
//   - Handle telemetry failures gracefully
//
package observability

import (
	"context"
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/ai-aas/shared-go/logging"
	"github.com/otherjamesbrown/ai-aas/shared/go/observability"
)

// Observability bundles initialized telemetry components.
type Observability struct {
	TracerProvider *observability.Provider
	Logger         *zap.Logger
}

// Config controls observability initialization.
type Config struct {
	ServiceName string
	Environment string
	Endpoint    string
	Protocol    string
	Headers     map[string]string
	Insecure    bool
	LogLevel    string
}

// Init initializes OpenTelemetry and structured logging.
func Init(ctx context.Context, cfg Config) (*Observability, error) {
	// Initialize OpenTelemetry
	otelCfg := observability.Config{
		ServiceName: cfg.ServiceName,
		Environment: cfg.Environment,
		Endpoint:    cfg.Endpoint,
		Protocol:    cfg.Protocol,
		Headers:     cfg.Headers,
		Insecure:    cfg.Insecure,
	}

	tracerProvider, err := observability.Init(ctx, otelCfg)
	if err != nil {
		return nil, fmt.Errorf("init observability: %w", err)
	}

	// Initialize logger using shared logging package
	loggingCfg := logging.DefaultConfig().
		WithServiceName(cfg.ServiceName).
		WithEnvironment(cfg.Environment).
		WithLogLevel(cfg.LogLevel)

	loggerWrapper, err := logging.New(loggingCfg)
	if err != nil {
		return nil, fmt.Errorf("init logger: %w", err)
	}
	logger := loggerWrapper.Logger

	return &Observability{
		TracerProvider: tracerProvider,
		Logger:         logger,
	}, nil
}

// MustInit panics if Init returns an error.
func MustInit(ctx context.Context, cfg Config) *Observability {
	obs, err := Init(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize observability: %v\n", err)
		os.Exit(1)
	}
	return obs
}

// Shutdown gracefully shuts down observability components.
func (o *Observability) Shutdown(ctx context.Context) error {
	var firstErr error

	if o.TracerProvider != nil {
		if err := o.TracerProvider.Shutdown(ctx); err != nil {
			firstErr = err
		}
	}

	if o.Logger != nil {
		if err := o.Logger.Sync(); err != nil {
			// Ignore sync errors on stdout/stderr
			if !strings.Contains(err.Error(), "sync /dev/stdout") &&
				!strings.Contains(err.Error(), "sync /dev/stderr") {
				if firstErr == nil {
					firstErr = err
				}
			}
		}
	}

	return firstErr
}

// parseLogLevel is deprecated - use shared/go/logging package instead.
// Kept for backward compatibility during migration.
func parseLogLevel(level string) zapcore.Level {
	switch strings.ToLower(level) {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

