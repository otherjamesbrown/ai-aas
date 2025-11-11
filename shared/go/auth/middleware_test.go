package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMiddlewareAllowsAuthorizedRequest(t *testing.T) {
	engine, err := LoadPolicy(strings.NewReader(`{"rules":{"GET:/secure":["admin"]}}`))
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}

	var recorded []AuditEvent
	SetAuditRecorder(func(event AuditEvent) {
		recorded = append(recorded, event)
	})

	mw := Middleware(engine, HeaderExtractor)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if actor, ok := ActorFromContext(r.Context()); !ok || actor.Subject != "alice" {
			t.Fatalf("expected actor in context")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/secure", nil)
	req.Header.Set("X-Actor-Subject", "alice")
	req.Header.Set("X-Actor-Roles", "admin,viewer")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !called {
		t.Fatalf("expected handler to be called")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 response, got %d", rr.Code)
	}
	if len(recorded) != 1 || !recorded[0].Allowed {
		t.Fatalf("expected allowed audit event")
	}
}

func TestMiddlewareBlocksUnauthorizedRequest(t *testing.T) {
	engine, err := LoadPolicy(strings.NewReader(`{"rules":{"POST:/secure":["admin"]}}`))
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}

	var recorded []AuditEvent
	SetAuditRecorder(func(event AuditEvent) {
		recorded = append(recorded, event)
	})

	mw := Middleware(engine, HeaderExtractor)
	handler := mw(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatalf("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodPost, "/secure", nil)
	req.Header.Set("X-Actor-Subject", "bob")
	req.Header.Set("X-Actor-Roles", "viewer")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 response, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "access denied") {
		t.Fatalf("expected error response body, got %s", rr.Body.String())
	}
	if len(recorded) != 1 || recorded[0].Allowed {
		t.Fatalf("expected denied audit event")
	}
}
