// Command reconciler is a background worker that reconciles declarative GitOps
// configuration with the operational database state.
//
// Purpose:
//
//	This binary runs as a separate process from admin-api, handling declarative
//	configuration reconciliation, drift detection, and conflict resolution.
//	It shares the same runtime dependencies (Postgres, Redis, OAuth) via bootstrap
//	but serves on a different port (HTTP_PORT + 1) to avoid conflicts.
//
// Dependencies:
//   - internal/bootstrap: Runtime initialization (shared with admin-api)
//   - internal/config: Configuration from environment variables
//   - internal/server: HTTP server for health/readiness endpoints
//   - internal/logging: Structured logging setup
//
// Key Responsibilities:
//   - Initialize runtime dependencies (Postgres, Redis, OAuth provider)
//   - Run background reconciliation worker (currently stub)
//   - Expose health/readiness endpoints on separate port
//   - Handle graceful shutdown
//
// Requirements Reference:
//   - specs/005-user-org-service/spec.md#US-003 (Declarative Management)
//   - specs/005-user-org-service/spec.md#FR-010 (Drift Detection)
//
// Debugging Notes:
//   - Server runs on HTTP_PORT + 1 (default 8082) to avoid conflicts with admin-api
//   - Worker loop is currently a stub (TODO: implement reconciliation logic)
//   - Uses same bootstrap.Initialize as admin-api for consistency
//   - Readiness probe uses runtime.ReadinessProbe (checks Postgres/Redis)
//
// Thread Safety:
//   - Main goroutine handles shutdown signals
//   - Worker goroutine runs reconciliation loop
//   - HTTP server handles concurrent health checks
//
// Error Handling:
//   - Bootstrap failures log fatal and exit
//   - Server errors log fatal and exit
//   - Shutdown errors log error but continue cleanup
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/bootstrap"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/logging"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/server"
)

func main() {
	cfg := config.MustLoad()
	logger := logging.New(cfg.ServiceName+"-reconciler", cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	runtime, err := bootstrap.Initialize(ctx, cfg)
	if err != nil {
		logger.Fatal("failed to bootstrap runtime", zap.Error(err))
	}
	defer func() {
		if err := runtime.Close(ctx); err != nil {
			logger.Error("failed to close runtime resources", zap.Error(err))
		}
	}()

	// Reconciler typically exposes health on a different port to avoid conflicts with admin API.
	port := cfg.HTTPPort + 1
	logger.Info("starting reconciler",
		zap.String("env", cfg.Environment),
		zap.Int("port", port))

	srv := server.New(server.Options{
		Port:        port,
		Logger:      logger,
		ServiceName: cfg.ServiceName + "-reconciler",
		Readiness:   runtime.ReadinessProbe,
	})

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("reconciler server failed", zap.Error(err))
		}
	}()

	// Placeholder worker loop - to be replaced with reconciliation job processing.
	go runWorker(ctx, logger, runtime)

	<-ctx.Done()
	stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", zap.Error(err))
		os.Exit(1)
	}

	logger.Info("reconciler stopped")
}

func runWorker(ctx context.Context, logger *zap.Logger, rt *bootstrap.Runtime) {
	logger.Info("reconciler worker started (stub)")
	// TODO: Use rt.Postgres, rt.OAuth2Provider, etc. for reconciliation logic
	<-ctx.Done()
	logger.Info("reconciler worker stopping")
}
