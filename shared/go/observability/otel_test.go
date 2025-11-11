package observability

import (
	"context"
	"testing"
)

func TestInitValidation(t *testing.T) {
	if _, err := Init(context.Background(), Config{}); err == nil {
		t.Fatalf("expected error when endpoint missing")
	}
}

func TestBuildClientUnsupported(t *testing.T) {
	if _, err := Init(context.Background(), Config{
		ServiceName: "svc",
		Endpoint:    "collector:4317",
		Protocol:    "ws",
	}); err == nil {
		t.Fatalf("expected unsupported protocol error")
	}
}
