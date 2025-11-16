package logging

import (
	"context"
	"io"
	"os"
	"strings"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger wraps zap.Logger with standardized configuration and OpenTelemetry integration.
type Logger struct {
	*zap.Logger
	config Config
}

// New creates a new logger with the provided configuration.
func New(cfg Config) (*Logger, error) {
	if cfg.ServiceName == "" {
		cfg.ServiceName = "unknown"
	}
	if cfg.Environment == "" {
		cfg.Environment = getEnvOrDefault("ENVIRONMENT", "development")
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = getEnvOrDefault("LOG_LEVEL", "info")
	}
	if cfg.OutputPath == "" {
		cfg.OutputPath = "stdout"
	}

	// Parse log level
	level := parseLogLevel(cfg.LogLevel)

	// Determine output writer
	writer, err := getOutputWriter(cfg.OutputPath)
	if err != nil {
		return nil, err
	}

	// Build encoder config
	encoderConfig := getEncoderConfig(cfg.IsDevelopment())

	// Create core
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(writer),
		zapcore.Level(level),
	)

	// Build logger with options
	opts := []zap.Option{
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
		zap.Fields(
			zap.String("service", cfg.ServiceName),
			zap.String("environment", cfg.Environment),
		),
	}

	logger := zap.New(core, opts...)

	return &Logger{
		Logger: logger,
		config: cfg,
	}, nil
}

// MustNew creates a new logger and panics on error.
func MustNew(cfg Config) *Logger {
	logger, err := New(cfg)
	if err != nil {
		panic(err)
	}
	return logger
}

// WithContext returns a logger with OpenTelemetry trace context fields.
func (l *Logger) WithContext(ctx context.Context) *zap.Logger {
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return l.Logger
	}

	spanCtx := span.SpanContext()
	fields := []zap.Field{
		zap.String("trace_id", spanCtx.TraceID().String()),
		zap.String("span_id", spanCtx.SpanID().String()),
	}

	return l.Logger.With(fields...)
}

// WithRequestID returns a logger with request_id field.
func (l *Logger) WithRequestID(requestID string) *zap.Logger {
	return l.Logger.With(zap.String("request_id", requestID))
}

// WithUserID returns a logger with user_id field.
func (l *Logger) WithUserID(userID string) *zap.Logger {
	return l.Logger.With(zap.String("user_id", userID))
}

// WithOrgID returns a logger with org_id field.
func (l *Logger) WithOrgID(orgID string) *zap.Logger {
	return l.Logger.With(zap.String("org_id", orgID))
}

// WithFields returns a logger with additional fields.
func (l *Logger) WithFields(fields ...zap.Field) *zap.Logger {
	return l.Logger.With(fields...)
}

// Sync flushes any buffered log entries.
func (l *Logger) Sync() error {
	return l.Logger.Sync()
}

// parseLogLevel converts a string log level to zapcore.Level.
func parseLogLevel(level string) zapcore.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

// getEncoderConfig returns encoder config based on environment.
func getEncoderConfig(development bool) zapcore.EncoderConfig {
	if development {
		cfg := zap.NewDevelopmentEncoderConfig()
		cfg.EncodeTime = zapcore.ISO8601TimeEncoder
		cfg.EncodeLevel = zapcore.LowercaseLevelEncoder
		return cfg
	}

	cfg := zap.NewProductionEncoderConfig()
	cfg.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.EncodeLevel = zapcore.LowercaseLevelEncoder
	return cfg
}

// getOutputWriter returns the output writer for the given path.
func getOutputWriter(path string) (io.Writer, error) {
	switch strings.ToLower(path) {
	case "stdout":
		return os.Stdout, nil
	case "stderr":
		return os.Stderr, nil
	default:
		// File path
		file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, err
		}
		return file, nil
	}
}

