package dataaccess

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRegistryEvaluate(t *testing.T) {
	reg := NewRegistry()
	reg.Register("ok", func(context.Context) error { return nil })
	reg.Register("fail", func(context.Context) error { return errors.New("boom") })

	result := reg.Evaluate(context.Background())
	if len(result.Checks) != 2 {
		t.Fatalf("expected 2 checks, got %d", len(result.Checks))
	}
	if !result.Checks["ok"].Healthy {
		t.Fatalf("expected ok probe to be healthy")
	}
	if result.Checks["fail"].Healthy {
		t.Fatalf("expected fail probe to be unhealthy")
	}
	if result.Checks["fail"].Error != "boom" {
		t.Fatalf("expected error to propagate")
	}
}

func TestHandlerStatusCodes(t *testing.T) {
	reg := NewRegistry()
	reg.Register("slow", func(context.Context) error { time.Sleep(5 * time.Millisecond); return nil })

	req := httptest.NewRequest("GET", "/healthz", nil)
	rr := httptest.NewRecorder()
	handler := Handler(reg)
	handler.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	reg.Register("bad", func(context.Context) error { return errors.New("uh oh") })
	req = httptest.NewRequest("GET", "/healthz", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != 503 {
		t.Fatalf("expected status 503, got %d", rr.Code)
	}
}

