// Package public provides public API handlers for the API Router Service.
//
// Purpose:
//
//	This file implements health and readiness endpoint handlers with component-level
//	health checks for operational visibility.
//
// Key Responsibilities:
//   - Health endpoint (/v1/status/healthz) - Basic liveness check
//   - Readiness endpoint (/v1/status/readyz) - Component-level readiness checks
//   - Component health checks (Redis, Kafka, Config Service, Backend Registry)
//   - Build metadata injection
//   - Degraded state handling
//   - Circuit breakers for slow dependencies
//   - Health check result caching
//
// Requirements Reference:
//   - specs/006-api-router-service/spec.md#US-005 (Operational visibility and reliability)
//   - specs/006-api-router-service/spec.md#FR-010 (Health, readiness, and diagnostics endpoints)
package public

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
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

// ComponentHealth represents the health status of a component.
type ComponentHealth struct {
	Status    string    `json:"status"`
	LastCheck time.Time `json:"last_check"`
	Error     string    `json:"error,omitempty"`
}

// CircuitBreaker implements a simple circuit breaker pattern.
type CircuitBreaker struct {
	mu           sync.RWMutex
	failures     int
	lastFailure  time.Time
	threshold    int           // Number of failures before opening
	resetTimeout time.Duration // Time to wait before half-open
	isOpen       bool
}

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker(threshold int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		threshold:    threshold,
		resetTimeout: resetTimeout,
	}
}

// Allow checks if the circuit breaker allows the request.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if !cb.isOpen {
		return true
	}

	// Check if we should try half-open
	if time.Since(cb.lastFailure) > cb.resetTimeout {
		return true
	}

	return false
}

// RecordSuccess records a successful call and resets the circuit.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures = 0
	cb.isOpen = false
}

// RecordFailure records a failed call and potentially opens the circuit.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures++
	cb.lastFailure = time.Now()
	if cb.failures >= cb.threshold {
		cb.isOpen = true
	}
}

// IsOpen returns whether the circuit is open.
func (cb *CircuitBreaker) IsOpen() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.isOpen
}

// HealthCache caches health check results to avoid repeated checks.
type HealthCache struct {
	mu      sync.RWMutex
	results map[string]ComponentHealth
	ttl     time.Duration
}

// NewHealthCache creates a new health cache.
func NewHealthCache(ttl time.Duration) *HealthCache {
	return &HealthCache{
		results: make(map[string]ComponentHealth),
		ttl:     ttl,
	}
}

// Get retrieves a cached health result if still valid.
func (c *HealthCache) Get(component string) (ComponentHealth, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result, exists := c.results[component]
	if !exists {
		return ComponentHealth{}, false
	}

	if time.Since(result.LastCheck) > c.ttl {
		return ComponentHealth{}, false
	}

	return result, true
}

// Set stores a health result in the cache.
func (c *HealthCache) Set(component string, health ComponentHealth) {
	c.mu.Lock()
	defer c.mu.Unlock()
	health.LastCheck = time.Now()
	c.results[component] = health
}

// StatusHandlers provides health and readiness endpoint handlers.
type StatusHandlers struct {
	redisClient       *redis.Client
	kafkaPublisher    *usage.Publisher
	configLoader      *config.Loader
	backendRegistry   *config.BackendRegistry
	userOrgServiceURL string
	httpClient        *http.Client
	buildMetadata     BuildMetadata
	logger            *zap.Logger
	healthTimeout     time.Duration
	readyTimeout      time.Duration

	// Circuit breakers for slow dependencies
	etcdBreaker    *CircuitBreaker
	userOrgBreaker *CircuitBreaker
	redisBreaker   *CircuitBreaker

	// Health check cache
	healthCache *HealthCache
}

// StatusHandlersConfig configures the status handlers.
type StatusHandlersConfig struct {
	RedisClient       *redis.Client
	KafkaPublisher    *usage.Publisher
	ConfigLoader      *config.Loader
	BackendRegistry   *config.BackendRegistry
	UserOrgServiceURL string
	BuildMetadata     BuildMetadata
	Logger            *zap.Logger
	HealthTimeout     time.Duration
	ReadyTimeout      time.Duration
}

// NewStatusHandlers creates a new status handlers instance.
func NewStatusHandlers(cfg StatusHandlersConfig) *StatusHandlers {
	if cfg.Logger == nil {
		cfg.Logger = zap.NewNop()
	}
	if cfg.HealthTimeout == 0 {
		cfg.HealthTimeout = 2 * time.Second
	}
	if cfg.ReadyTimeout == 0 {
		cfg.ReadyTimeout = 8 * time.Second
	}

	return &StatusHandlers{
		redisClient:       cfg.RedisClient,
		kafkaPublisher:    cfg.KafkaPublisher,
		configLoader:      cfg.ConfigLoader,
		backendRegistry:   cfg.BackendRegistry,
		userOrgServiceURL: cfg.UserOrgServiceURL,
		httpClient: &http.Client{
			Timeout: cfg.HealthTimeout,
		},
		buildMetadata: cfg.BuildMetadata,
		logger:        cfg.Logger,
		healthTimeout: cfg.HealthTimeout,
		readyTimeout:  cfg.ReadyTimeout,

		// Circuit breakers: open after 3 failures, try again after 30s
		etcdBreaker:    NewCircuitBreaker(3, 30*time.Second),
		userOrgBreaker: NewCircuitBreaker(3, 30*time.Second),
		redisBreaker:   NewCircuitBreaker(3, 30*time.Second),

		// Cache health results for 5 seconds
		healthCache: NewHealthCache(5 * time.Second),
	}
}

// HealthResponse represents the health endpoint response.
type HealthResponse struct {
	Status    string         `json:"status"`
	Build     *BuildMetadata `json:"build,omitempty"`
	Timestamp string         `json:"timestamp,omitempty"`
}

// ReadinessResponse represents the readiness endpoint response.
type ReadinessResponse struct {
	Status     string            `json:"status"`
	Components map[string]string `json:"components,omitempty"`
	Build      *BuildMetadata    `json:"build,omitempty"`
	Timestamp  string            `json:"timestamp"`
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
// Health checks run in parallel for faster response times.
func (h *StatusHandlers) Readyz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), h.readyTimeout)
	defer cancel()

	// Run all health checks in parallel
	components := h.runParallelHealthChecks(ctx)

	// Determine overall health
	// Critical components: backend_registry
	// Important but degradable: redis, user_org_service
	// Optional: kafka, config_service (if cache available)
	allHealthy := true
	for name, status := range components {
		switch name {
		case "backend_registry":
			// Backend registry is critical - must be healthy
			if status != "healthy" {
				allHealthy = false
			}
		case "redis":
			// Redis is important for rate limiting but service can work without it
			// Don't fail readiness for redis issues
		case "user_org_service":
			// User-org service is important but we can serve cached auth if degraded
			// Don't fail readiness for temporary connectivity issues
		case "kafka":
			// Kafka is optional - audit logging can be disabled
		case "config_service":
			// Config service (etcd) is optional if we have cached config
		}
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
		response.Status = "not_ready"
		h.logger.Warn("Readiness check failed - critical component unhealthy",
			zap.Any("components", components),
		)
	} else {
		// Check if any components are degraded
		hasDegraded := false
		for _, status := range components {
			if status == "unhealthy" || status == "degraded" || status == "circuit_open" {
				hasDegraded = true
				break
			}
		}
		if hasDegraded {
			response.Status = "degraded"
			h.logger.Debug("Service ready but degraded",
				zap.Any("components", components),
			)
		}
		w.WriteHeader(http.StatusOK)
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("failed to encode readiness response", zap.Error(err))
	}
}

// runParallelHealthChecks runs all health checks concurrently.
func (h *StatusHandlers) runParallelHealthChecks(ctx context.Context) map[string]string {
	components := make(map[string]string)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Helper to safely set component status
	setStatus := func(name, status string) {
		mu.Lock()
		components[name] = status
		mu.Unlock()
	}

	// Check Redis
	wg.Add(1)
	go func() {
		defer wg.Done()
		status := h.checkRedis(ctx)
		setStatus("redis", status)
	}()

	// Check Kafka
	wg.Add(1)
	go func() {
		defer wg.Done()
		status := h.checkKafka(ctx)
		setStatus("kafka", status)
	}()

	// Check Config Service (etcd)
	wg.Add(1)
	go func() {
		defer wg.Done()
		status := h.checkConfigService(ctx)
		setStatus("config_service", status)
	}()

	// Check Backend Registry
	wg.Add(1)
	go func() {
		defer wg.Done()
		status := h.checkBackendRegistry()
		setStatus("backend_registry", status)
	}()

	// Check User-Org Service
	wg.Add(1)
	go func() {
		defer wg.Done()
		status := h.checkUserOrgService(ctx)
		setStatus("user_org_service", status)
	}()

	// Wait for all checks to complete
	wg.Wait()

	return components
}

// checkRedis checks Redis connectivity with circuit breaker and caching.
func (h *StatusHandlers) checkRedis(ctx context.Context) string {
	if h.redisClient == nil {
		return "not_configured"
	}

	// Check cache first
	if cached, ok := h.healthCache.Get("redis"); ok {
		return cached.Status
	}

	// Check circuit breaker
	if !h.redisBreaker.Allow() {
		return "circuit_open"
	}

	// Perform actual check with timeout
	checkCtx, cancel := context.WithTimeout(ctx, h.healthTimeout)
	defer cancel()

	if err := h.redisClient.Ping(checkCtx).Err(); err != nil {
		h.redisBreaker.RecordFailure()
		h.healthCache.Set("redis", ComponentHealth{Status: "unhealthy", Error: err.Error()})
		h.logger.Debug("Redis health check failed", zap.Error(err))
		return "unhealthy"
	}

	h.redisBreaker.RecordSuccess()
	h.healthCache.Set("redis", ComponentHealth{Status: "healthy"})
	return "healthy"
}

// checkKafka checks Kafka connectivity.
func (h *StatusHandlers) checkKafka(ctx context.Context) string {
	if h.kafkaPublisher == nil {
		return "not_configured"
	}

	// Check cache first
	if cached, ok := h.healthCache.Get("kafka"); ok {
		return cached.Status
	}

	// Perform actual check with timeout
	checkCtx, cancel := context.WithTimeout(ctx, h.healthTimeout)
	defer cancel()

	if err := h.kafkaPublisher.Health(checkCtx); err != nil {
		h.healthCache.Set("kafka", ComponentHealth{Status: "unhealthy", Error: err.Error()})
		h.logger.Debug("Kafka health check failed", zap.Error(err))
		return "unhealthy"
	}

	h.healthCache.Set("kafka", ComponentHealth{Status: "healthy"})
	return "healthy"
}

// checkConfigService checks etcd/config service connectivity with circuit breaker.
func (h *StatusHandlers) checkConfigService(ctx context.Context) string {
	if h.configLoader == nil {
		return "not_configured"
	}

	// Check cache first
	if cached, ok := h.healthCache.Get("config_service"); ok {
		return cached.Status
	}

	// Check circuit breaker
	if !h.etcdBreaker.Allow() {
		return "circuit_open"
	}

	// Perform actual check with timeout
	checkCtx, cancel := context.WithTimeout(ctx, h.healthTimeout)
	defer cancel()

	if err := h.configLoader.Health(checkCtx); err != nil {
		h.etcdBreaker.RecordFailure()
		h.healthCache.Set("config_service", ComponentHealth{Status: "unhealthy", Error: err.Error()})
		h.logger.Debug("Config Service health check failed", zap.Error(err))
		return "unhealthy"
	}

	h.etcdBreaker.RecordSuccess()
	h.healthCache.Set("config_service", ComponentHealth{Status: "healthy"})
	return "healthy"
}

// checkBackendRegistry checks if backend registry has backends configured.
func (h *StatusHandlers) checkBackendRegistry() string {
	if h.backendRegistry == nil {
		return "not_configured"
	}

	backends := h.backendRegistry.ListBackends()
	if len(backends) == 0 {
		h.logger.Debug("Backend registry is empty")
		return "unhealthy"
	}

	return "healthy"
}

// checkUserOrgService checks user-org-service connectivity with circuit breaker.
func (h *StatusHandlers) checkUserOrgService(ctx context.Context) string {
	if h.userOrgServiceURL == "" {
		return "not_configured"
	}

	// Check cache first
	if cached, ok := h.healthCache.Get("user_org_service"); ok {
		return cached.Status
	}

	// Check circuit breaker
	if !h.userOrgBreaker.Allow() {
		return "circuit_open"
	}

	// Perform actual check with timeout
	checkCtx, cancel := context.WithTimeout(ctx, h.healthTimeout)
	defer cancel()

	userOrgHealthURL := h.userOrgServiceURL + "/healthz"
	req, err := http.NewRequestWithContext(checkCtx, http.MethodGet, userOrgHealthURL, nil)
	if err != nil {
		h.userOrgBreaker.RecordFailure()
		h.healthCache.Set("user_org_service", ComponentHealth{Status: "unhealthy", Error: err.Error()})
		h.logger.Debug("Failed to create user-org-service health request", zap.Error(err))
		return "unhealthy"
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		h.userOrgBreaker.RecordFailure()
		h.healthCache.Set("user_org_service", ComponentHealth{Status: "unhealthy", Error: err.Error()})
		h.logger.Debug("user-org-service health check failed", zap.Error(err))
		return "unhealthy"
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		h.userOrgBreaker.RecordSuccess()
		h.healthCache.Set("user_org_service", ComponentHealth{Status: "healthy"})
		return "healthy"
	}

	h.healthCache.Set("user_org_service", ComponentHealth{Status: "degraded"})
	h.logger.Debug("user-org-service returned non-OK status",
		zap.Int("status_code", resp.StatusCode))
	return "degraded"
}
