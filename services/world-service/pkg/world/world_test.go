package world

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/world", nil)
	rec := httptest.NewRecorder()

	Handler(rec, req)

	res := rec.Result()
	t.Cleanup(func() {
		if err := res.Body.Close(); err != nil {
			t.Fatalf("failed to close body: %v", err)
		}
	})

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected content-type application/json, got %s", ct)
	}

	var payload Greeting
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if payload.Message != "Hello, world-service!" {
		t.Fatalf("unexpected message: %s", payload.Message)
	}
	if time.Since(payload.Timestamp) > time.Minute {
		t.Fatalf("timestamp too old: %s", payload.Timestamp)
	}
}
