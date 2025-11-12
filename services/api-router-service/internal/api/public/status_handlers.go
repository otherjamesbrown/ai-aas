// Package public provides public API handlers for the API Router Service.
//
// Purpose:
//   This file implements health and readiness endpoint handlers with component-level
//   health checks for operational visibility.
//
// Key Responsibilities:
//   - Health endpoint (/v1/status/healthz) - Basic liveness check
//   - Readiness endpoint (/v1/status/readyz) - Component-level readiness checks
//   - Component health checks (Redis, Kafka, Config Service, Backend Registry)
//   - Build metadata injection
//   - Degraded state handling
//
// Requirements Reference:
//   - specs/006-api-router-service/spec.md#US-005 (Operational visibility and reliability)
//   - specs/006-api-router-service/spec.md#FR-010 (Health, readiness, and diagnostics endpoints)
//
package public

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/usage"
)

// BuildMetadata holds build-time information.
type BuildMetadata struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
}

// StatusHandlers provides health and readiness endpoint handlers.
type StatusHandlers struct {
	redisClient    *redis.Client
	kafkaPublisher *usage.Publisher
	configLoader   *config.Loader
	backendRegistry *config.BackendRegistry
	buildMetadata  BuildMetadata
	logger         *zap.Logger
	healthTimeout  time.Duration
	readyTimeout   time.Duration
}

// StatusHandlersConfig configures the status handlers.
type StatusHandlersConfig struct {
	RedisClient    *redis.Client
	KafkaPublisher *usage.Publisher
	ConfigLoader   *config.Loader
	BackendRegistry *config.BackendRegistry
	BuildMetadata  BuildMetadata
	Logger         *zap.Logger
	HealthTimeout  time.Duration
	ReadyTimeout   time.Duration
}

// NewStatusHandlers creates a new status handlers instance.
func NewStatusHandlers(cfg StatusHandlersConfig) *StatusHandlers {
	if cfg.Logger == nil {
		cfg.Logger = zap.NewNop()
	}
	if cfg.HealthTimeout == 0 {
		cfg.HealthTimeout = 1 * time.Second
	}
	if cfg.ReadyTimeout == 0 {
		cfg.ReadyTimeout = 5 * time.Second
	}

	return &StatusHandlers{
		redisClient:    cfg.RedisClient,
		kafkaPublisher: cfg.KafkaPublisher,
		configLoader:   cfg.ConfigLoader,
		backendRegistry: cfg.BackendRegistry,
		buildMetadata:  cfg.BuildMetadata,
		logger:         cfg.Logger,
		healthTimeout:  cfg.HealthTimeout,
		readyTimeout:   cfg.ReadyTimeout,
	}
}

// HealthResponse represents the health endpoint response.
type HealthResponse struct {
	Status    string        `json:"status"`
	Build     *BuildMetadata `json:"build,omitempty"`
	Timestamp string        `json:"timestamp,omitempty"`
}

// ReadinessResponse represents the readiness endpoint response.
type ReadinessResponse struct {
	Status     string                 `json:"status"`
	Components map[string]string      `json:"components,omitempty"`
	Build      *BuildMetadata         `json:"build,omitempty"`
	Timestamp  string                 `json:"timestamp"`
}

// Healthz handles GET /v1/status/healthz - Basic liveness check.
func (h *StatusHandlers) Healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().Format(time.RFC3339),
	}

	// Include build metadata if available
	if h.buildMetadata.Version != "" {
		response.Build = &h.buildMetadata
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("failed to encode health response", zap.Error(err))
	}
}

// Readyz handles GET /v1/status/readyz - Readiness check with component probes.
func (h *StatusHandlers) Readyz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), h.readyTimeout)
	defer cancel()

	components := make(map[string]string)
	allHealthy := true

	// Check Redis connectivity
	if h.redisClient != nil {
		redisCtx, redisCancel := context.WithTimeout(ctx, h.healthTimeout)
		if err := h.redisClient.Ping(redisCtx).Err(); err != nil {
			components["redis"] = "unhealthy"
			allHealthy = false
			h.logger.Debug("Redis health check failed", zap.Error(err))
		} else {
			components["redis"] = "healthy"
		}
		redisCancel()
	} else {
		components["redis"] = "not_configured"
		// Redis is optional for readiness (rate limiting can be disabled)
	}

	// Check Kafka connectivity
	if h.kafkaPublisher != nil {
		if err := h.kafkaPublisher.Health(ctx); err != nil {
			components["kafka"] = "unhealthy"
			allHealthy = false
			h.logger.Debug("Kafka health check failed", zap.Error(err))
		} else {
			components["kafka"] = "healthy"
		}
	} else {
		components["kafka"] = "not_configured"
		// Kafka is optional for readiness (audit logging can be disabled)
	}

	// Check Config Service (etcd) connectivity
	if h.configLoader != nil {
		if err := h.configLoader.Health(ctx); err != nil {
			components["config_service"] = "unhealthy"
			// Config Service failure is not critical if cache is available
			// We mark as degraded but don't fail readiness
			h.logger.Debug("Config Service health check failed", zap.Error(err))
		} else {
			components["config_service"] = "healthy"
		}
	} else {
		components["config_service"] = "unhealthy"
		allHealthy = false
		h.logger.Debug("Config loader not available")
	}

	// Check Backend Registry
	if h.backendRegistry != nil {
		backends := h.backendRegistry.ListBackends()
		if len(backends) == 0 {
			components["backend_registry"] = "unhealthy"
			allHealthy = false
			h.logger.Debug("Backend registry is empty")
		} else {
			components["backend_registry"] = "healthy"
		}
	} else {
		components["backend_registry"] = "unhealthy"
		allHealthy = false
		h.logger.Debug("Backend registry not available")
	}

	// Build metadata
	var build *BuildMetadata
	if h.buildMetadata.Version != "" {
		build = &h.buildMetadata
	}

	response := ReadinessResponse{
		Status:     "ready",
		Components: components,
		Build:      build,
		Timestamp:  time.Now().Format(time.RFC3339),
	}

	if !allHealthy {
		w.WriteHeader(http.StatusServiceUnavailable)
		response.Status = "degraded"
		h.logger.Warn("Readiness check failed - service degraded",
			zap.Any("components", components),
		)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("failed to encode readiness response", zap.Error(err))
	}
}

