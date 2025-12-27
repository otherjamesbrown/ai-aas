// Package api provides HTTP server setup and routing for the analytics service.
//
// Purpose:
//
//	This package sets up the chi router with middleware, health/readiness probes,
//	and API route registration. It provides a clean separation between server
//	configuration and handler implementation.
//
// Dependencies:
//   - github.com/go-chi/chi/v5: HTTP router
//   - github.com/prometheus/client_golang: Prometheus metrics
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/analytics-service/internal/audit"
	rbacmiddleware "github.com/otherjamesbrown/ai-aas/services/analytics-service/internal/middleware"
	"github.com/otherjamesbrown/ai-aas/services/analytics-service/internal/storage/postgres"

	sharedMiddleware "github.com/ai-aas/shared-go/middleware"
	"github.com/ai-aas/shared-go/observability"
)

// Server wraps the HTTP server and router.
type Server struct {
	router             *chi.Mux
	logger             *zap.Logger
	port               int
	authCfg            rbacmiddleware.AuthConfig
	rbacCfg            rbacmiddleware.RBACConfig
	store              *postgres.Store
	redisClient        *redis.Client
	usageHandler       *UsageHandler
	reliabilityHandler *ReliabilityHandler
	exportsHandler     *ExportsHandler
}

// Config holds server configuration.
type Config struct {
	Port         int
	Logger       *zap.Logger
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	// EnableRBAC controls whether RBAC middleware is enabled (default: true)
	EnableRBAC bool
	// MasterAdminAPIKey is used for Bearer token authentication
	MasterAdminAPIKey string
	// Dependencies for readiness checks
	Store       *postgres.Store
	RedisClient *redis.Client
}

// NewServer creates a new HTTP server with configured middleware and routes.
func NewServer(cfg Config) *Server {
	r := chi.NewRouter()

	// Setup audit logging
	auditLogger := audit.NewLogger(cfg.Logger)
	auditLogger.Setup()

	// Middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// Request context middleware (for request IDs and correlation)
	r.Use(observability.RequestContextMiddleware)

	// Request logger middleware (structured logging for all requests)
	requestLoggerConfig := sharedMiddleware.DefaultRequestLoggerConfig()
	r.Use(sharedMiddleware.RequestLogger(cfg.Logger, requestLoggerConfig))

	// Auth configuration - validates Bearer tokens
	authCfg := rbacmiddleware.AuthConfig{
		Logger:            cfg.Logger,
		MasterAdminAPIKey: cfg.MasterAdminAPIKey,
		EnableAuth:        cfg.EnableRBAC, // Auth is enabled when RBAC is enabled
	}

	// RBAC configuration - will be applied to analytics API routes
	rbacCfg := rbacmiddleware.RBACConfig{
		Logger:     cfg.Logger,
		EnableRBAC: cfg.EnableRBAC,
	}

	s := &Server{
		router:      r,
		logger:      cfg.Logger,
		port:        cfg.Port,
		authCfg:     authCfg,
		rbacCfg:     rbacCfg,
		store:       cfg.Store,
		redisClient: cfg.RedisClient,
	}

	// Health and readiness endpoints (no RBAC)
	r.Route("/analytics/v1/status", func(r chi.Router) {
		r.Get("/healthz", healthzHandler)
		r.Get("/readyz", s.readyzHandler)
	})

	// Prometheus metrics endpoint (no RBAC)
	r.Handle("/metrics", promhttp.Handler())

	// Analytics API routes will be registered via RegisterRoutes
	// RBAC middleware will be applied to each route group

	return s
}

// Router returns the chi router for route registration.
func (s *Server) Router() *chi.Mux {
	return s.router
}

// RegisterUsageRoutes registers usage API routes.
func (s *Server) RegisterUsageRoutes(handler *UsageHandler) {
	s.usageHandler = handler
}

// RegisterReliabilityRoutes registers reliability API routes.
func (s *Server) RegisterReliabilityRoutes(handler *ReliabilityHandler) {
	s.reliabilityHandler = handler
}

// RegisterExportsRoutes registers export job management API routes.
func (s *Server) RegisterExportsRoutes(handler *ExportsHandler) {
	s.exportsHandler = handler
	// Now that all handlers are registered, set up the routes
	s.setupRoutes()
}

// setupRoutes consolidates all API routes under a single /analytics/v1 path
func (s *Server) setupRoutes() {
	s.router.Route("/analytics/v1", func(r chi.Router) {
		r.Use(rbacmiddleware.Auth(s.authCfg)) // Auth: validate Bearer token, set X-Actor-* headers
		r.Use(rbacmiddleware.RBAC(s.rbacCfg)) // RBAC: check X-Actor-Roles against policy
		r.Route("/orgs/{orgId}", func(r chi.Router) {
			// Usage routes
			if s.usageHandler != nil {
				r.Get("/usage", s.usageHandler.GetOrgUsage)
				// API key usage route
				r.Get("/apikeys/{apiKeyId}/usage", s.usageHandler.GetAPIKeyUsage)
			}
			// Reliability routes
			if s.reliabilityHandler != nil {
				r.Get("/reliability", s.reliabilityHandler.GetOrgReliability)
			}
			// Export routes
			if s.exportsHandler != nil {
				r.Route("/exports", func(r chi.Router) {
					r.Post("/", s.exportsHandler.CreateExportJob)
					r.Get("/", s.exportsHandler.ListExportJobs)
					r.Get("/{jobId}", s.exportsHandler.GetExportJob)
					r.Get("/{jobId}/download", s.exportsHandler.GetExportDownloadUrl)
				})
			}
		})
	})
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// healthzHandler returns a simple health check.
func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

// readyzHandler checks readiness of dependencies.
func (s *Server) readyzHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	components := make(map[string]string)
	allHealthy := true

	// Check Postgres connectivity
	if s.store != nil && s.store.Pool() != nil {
		pgCtx, pgCancel := context.WithTimeout(ctx, 1*time.Second)
		if err := s.store.Pool().Ping(pgCtx); err != nil {
			components["postgres"] = "unhealthy"
			allHealthy = false
			s.logger.Debug("Postgres health check failed", zap.Error(err))
		} else {
			components["postgres"] = "healthy"
		}
		pgCancel()
	} else {
		components["postgres"] = "unhealthy"
		allHealthy = false
		s.logger.Debug("Postgres store not available")
	}

	// Check Redis connectivity
	if s.redisClient != nil {
		redisCtx, redisCancel := context.WithTimeout(ctx, 1*time.Second)
		if err := s.redisClient.Ping(redisCtx).Err(); err != nil {
			components["redis"] = "unhealthy"
			allHealthy = false
			s.logger.Debug("Redis health check failed", zap.Error(err))
		} else {
			components["redis"] = "healthy"
		}
		redisCancel()
	} else {
		components["redis"] = "not_configured"
		// Redis is optional (freshness cache can be disabled)
	}

	// Build response
	response := map[string]interface{}{
		"status":     "ready",
		"components": components,
		"timestamp":  time.Now().Format(time.RFC3339),
	}

	if !allHealthy {
		w.WriteHeader(http.StatusServiceUnavailable)
		response["status"] = "degraded"
	} else {
		w.WriteHeader(http.StatusOK)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		// Error already logged, status already sent
		return
	}
}
