package observability

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestContextMiddleware(t *testing.T) {
	handler := RequestContextMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := RequestIDFromContext(r.Context())
		if !ok || id == "" {
			t.Fatalf("expected request id in context")
		}
		if r.Header.Get("X-Request-ID") == "" {
			t.Fatalf("expected request id header on request")
		}
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("X-Request-ID") == "" {
		t.Fatalf("expected response header set")
	}
}
