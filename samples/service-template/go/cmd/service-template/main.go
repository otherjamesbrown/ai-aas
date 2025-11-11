package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ai-aas/shared-go/config"
	"github.com/ai-aas/shared-go/dataaccess"
	"github.com/ai-aas/shared-go/observability"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg := config.MustLoad(ctx)

	telemetry := observability.MustInit(ctx, observability.Config{
		ServiceName: cfg.Service.Name,
		Environment: "development",
		Endpoint:    cfg.Telemetry.Endpoint,
		Protocol:    cfg.Telemetry.Protocol,
		Headers:     cfg.Telemetry.Headers,
		Insecure:    cfg.Telemetry.Insecure,
	})
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := telemetry.Shutdown(shutdownCtx); err != nil {
			log.Printf("failed to shutdown telemetry: %v", err)
		}
	}()

	registry := dataaccess.NewRegistry()
	registry.Register("self", func(ctx context.Context) error { return nil })

	var db *sql.DB
	if cfg.Database.DSN != "" {
		var err error
		db, err = dataaccess.OpenSQL(ctx, "postgres", cfg.Database)
		if err != nil {
			log.Printf("database connection failed: %v", err)
		} else {
			registry.Register("database", dataaccess.SQLProbe(db))
			defer db.Close()
		}
	}

	router := chi.NewRouter()

	router.Get("/healthz", dataaccess.Handler(registry))
	router.Get("/info", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"service": cfg.Service.Name,
			"version": "0.0.0",
		})
	})

	server := &http.Server{
		Addr:    cfg.Service.Address,
		Handler: router,
	}

	log.Printf("service-template (go) listening on %s", cfg.Service.Address)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
