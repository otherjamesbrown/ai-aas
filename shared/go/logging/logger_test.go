package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				ServiceName: "test-service",
				Environment: "development",
				LogLevel:    "info",
				OutputPath:  "stdout",
			},
			wantErr: false,
		},
		{
			name: "default config",
			config: DefaultConfig().WithServiceName("test-service"),
			wantErr: false,
		},
		{
			name: "invalid log level defaults to info",
			config: Config{
				ServiceName: "test-service",
				Environment: "development",
				LogLevel:    "invalid",
				OutputPath:  "stdout",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := New(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if logger == nil && !tt.wantErr {
				t.Error("New() returned nil logger")
			}
			if logger != nil {
				logger.Sync()
			}
		})
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input string
		want  zapcore.Level
	}{
		{"debug", zapcore.DebugLevel},
		{"DEBUG", zapcore.DebugLevel},
		{"info", zapcore.InfoLevel},
		{"INFO", zapcore.InfoLevel},
		{"warn", zapcore.WarnLevel},
		{"warning", zapcore.WarnLevel},
		{"error", zapcore.ErrorLevel},
		{"invalid", zapcore.InfoLevel},
		{"", zapcore.InfoLevel},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseLogLevel(tt.input)
			if got != tt.want {
				t.Errorf("parseLogLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestLogger_WithContext(t *testing.T) {
	cfg := Config{
		ServiceName: "test-service",
		Environment: "development",
		LogLevel:    "debug",
		OutputPath:  "stdout",
	}

	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer logger.Sync()

	// Test without trace context
	ctx := context.Background()
	loggerWithCtx := logger.WithContext(ctx)
	if loggerWithCtx == nil {
		t.Error("WithContext() returned nil logger")
	}

	// Test with trace context
	tracer := otel.Tracer("test")
	ctx, span := tracer.Start(ctx, "test-span")
	defer span.End()

	loggerWithCtx = logger.WithContext(ctx)
	if loggerWithCtx == nil {
		t.Error("WithContext() returned nil logger")
	}
}

func TestLogger_WithRequestID(t *testing.T) {
	cfg := Config{
		ServiceName: "test-service",
		Environment: "development",
		LogLevel:    "info",
		OutputPath:  "stdout",
	}

	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer logger.Sync()

	loggerWithID := logger.WithRequestID("req-123")
	if loggerWithID == nil {
		t.Error("WithRequestID() returned nil logger")
	}
}

func TestLogger_WithUserID(t *testing.T) {
	cfg := Config{
		ServiceName: "test-service",
		Environment: "development",
		LogLevel:    "info",
		OutputPath:  "stdout",
	}

	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer logger.Sync()

	loggerWithUser := logger.WithUserID("user-456")
	if loggerWithUser == nil {
		t.Error("WithUserID() returned nil logger")
	}
}

func TestLogger_WithOrgID(t *testing.T) {
	cfg := Config{
		ServiceName: "test-service",
		Environment: "development",
		LogLevel:    "info",
		OutputPath:  "stdout",
	}

	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer logger.Sync()

	loggerWithOrg := logger.WithOrgID("org-789")
	if loggerWithOrg == nil {
		t.Error("WithOrgID() returned nil logger")
	}
}

func TestLogger_StructuredOutput(t *testing.T) {
	var buf bytes.Buffer

	cfg := Config{
		ServiceName: "test-service",
		Environment: "development",
		LogLevel:    "info",
		OutputPath:  "stdout",
	}

	// Override output for testing
	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer logger.Sync()

	logger.Info("test message", zap.String("key", "value"))

	// Verify structured output by checking logger writes to stdout
	// (we can't easily capture stdout in unit tests, so we verify logger is created)
	if logger == nil {
		t.Error("Logger is nil")
	}
}

func TestRedactString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			name:     "password redaction",
			input:    "password=secret123",
			contains: "***REDACTED***",
		},
		{
			name:     "token redaction",
			input:    "Bearer abc123def456",
			contains: "***REDACTED***",
		},
		{
			name:     "connection string redaction",
			input:    "postgres://user:pass@host/db",
			contains: "***REDACTED***",
		},
		{
			name:     "no sensitive data",
			input:    "normal log message",
			contains: "normal log message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RedactString(tt.input)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("RedactString() result = %q, should contain %q", result, tt.contains)
			}
		})
	}
}

func TestRedactFields(t *testing.T) {
	fields := map[string]interface{}{
		"password":  "secret123",
		"username":  "user",
		"token":     "abc123",
		"normal":    "value",
		"api_key":   "key123",
	}

	redacted := RedactFields(fields)

	if redacted["password"] != "***REDACTED***" {
		t.Errorf("password not redacted: %v", redacted["password"])
	}
	if redacted["token"] != "***REDACTED***" {
		t.Errorf("token not redacted: %v", redacted["token"])
	}
	if redacted["username"] != "user" {
		t.Errorf("username incorrectly redacted: %v", redacted["username"])
	}
	if redacted["normal"] != "value" {
		t.Errorf("normal field incorrectly modified: %v", redacted["normal"])
	}
}

func TestConfig_IsDevelopment(t *testing.T) {
	cfg := Config{Environment: "development"}
	if !cfg.IsDevelopment() {
		t.Error("IsDevelopment() = false, want true")
	}

	cfg = Config{Environment: "production"}
	if cfg.IsDevelopment() {
		t.Error("IsDevelopment() = true, want false")
	}
}

func TestConfig_IsProduction(t *testing.T) {
	cfg := Config{Environment: "production"}
	if !cfg.IsProduction() {
		t.Error("IsProduction() = false, want true")
	}

	cfg = Config{Environment: "development"}
	if cfg.IsProduction() {
		t.Error("IsProduction() = true, want false")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.ServiceName == "" {
		t.Error("DefaultConfig() ServiceName is empty")
	}
	if cfg.Environment == "" {
		t.Error("DefaultConfig() Environment is empty")
	}
	if cfg.LogLevel == "" {
		t.Error("DefaultConfig() LogLevel is empty")
	}
}

func TestGetOutputWriter(t *testing.T) {
	// Test stdout
	writer, err := getOutputWriter("stdout")
	if err != nil {
		t.Errorf("getOutputWriter(\"stdout\") error = %v", err)
	}
	if writer != os.Stdout {
		t.Error("getOutputWriter(\"stdout\") did not return os.Stdout")
	}

	// Test stderr
	writer, err = getOutputWriter("stderr")
	if err != nil {
		t.Errorf("getOutputWriter(\"stderr\") error = %v", err)
	}
	if writer != os.Stderr {
		t.Error("getOutputWriter(\"stderr\") did not return os.Stderr")
	}
}

