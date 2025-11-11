package config

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	unset := snapshotEnv(t, []string{
		"SERVICE_NAME", "SERVICE_ADDRESS",
		"OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_EXPORTER_OTLP_PROTOCOL",
		"DATABASE_DSN",
	})
	defer unset()

	cfg, err := Load(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Service.Name != "shared-service" {
		t.Fatalf("expected default service name, got %s", cfg.Service.Name)
	}
	if cfg.Telemetry.Protocol != "grpc" {
		t.Fatalf("expected default protocol grpc")
	}
	if cfg.Telemetry.Insecure {
		t.Fatalf("expected default insecure false")
	}
}

func TestLoadCustom(t *testing.T) {
	defer snapshotEnv(t, []string{
		"SERVICE_NAME", "OTEL_EXPORTER_OTLP_PROTOCOL",
		"OTEL_EXPORTER_OTLP_HEADERS", "DATABASE_CONN_MAX_LIFETIME",
	})()

	os.Setenv("SERVICE_NAME", "catalog")
	os.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "http")
	os.Setenv("OTEL_EXPORTER_OTLP_HEADERS", "x-token=abc, x-team = shared ")
	os.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "true")
	os.Setenv("DATABASE_CONN_MAX_LIFETIME", "10m")

	cfg, err := Load(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Service.Name != "catalog" {
		t.Fatalf("expected overridden service name")
	}
	if cfg.Telemetry.Protocol != "http" {
		t.Fatalf("expected protocol http")
	}
	if cfg.Telemetry.Headers["x-token"] != "abc" {
		t.Fatalf("expected header parsed")
	}
	if !cfg.Telemetry.Insecure {
		t.Fatalf("expected insecure flag true")
	}
	if cfg.Database.ConnMaxLifetime != 10*time.Minute {
		t.Fatalf("expected lifetime 10m, got %s", cfg.Database.ConnMaxLifetime)
	}
}

func TestLoadValidation(t *testing.T) {
	defer snapshotEnv(t, []string{"SERVICE_NAME", "OTEL_EXPORTER_OTLP_PROTOCOL"})()

	os.Setenv("SERVICE_NAME", "")

	if _, err := Load(context.Background()); err == nil {
		t.Fatalf("expected validation error")
	}

	os.Setenv("SERVICE_NAME", "ok")
	os.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "ws")
	if _, err := Load(context.Background()); err == nil {
		t.Fatalf("expected invalid protocol error")
	}
}

func TestMustLoadPanics(t *testing.T) {
	defer snapshotEnv(t, []string{"SERVICE_NAME"})()
	os.Setenv("SERVICE_NAME", "")

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic from MustLoad")
		}
	}()

	MustLoad(context.Background())
}

func snapshotEnv(t *testing.T, keys []string) func() {
	t.Helper()
	values := make(map[string]*string, len(keys))
	for _, key := range keys {
		if val, ok := os.LookupEnv(key); ok {
			v := val
			values[key] = &v
		} else {
			values[key] = nil
		}
	}
	return func() {
		for key, val := range values {
			if val == nil {
				os.Unsetenv(key)
				continue
			}
			os.Setenv(key, *val)
		}
	}
}
