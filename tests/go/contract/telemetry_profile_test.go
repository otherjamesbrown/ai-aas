package contract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestTelemetryProfileContract(t *testing.T) {
	profile := map[string]any{
		"service_name": "shared-example",
		"otel_exporter": map[string]any{
			"endpoint":   "http://otel-collector:4317",
			"protocol":   "grpc",
			"headers":    map[string]any{"authorization": "Bearer token"},
			"timeout_ms": 2000,
		},
		"log": map[string]any{
			"level":           "info",
			"required_fields": []any{"request_id", "trace_id", "actor.subject"},
			"sampling_rate":   1,
		},
		"metrics": map[string]any{
			"histograms": []any{
				map[string]any{
					"name":    "http.server.duration",
					"buckets": []any{0.01, 0.05, 0.1},
				},
			},
			"resource_attributes": map[string]any{
				"service.namespace": "shared",
			},
		},
		"tracing": map[string]any{
			"sampler":     "ratio",
			"sampler_arg": 0.25,
			"propagators": []any{"tracecontext", "baggage"},
		},
	}

	serviceName := profile["service_name"].(string)
	matched, err := regexp.MatchString("^[a-z0-9-]+$", serviceName)
	if err != nil || !matched {
		t.Fatalf("service_name must match pattern: %s", serviceName)
	}

	exporter := profile["otel_exporter"].(map[string]any)
	if exporter["protocol"] != "grpc" && exporter["protocol"] != "http/protobuf" {
		t.Fatalf("invalid protocol: %v", exporter["protocol"])
	}

	if _, ok := exporter["endpoint"].(string); !ok {
		t.Fatalf("endpoint must be a string")
	}

	log := profile["log"].(map[string]any)
	if len(log["required_fields"].([]any)) < 3 {
		t.Fatalf("expected at least 3 required log fields")
	}

	tracing := profile["tracing"].(map[string]any)
	if tracing["sampler"] != "ratio" {
		t.Fatalf("expected sampler ratio")
	}
	if tracing["sampler_arg"].(float64) < 0 || tracing["sampler_arg"].(float64) > 1 {
		t.Fatalf("sampler_arg must be between 0 and 1")
	}

	raw, err := json.Marshal(profile)
	if err != nil {
		t.Fatalf("failed to marshal profile: %v", err)
	}
	if len(raw) == 0 {
		t.Fatalf("expected marshal output")
	}

	// Ensure schema file remains available for downstream consumers.
	schemaPath := filepath.Join("..", "..", "..", "specs", "004-shared-libraries", "contracts", "telemetry-profile.schema.json")
	if _, err := os.Stat(schemaPath); err != nil {
		t.Fatalf("schema missing: %v", err)
	}
}
