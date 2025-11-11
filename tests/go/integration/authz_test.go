package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/ai-aas/shared-go/auth"
	"github.com/ai-aas/shared-go/dataaccess"
	"github.com/ai-aas/shared-go/observability"
)

func TestSecureRouteAuthorization(t *testing.T) {
	engine, err := auth.LoadPolicy(strings.NewReader(`{"rules":{"GET:/secure/resource":["admin"]}}`))
	if err != nil {
		t.Fatalf("failed to load policy: %v", err)
	}

	router := chi.NewRouter()
	router.Use(observability.RequestContextMiddleware)

	registry := dataaccess.NewRegistry()
	registry.Register("self", func(ctx context.Context) error { return nil })
	router.Get("/healthz", dataaccess.Handler(registry))

	secure := chi.NewRouter()
	secure.Use(auth.Middleware(engine, auth.HeaderExtractor))
	secure.Get("/resource", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	router.Mount("/secure", secure)

	// Denied request
	req := httptest.NewRequest(http.MethodGet, "/secure/resource", nil)
	req.Header.Set("X-Actor-Roles", "viewer")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "access denied") {
		t.Fatalf("expected error body")
	}

	// Allowed request
	req = httptest.NewRequest(http.MethodGet, "/secure/resource", nil)
	req.Header.Set("X-Actor-Roles", "admin")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}
