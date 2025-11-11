package errors

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewAndOptions(t *testing.T) {
	ts := time.Unix(0, 0).UTC()
	err := New("EXAMPLE", "example failure",
		WithDetail("detail"),
		WithRequestID("req-123"),
		WithTraceID("trace-456"),
		WithActor(&Actor{Subject: "user-1", Roles: []string{"admin"}}),
		WithTimestamp(ts),
	)

	if err.Code != "EXAMPLE" || err.Message != "example failure" {
		t.Fatalf("unexpected code or message: %+v", err)
	}
	if err.Detail != "detail" || err.RequestID != "req-123" || err.TraceID != "trace-456" {
		t.Fatalf("unexpected metadata: %+v", err)
	}
	if err.Actor == nil || err.Actor.Subject != "user-1" {
		t.Fatalf("missing actor: %+v", err)
	}
	if !err.Timestamp.Equal(ts) {
		t.Fatalf("unexpected timestamp: %s", err.Timestamp)
	}
}

func TestMarshalWrapsGenericError(t *testing.T) {
	data, marshalErr := Marshal(assertionError("boom"))
	if marshalErr != nil {
		t.Fatalf("marshal failed: %v", marshalErr)
	}

	var payload Error
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("json failed: %v", err)
	}

	if payload.Code != "INTERNAL" {
		t.Fatalf("expected INTERNAL code, got %s", payload.Code)
	}
	if payload.Detail != "boom" {
		t.Fatalf("expected detail to propagate underlying message")
	}
}

type assertionError string

func (a assertionError) Error() string { return string(a) }
