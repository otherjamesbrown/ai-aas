package api

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/api/handlers"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/api/middleware"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/repository"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/service"
	"go.uber.org/zap"
)

// NewRouter creates and configures the HTTP router
func NewRouter(cfg *config.Config, db *repository.DB, logger *zap.Logger) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Recoverer)
	r.Use(middleware.RequestLogging(logger))

	// Rate limiter
	rateLimiter := middleware.NewRateLimiter(cfg.RateLimitPerMin)

	// Create handlers
	healthHandler := handlers.NewHealthHandler(db, cfg.Version)

	// Public endpoints (no auth required)
	r.Get("/healthz", healthHandler.Healthz)
	r.Get("/readyz", healthHandler.Readyz)
	r.Handle("/metrics", handlers.MetricsHandler())

	// API key validator (placeholder - will be implemented with repository)
	validator := &placeholderValidator{}

	// Protected API routes
	r.Route("/v1", func(r chi.Router) {
		r.Use(middleware.Auth(validator, logger))
		r.Use(middleware.RateLimit(rateLimiter))

		// Model registry routes (Phase 3)
		r.Route("/registry/models", func(r chi.Router) {
			modelRepo := repository.NewModelRepository(db)
			modelSvc := service.NewModelRegistryService(modelRepo, logger)
			modelHandler := handlers.NewModelHandler(modelSvc, logger)

			r.Post("/", modelHandler.Register)
			r.Get("/", modelHandler.List)
			r.Get("/{model_name}", modelHandler.Get)
			r.Patch("/{model_name}", modelHandler.Update)
			r.Delete("/{model_name}", modelHandler.Delete)
		})

		// Organization routes (Phase 4)
		r.Route("/organizations", func(r chi.Router) {
			orgRepo := repository.NewOrganizationRepository(db)
			orgSvc := service.NewOrganizationService(orgRepo, logger)
			orgHandler := handlers.NewOrganizationHandler(orgSvc, logger)

			r.Post("/", orgHandler.Create)
			r.Get("/", orgHandler.List)
			r.Get("/{org_id}", orgHandler.Get)
			r.Patch("/{org_id}", orgHandler.Update)
		})

		// Audit log routes (Phase 5)
		r.Route("/audit-logs", func(r chi.Router) {
			auditRepo := repository.NewAuditRepository(db)
			auditSvc := service.NewAuditService(auditRepo, logger)
			auditHandler := handlers.NewAuditHandler(auditSvc, logger)

			r.Get("/", auditHandler.List)
		})

		// Routing policy routes (Phase 6)
		r.Route("/routing/policies", func(r chi.Router) {
			policyRepo := repository.NewPolicyRepository(db)
			modelRepo := repository.NewModelRepository(db)
			policySvc := service.NewPolicyService(policyRepo, modelRepo, logger)
			policyHandler := handlers.NewPolicyHandler(policySvc, logger)

			r.Post("/", policyHandler.Create)
			r.Get("/", policyHandler.List)
			r.Post("/validate", policyHandler.Validate)
			r.Get("/sync", policyHandler.Sync)
			r.Get("/{policy_id}", policyHandler.Get)
			r.Patch("/{policy_id}", policyHandler.Update)
			r.Delete("/{policy_id}", policyHandler.Delete)
			r.Post("/{policy_id}/activate", policyHandler.Activate)
			r.Post("/{policy_id}/deactivate", policyHandler.Deactivate)
		})
	})

	return r
}

// placeholderValidator is a temporary implementation until repository is complete
type placeholderValidator struct{}

func (v *placeholderValidator) ValidateKey(ctx context.Context, key string) (string, bool, error) {
	// TODO: Implement actual validation against database
	// For now, accept any non-empty key
	if key != "" {
		return "placeholder-key-id", true, nil
	}
	return "", false, nil
}

