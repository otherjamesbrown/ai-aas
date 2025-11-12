// Command admin-api is the main HTTP API server for the user-org service.
//
// Purpose:
//   This binary provides the primary REST API for user and organization management,
//   authentication, and authorization. It initializes core dependencies (Postgres,
//   Redis, OAuth provider) via bootstrap, registers authentication routes, and
//   serves HTTP requests with graceful shutdown handling.
//
// Dependencies:
//   - internal/bootstrap: Runtime initialization and lifecycle management
//   - internal/config: Configuration from environment variables
//   - internal/httpapi/auth: Authentication endpoint handlers
//   - internal/server: HTTP server with health/readiness endpoints
//   - internal/logging: Structured logging setup
//
// Key Responsibilities:
//   - Load configuration and initialize runtime dependencies
//   - Register authentication routes (/v1/auth/login, /refresh, /logout)
//   - Serve HTTP requests on configured port
//   - Handle graceful shutdown (SIGINT/SIGTERM) with 10s timeout
//   - Expose health/readiness endpoints for Kubernetes
//
// Requirements Reference:
//   - specs/005-user-org-service/spec.md#US-001 (User Authentication)
//   - specs/005-user-org-service/spec.md#NFR-001 (Service Availability)
//   - specs/005-user-org-service/contracts/user-org-service.openapi.yaml
//
// Debugging Notes:
//   - Server starts on HTTP_PORT (default 8081)
//   - Readiness probe checks Postgres and Redis connectivity
//   - Graceful shutdown allows in-flight requests to complete (10s timeout)
//   - Runtime.Close() releases Postgres pool and Redis connections
//   - Logs include service name, environment, and port on startup
//
// Thread Safety:
//   - Main goroutine handles shutdown signals
//   - HTTP server handles concurrent requests
//   - Runtime dependencies are safe for concurrent use
//
// Error Handling:
//   - Configuration errors exit with code 1
//   - Bootstrap failures log fatal and exit
//   - Server errors log fatal and exit
//   - Shutdown errors log warning but don't exit
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/bootstrap"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/httpapi/auth"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/httpapi/orgs"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/httpapi/users"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/logging"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/server"
)

func main() {
	cfg := config.MustLoad()
	logger := logging.New(cfg.ServiceName+"-admin-api", cfg.LogLevel)
	logger.Info().
		Str("env", cfg.Environment).
		Int("port", cfg.HTTPPort).
		Msg("starting admin API")

	ctx := context.Background()
	runtime, err := bootstrap.Initialize(ctx, cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to bootstrap runtime")
	}
	logger.Info().Msg("runtime dependencies initialized")

	srv := server.New(server.Options{
		Port:        cfg.HTTPPort,
		Logger:      logger,
		ServiceName: cfg.ServiceName + "-admin-api",
		Readiness:   readinessProbe(runtime, logger),
		RegisterRoutes: func(r chi.Router) {
			auth.RegisterRoutes(r, runtime)
			// Register orgs routes first - this creates /v1/orgs/{orgId} routes
			orgs.RegisterRoutes(r, runtime, logger)
			// Register users routes - these are more specific (/v1/orgs/{orgId}/invites, etc.)
			// and will match after the orgs routes
			users.RegisterRoutes(r, runtime, logger)
		},
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("admin API server failed")
		}
	}()

	<-ctx.Done()
	stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("graceful shutdown failed")
		os.Exit(1)
	}
	if err := runtime.Close(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("failed to cleanly close runtime")
	}

	logger.Info().Msg("admin API stopped")
}

// readinessProbe returns a function that checks Postgres and Redis connectivity.
// Used by the HTTP server's /readyz endpoint. Redis failures are logged as warnings
// but still fail the probe (Redis is optional but if configured, must be available).
func readinessProbe(rt *bootstrap.Runtime, logger zerolog.Logger) func(context.Context) error {
	return func(ctx context.Context) error {
		if rt == nil {
			return nil
		}
		if rt.Postgres != nil && rt.Postgres.Pool() != nil {
			if err := rt.Postgres.Pool().Ping(ctx); err != nil {
				return err
			}
		}
		if rt.Redis != nil {
			if err := rt.Redis.Ping(ctx).Err(); err != nil {
				logger.Warn().Err(err).Msg("redis ping failed")
				return err
			}
		}
		return nil
	}
}
