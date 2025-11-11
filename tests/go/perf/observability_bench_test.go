package perf

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/ai-aas/shared-go/config"
	"github.com/ai-aas/shared-go/observability"
)

func BenchmarkRequestContextMiddleware(b *testing.B) {
	router := chi.NewRouter()
	router.Use(observability.RequestContextMiddleware)
	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req.Clone(req.Context()))
		if rr.Code != http.StatusOK {
			b.Fatalf("unexpected status: %d", rr.Code)
		}
	}
}

func BenchmarkTelemetryInit(b *testing.B) {
	cfg := observability.Config{
		ServiceName: "benchmark",
		Environment: "test",
		Endpoint:    "127.0.0.1:43199",
		Protocol:    "http",
		Insecure:    true,
	}

	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		provider, err := observability.Init(ctx, cfg)
		if err != nil {
			b.Fatalf("init failed: %v", err)
		}
		_ = provider.Shutdown(ctx)
	}
}

func BenchmarkConfigLoad(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := config.Load(context.Background()); err != nil {
			b.Fatalf("load failed: %v", err)
		}
	}
}

