package contract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ai-aas/shared-go/errors"
)

func TestErrorResponseContract(t *testing.T) {
	err := errors.New("EXAMPLE", "example failure",
		errors.WithDetail("missing widget"),
		errors.WithRequestID("req-123"),
		errors.WithTraceID("trace-456"),
		errors.WithActor(&errors.Actor{
			Subject: "alice",
			Roles:   []string{"admin"},
		}),
	)

	raw, marshalErr := errors.Marshal(err)
	if marshalErr != nil {
		t.Fatalf("marshal failed: %v", marshalErr)
	}

	var payload map[string]any
	if unmarshalErr := json.Unmarshal(raw, &payload); unmarshalErr != nil {
		t.Fatalf("unmarshal failed: %v", unmarshalErr)
	}

	requiredFields := []string{"error", "code", "request_id", "trace_id", "timestamp"}
	for _, field := range requiredFields {
		if _, ok := payload[field]; !ok {
			t.Fatalf("expected field %q in payload", field)
		}
		if _, ok := payload[field].(string); !ok {
			t.Fatalf("expected field %q to be a string", field)
		}
	}

	if payload["code"] != "EXAMPLE" {
		t.Fatalf("expected code EXAMPLE, got %v", payload["code"])
	}

	ts := payload["timestamp"].(string)
	if _, err := time.Parse(time.RFC3339, ts); err != nil {
		t.Fatalf("expected timestamp in RFC3339 format: %v", err)
	}

	actor, ok := payload["actor"].(map[string]any)
	if !ok {
		t.Fatalf("expected actor object")
	}
	if actor["subject"] != "alice" {
		t.Fatalf("expected actor subject 'alice'")
	}
	roles, ok := actor["roles"].([]any)
	if !ok || len(roles) == 0 || roles[0] != "admin" {
		t.Fatalf("expected actor roles to include admin")
	}

	// Guard against extra unexpected fields defined in schema.
	schemaPath := filepath.Join("..", "..", "..", "specs", "004-shared-libraries", "contracts", "error-response.schema.json")
	schemaRaw, readErr := os.ReadFile(schemaPath)
	if readErr != nil {
		t.Fatalf("failed to read schema: %v", readErr)
	}
	if !strings.Contains(string(schemaRaw), `"error"`) {
		t.Fatalf("expected schema to require error field")
	}
}
