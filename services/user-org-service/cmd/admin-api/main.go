// Command admin-api is the main HTTP API server for the user-org service.
//
// Purpose:
//
//	This binary provides the primary REST API for user and organization management,
//	authentication, and authorization. It initializes core dependencies (Postgres,
//	Redis, OAuth provider) via bootstrap, registers authentication routes, and
//	serves HTTP requests with graceful shutdown handling.
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
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/bootstrap"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/httpapi/apikeys"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/httpapi/auth"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/httpapi/middleware"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/httpapi/orgs"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/httpapi/serviceaccounts"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/httpapi/users"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/logging"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/server"
)

func main() {
	cfg := config.MustLoad()
	logger := logging.New(cfg.ServiceName+"-admin-api", cfg.LogLevel)
	logger.Info("starting admin API",
		zap.String("env", cfg.Environment),
		zap.Int("port", cfg.HTTPPort))

	ctx := context.Background()
	runtime, err := bootstrap.Initialize(ctx, cfg)
	if err != nil {
		logger.Fatal("failed to bootstrap runtime", zap.Error(err))
	}
	logger.Info("runtime dependencies initialized")

	// Initialize IdP registry if OIDC is configured (moved here to avoid import cycles)
	baseURL := cfg.OIDCBaseURL
	if baseURL == "" {
		// Default to HTTP port if base URL not provided (for development)
		baseURL = fmt.Sprintf("http://localhost:%d", cfg.HTTPPort)
	}
	idpRegistry, err := auth.InitializeIdPProviders(ctx, baseURL, cfg)
	if err != nil {
		logger.Warn("failed to initialize IdP providers, OIDC federation disabled", zap.Error(err))
	} else if idpRegistry != nil {
		// Set IdP registry in runtime (we need to add a setter or make it accessible)
		// For now, we'll pass it to RegisterRoutes
		logger.Info("IdP providers initialized")
	}

	srv := server.New(server.Options{
		Port:        cfg.HTTPPort,
		Logger:      logger,
		ServiceName: cfg.ServiceName + "-admin-api",
		Readiness:   readinessProbe(runtime, logger),
		RegisterRoutes: func(r chi.Router) {
			// Public auth routes (no auth required)
			auth.RegisterRoutes(r, runtime, idpRegistry, logger)

			// Protected routes (require authentication)
			r.Group(func(r chi.Router) {
				// Apply auth middleware to all routes in this group
				r.Use(middleware.RequireAuth(runtime, logger))

				// Register orgs routes first - this creates /v1/orgs/{orgId} routes
				orgs.RegisterRoutes(r, runtime, logger)
				// Register users routes - these are more specific (/v1/orgs/{orgId}/invites, etc.)
				// and will match after the orgs routes
				users.RegisterRoutes(r, runtime, logger)
				// Register service account routes
				serviceaccounts.RegisterRoutes(r, runtime, logger)
				// Register API key routes
				apikeys.RegisterRoutes(r, runtime, logger)
			})
		},
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("admin API server failed", zap.Error(err))
		}
	}()

	<-ctx.Done()
	stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", zap.Error(err))
		os.Exit(1)
	}
	if err := runtime.Close(shutdownCtx); err != nil {
		logger.Error("failed to cleanly close runtime", zap.Error(err))
	}

	logger.Info("admin API stopped")
}

// readinessProbe returns a function that checks Postgres and Redis connectivity.
// Used by the HTTP server's /readyz endpoint. Redis failures are logged as warnings
// but still fail the probe (Redis is optional but if configured, must be available).
func readinessProbe(rt *bootstrap.Runtime, logger *zap.Logger) func(context.Context) error {
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
				logger.Warn("redis ping failed", zap.Error(err))
				return err
			}
		}
		return nil
	}
}
