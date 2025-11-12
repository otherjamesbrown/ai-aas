// Command router is the main HTTP server for the API Router Service.
//
// Purpose:
//   This binary provides the primary entrypoint for inference requests, routing them
//   to appropriate model backends while enforcing authentication, budgets, quotas,
//   and usage tracking. It initializes core dependencies (config, telemetry, Redis,
//   Kafka) and serves HTTP requests with graceful shutdown handling.
//
// Dependencies:
//   - internal/config: Configuration loading and caching
//   - internal/telemetry: OpenTelemetry and structured logging
//   - internal/auth: API key authentication
//   - internal/routing: Backend selection and routing logic
//   - internal/limiter: Rate limiting and budget enforcement
//   - internal/usage: Usage record tracking and export
//
// Key Responsibilities:
//   - Load configuration and initialize runtime dependencies
//   - Register public API routes (/v1/inference)
//   - Register admin routes (/v1/admin/*)
//   - Register health/readiness endpoints (/v1/status/*)
//   - Serve HTTP requests on configured port
//   - Handle graceful shutdown (SIGINT/SIGTERM)
//
// Requirements Reference:
//   - specs/006-api-router-service/spec.md#US-001 (Route authenticated inference requests)
//   - specs/006-api-router-service/spec.md#NFR-004 (Service Availability)
//
// Debugging Notes:
//   - Server starts on configured HTTP port (default 8080)
//   - Readiness probe checks Redis, Kafka, and config service connectivity
//   - Graceful shutdown allows in-flight requests to complete (10s timeout)
//
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"

	"github.com/redis/go-redis/v9"

	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/api/admin"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/api/public"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/auth"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/limiter"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/routing"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/telemetry"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/usage"
)

func main() {
	ctx := context.Background()

	// Load configuration
	cfg := config.MustLoad()

	// Initialize telemetry
	telemetryCfg := telemetry.Config{
		ServiceName: cfg.ServiceName,
		Environment: cfg.Environment,
		Endpoint:    cfg.TelemetryEndpoint,
		Protocol:    cfg.TelemetryProtocol,
		Headers:     map[string]string{},
		Insecure:    cfg.TelemetryInsecure,
		LogLevel:    cfg.LogLevel,
	}

	tel := telemetry.MustInit(ctx, telemetryCfg)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tel.Shutdown(shutdownCtx); err != nil {
			tel.Logger.Error("failed to shutdown telemetry", zap.Error(err))
		}
	}()

	logger := tel.Logger
	logger.Info("starting API router service",
		zap.String("environment", cfg.Environment),
		zap.Int("port", cfg.HTTPPort),
	)

	// Initialize configuration cache and loader
	cache, err := config.NewCache(cfg.ConfigCachePath)
	if err != nil {
		logger.Fatal("failed to initialize config cache", zap.Error(err))
	}
	defer cache.Close()

	loader := config.NewLoader(cfg.ConfigServiceEndpoint, cfg.ConfigWatchEnabled, cache, logger)
	if err := loader.Load(ctx); err != nil {
		logger.Warn("failed to load initial configuration, using cache fallback", zap.Error(err))
	}

	watchCtx := context.Background()
	if err := loader.Watch(watchCtx); err != nil {
		logger.Warn("failed to start config watch", zap.Error(err))
	}
	defer loader.Stop()

	// Set up HTTP server with middleware
	router := chi.NewRouter()

	// Middleware stack
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Timeout(60 * time.Second))

	// Initialize authentication
	authenticator := auth.NewAuthenticator(logger, cfg.UserOrgServiceURL, cfg.UserOrgServiceTimeout)

	// Initialize Redis for rate limiting
	var redisClient *redis.Client
	if cfg.RateLimitRedisAddr != "" {
		redisClient = redis.NewClient(&redis.Options{
			Addr:     cfg.RateLimitRedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		})

		// Test Redis connection
		pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := redisClient.Ping(pingCtx).Err(); err != nil {
			logger.Warn("Redis unavailable, rate limiting disabled", zap.Error(err))
			redisClient = nil
		} else {
			logger.Info("Redis connected for rate limiting", zap.String("addr", cfg.RateLimitRedisAddr))
		}
	}

	// Initialize rate limiter
	var rateLimiter *limiter.RateLimiter
	if redisClient != nil {
		rateLimiter = limiter.NewRateLimiter(redisClient, logger, cfg.RateLimitDefaultRPS, cfg.RateLimitBurstSize)
		logger.Info("rate limiter initialized",
			zap.Int("default_rps", cfg.RateLimitDefaultRPS),
			zap.Int("burst_size", cfg.RateLimitBurstSize),
		)
	}

	// Initialize budget client
	budgetClient := limiter.NewBudgetClient(cfg.BudgetServiceEndpoint, cfg.BudgetServiceTimeout, logger)
	if cfg.BudgetServiceEndpoint != "" {
		logger.Info("budget client initialized", zap.String("endpoint", cfg.BudgetServiceEndpoint))
	} else {
		logger.Info("budget client using stub implementation")
	}

	// Initialize audit logger
	auditLogger := usage.NewAuditLogger(logger)

	// Initialize backend registry from config
	backendRegistry := config.NewBackendRegistry(cfg)
	logger.Info("backend registry initialized",
		zap.Strings("backends", backendRegistry.ListBackends()),
	)

	// Initialize Kafka publisher for usage records (if configured)
	var kafkaPublisher *usage.Publisher
	if cfg.KafkaBrokers != "" {
		kafkaPublisher = usage.NewPublisher(usage.PublisherConfig{
			Brokers:      parseKafkaBrokers(cfg.KafkaBrokers),
			Topic:        cfg.KafkaTopic,
			ClientID:     cfg.ServiceName,
			BatchSize:    100,
			BatchTimeout: 1 * time.Second,
			WriteTimeout: 5 * time.Second,
			RequiredAcks: 1,
		}, logger)
		logger.Info("Kafka publisher initialized", zap.String("brokers", cfg.KafkaBrokers), zap.String("topic", cfg.KafkaTopic))
	} else {
		logger.Info("Kafka publisher not configured (usage tracking disabled)")
	}

	// Build metadata (can be set at build time via environment variables)
	buildMetadata := public.BuildMetadata{
		Version:   getEnvOrDefault("VERSION", "dev"),
		Commit:    getEnvOrDefault("COMMIT_SHA", ""),
		BuildTime: getEnvOrDefault("BUILD_TIME", ""),
	}

	// Initialize status handlers
	statusHandlers := public.NewStatusHandlers(public.StatusHandlersConfig{
		RedisClient:    redisClient,
		KafkaPublisher: kafkaPublisher,
		ConfigLoader:   loader,
		BackendRegistry: backendRegistry,
		BuildMetadata:  buildMetadata,
		Logger:         logger,
		HealthTimeout:  2 * time.Second,
		ReadyTimeout:   5 * time.Second,
	})

	// Register health endpoints
	router.Get("/v1/status/healthz", statusHandlers.Healthz)
	router.Get("/v1/status/readyz", statusHandlers.Readyz)

	// Initialize backend client
	backendClient := routing.NewBackendClient(logger, 30*time.Second)

	// Initialize health monitor
	healthMonitor := routing.NewHealthMonitor(backendClient, logger, cfg.HealthCheckInterval)
	
	// Initialize routing engine
	routingEngine := routing.NewEngine(healthMonitor, backendRegistry, logger)
	
	// Initialize routing metrics
	routingMetrics, err := telemetry.NewRoutingMetrics(logger)
	if err != nil {
		logger.Warn("failed to initialize routing metrics", zap.Error(err))
		routingMetrics = nil
	}

	// Register backends with health monitor
	for _, backendID := range backendRegistry.ListBackends() {
		backendCfg, err := backendRegistry.GetBackend(backendID)
		if err == nil {
			endpoint := &routing.BackendEndpoint{
				ID:      backendCfg.ID,
				URI:     backendCfg.URI,
				Timeout: backendCfg.Timeout,
			}
			healthMonitor.RegisterBackend(backendID, endpoint)
		}
	}

	// Start health monitor
	healthMonitor.Start()
	defer healthMonitor.Stop()

	// Initialize public API handler with routing engine
	publicHandler := public.NewHandler(logger, authenticator, loader, backendClient, backendRegistry, routingEngine, routingMetrics)

	// Create tracer for middleware
	tracer := otel.Tracer("api-router-service")

	// Register middleware (order matters: body buffer -> auth -> rate limit -> budget -> handler)
	// BodyBufferMiddleware must be first to buffer request body for HMAC verification and model extraction
	router.Use(public.BodyBufferMiddleware(64 * 1024)) // 64 KB max body size
	// AuthContextMiddleware must come after body buffer for HMAC verification
	router.Use(public.AuthContextMiddleware(authenticator, logger, tracer))
	
	// Rate limit middleware (only if Redis is available)
	if rateLimiter != nil {
		router.Use(public.RateLimitMiddleware(rateLimiter, auditLogger, logger, tracer))
	} else {
		logger.Warn("rate limiting disabled (Redis unavailable)")
	}

	// Budget middleware
	router.Use(public.BudgetMiddleware(budgetClient, auditLogger, logger, tracer))

	// Register routes (handler will use auth context from middleware)
	publicHandler.RegisterRoutes(router)

	// Store references for graceful shutdown
	_ = auditLogger // TODO: Close audit logger on shutdown
	if redisClient != nil {
		defer redisClient.Close()
	}

	// Initialize admin handler
	adminHandler := admin.NewHandler(logger, loader, healthMonitor, routingEngine, backendRegistry)
	adminHandler.RegisterRoutes(router)

	// Metrics endpoint
	router.Handle("/metrics", promhttp.Handler())

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler: router,
	}

	// Start server in goroutine
	go func() {
		logger.Info("HTTP server starting", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP server failed", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	logger.Info("shutting down gracefully")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", zap.Error(err))
		os.Exit(1)
	}

	logger.Info("API router service stopped")
}

// parseKafkaBrokers parses a comma-separated list of Kafka broker addresses.
func parseKafkaBrokers(brokers string) []string {
	if brokers == "" {
		return nil
	}
	parts := strings.Split(brokers, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

// getEnvOrDefault returns the value of an environment variable or a default value.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
