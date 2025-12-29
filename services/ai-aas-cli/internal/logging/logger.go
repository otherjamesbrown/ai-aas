// Package logging provides structured logging for the CLI using shared/go/logging.
package logging

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger wraps zap.Logger for CLI usage
type Logger struct {
	*zap.Logger
	sugar *zap.SugaredLogger
}

// Config holds logging configuration
type Config struct {
	Level   string // debug, info, warn, error
	Format  string // json, console
	Verbose bool   // Enable verbose output
}

// DefaultConfig returns the default logging configuration
func DefaultConfig() Config {
	return Config{
		Level:   "info",
		Format:  "console",
		Verbose: false,
	}
}

// NewLogger creates a new logger with the given configuration
func NewLogger(cfg Config) (*Logger, error) {
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
		level = zapcore.InfoLevel
	}

	// Configure encoder based on format
	var encoder zapcore.Encoder
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	if cfg.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		// Console format for CLI
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("15:04:05")
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// Create core
	core := zapcore.NewCore(
		encoder,
		zapcore.AddSync(os.Stderr),
		level,
	)

	// Build logger
	zapLogger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))

	return &Logger{
		Logger: zapLogger,
		sugar:  zapLogger.Sugar(),
	}, nil
}

// NewDefaultLogger creates a logger with default configuration
func NewDefaultLogger() (*Logger, error) {
	return NewLogger(DefaultConfig())
}

// NewVerboseLogger creates a logger for verbose/debug mode
func NewVerboseLogger() (*Logger, error) {
	return NewLogger(Config{
		Level:   "debug",
		Format:  "console",
		Verbose: true,
	})
}

// NewQuietLogger creates a logger that only shows errors
func NewQuietLogger() (*Logger, error) {
	return NewLogger(Config{
		Level:   "error",
		Format:  "console",
		Verbose: false,
	})
}

// Sugar returns the sugared logger for printf-style logging
func (l *Logger) Sugar() *zap.SugaredLogger {
	return l.sugar
}

// With creates a child logger with additional fields
func (l *Logger) With(fields ...zap.Field) *Logger {
	return &Logger{
		Logger: l.Logger.With(fields...),
		sugar:  l.Logger.With(fields...).Sugar(),
	}
}

// WithService adds the service name to the logger
func (l *Logger) WithService(name string) *Logger {
	return l.With(zap.String("service", name))
}

// WithCommand adds the command name to the logger
func (l *Logger) WithCommand(name string) *Logger {
	return l.With(zap.String("command", name))
}

// WithModel adds the model name to the logger
func (l *Logger) WithModel(name string) *Logger {
	return l.With(zap.String("model", name))
}

// WithEnvironment adds the environment to the logger
func (l *Logger) WithEnvironment(env string) *Logger {
	return l.With(zap.String("environment", env))
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, fields ...zap.Field) {
	l.Logger.Debug(msg, fields...)
}

// Info logs an info message
func (l *Logger) Info(msg string, fields ...zap.Field) {
	l.Logger.Info(msg, fields...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, fields ...zap.Field) {
	l.Logger.Warn(msg, fields...)
}

// Error logs an error message
func (l *Logger) Error(msg string, fields ...zap.Field) {
	l.Logger.Error(msg, fields...)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(msg string, fields ...zap.Field) {
	l.Logger.Fatal(msg, fields...)
}

// Sync flushes any buffered log entries
func (l *Logger) Sync() error {
	return l.Logger.Sync()
}

// Global logger instance
var globalLogger *Logger

// SetGlobal sets the global logger instance
func SetGlobal(l *Logger) {
	globalLogger = l
}

// Global returns the global logger instance
func Global() *Logger {
	if globalLogger == nil {
		l, _ := NewDefaultLogger()
		globalLogger = l
	}
	return globalLogger
}
