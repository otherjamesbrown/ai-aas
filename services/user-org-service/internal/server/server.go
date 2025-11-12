package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
)

// Options configure the HTTP server instance.
type Options struct {
	Port        int
	Logger      zerolog.Logger
	ServiceName string
	Readiness   func(context.Context) error
	RegisterRoutes func(chi.Router)
}

// New constructs an http.Server pre-configured with health and readiness routes.
func New(opts Options) *http.Server {
	if opts.Readiness == nil {
		opts.Readiness = func(context.Context) error { return nil }
	}

	router := chi.NewRouter()

	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	router.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := opts.Readiness(ctx); err != nil {
			opts.Logger.Warn().Err(err).Msg("readiness check failed")
			http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	})

	// Prometheus metrics endpoint
	router.Get("/metrics", promhttp.Handler().ServeHTTP)

	if opts.RegisterRoutes != nil {
		opts.RegisterRoutes(router)
	}

	addr := fmt.Sprintf(":%d", opts.Port)
	return &http.Server{
		Addr:    addr,
		Handler: router,
	}
}
