// Command analytics-service is the main HTTP server for the Analytics Service.
//
// Purpose:
//
//	This binary provides the primary entrypoint for analytics ingestion, aggregation,
//	and querying. It initializes core dependencies (Postgres, Redis, RabbitMQ, S3)
//	and serves HTTP requests with graceful shutdown handling.
//
// Dependencies:
//   - internal/config: Configuration loading and validation
//   - internal/api: HTTP server with health/readiness endpoints
//   - internal/ingestion: RabbitMQ consumer for usage events
//   - internal/aggregation: Rollup workers and freshness tracking
//   - internal/exports: CSV export generation and S3 delivery
//   - internal/freshness: Redis-backed freshness cache
//
// Key Responsibilities:
//   - Load configuration and initialize runtime dependencies
//   - Register analytics API routes (/analytics/v1/*)
//   - Register health/readiness endpoints (/analytics/v1/status/*)
//   - Start background workers for ingestion and aggregation
//   - Serve HTTP requests on configured port
//   - Handle graceful shutdown (SIGINT/SIGTERM)
//
// Requirements Reference:
//   - specs/007-analytics-service/spec.md#US-001 (Org-level usage and spend visibility)
//   - specs/007-analytics-service/spec.md#US-002 (Reliability and error insights)
//   - specs/007-analytics-service/spec.md#US-003 (Finance-friendly reporting)
//
// Debugging Notes:
//   - Server starts on configured HTTP port (default 8084)
//   - Readiness probe checks Postgres, Redis, and RabbitMQ connectivity
//   - Graceful shutdown allows in-flight requests to complete (10s timeout)
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/analytics-service/internal/aggregation"
	"github.com/otherjamesbrown/ai-aas/services/analytics-service/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/analytics-service/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/analytics-service/internal/exports"
	"github.com/otherjamesbrown/ai-aas/services/analytics-service/internal/freshness"
	"github.com/otherjamesbrown/ai-aas/services/analytics-service/internal/ingestion"
	"github.com/otherjamesbrown/ai-aas/services/analytics-service/internal/observability"
	"github.com/otherjamesbrown/ai-aas/services/analytics-service/internal/storage/postgres"
)

// testS3Connection verifies S3 bucket accessibility by performing a HEAD request.
func testS3Connection(ctx context.Context, s3Delivery *exports.S3Delivery, bucket string) error {
	if s3Delivery == nil {
		return fmt.Errorf("S3 delivery adapter is nil")
	}

	// Test by generating a signed URL for a test key
	// This verifies credentials and bucket access without uploading data
	testKey := "analytics/exports/.healthcheck"
	_, err := s3Delivery.GenerateSignedURL(ctx, testKey)
	if err != nil {
		return fmt.Errorf("failed to generate test signed URL: %w", err)
	}

	return nil
}

func main() {
	ctx := context.Background()

	// Load configuration
	cfg := config.MustLoad()

	// Initialize observability
	obsCfg := observability.Config{
		ServiceName: cfg.ServiceName,
		Environment: cfg.Environment,
		Endpoint:    cfg.TelemetryEndpoint,
		Protocol:    cfg.TelemetryProtocol,
		Headers:     map[string]string{},
		Insecure:    cfg.TelemetryInsecure,
		LogLevel:    cfg.LogLevel,
	}

	obs := observability.MustInit(ctx, obsCfg)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := obs.Shutdown(shutdownCtx); err != nil {
			obs.Logger.Error("failed to shutdown observability", zap.Error(err))
		}
	}()

	logger := obs.Logger

	// Initialize database store
	store, err := postgres.NewStore(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("failed to initialize database", zap.Error(err))
	}
	defer store.Close()

	// Initialize Redis client for freshness cache
	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		logger.Fatal("failed to parse Redis URL", zap.Error(err))
	}
	redisClient := redis.NewClient(redisOpts)
	defer func() {
		if err := redisClient.Close(); err != nil {
			logger.Error("failed to close Redis client", zap.Error(err))
		}
	}()

	// Verify Redis connection
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := redisClient.Ping(pingCtx).Err(); err != nil {
		logger.Fatal("failed to connect to Redis", zap.Error(err))
	}

	// Create HTTP server
	// RBAC is enabled by default, can be disabled via ENABLE_RBAC=false for development
	apiServer := api.NewServer(api.Config{
		Port:              cfg.HTTPPort,
		Logger:            logger,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		EnableRBAC:        cfg.EnableRBAC,
		MasterAdminAPIKey: cfg.MasterAdminAPIKey,
		Store:             store,
		RedisClient:       redisClient,
	})

	// Initialize freshness cache
	freshnessCache := freshness.NewCache(freshness.Config{
		Client: redisClient,
		Logger: logger,
		TTL:    cfg.FreshnessCacheTTL,
	})

	// Register usage API routes
	usageHandler := api.NewUsageHandler(store, logger, freshnessCache)
	apiServer.RegisterUsageRoutes(usageHandler)

	// Register reliability API routes
	reliabilityHandler := api.NewReliabilityHandler(store, logger)
	apiServer.RegisterReliabilityRoutes(reliabilityHandler)

	// Initialize Linode Object Storage delivery adapter (if configured)
	var s3Delivery *exports.S3Delivery

	// Check if S3 is partially configured (fail fast if misconfigured)
	s3ConfigCount := 0
	if cfg.S3Endpoint != "" {
		s3ConfigCount++
	}
	if cfg.S3AccessKey != "" {
		s3ConfigCount++
	}
	if cfg.S3SecretKey != "" {
		s3ConfigCount++
	}

	if s3ConfigCount > 0 && s3ConfigCount < 3 {
		// Partial configuration detected - fail fast
		logger.Fatal("S3 partially configured - all credentials required",
			zap.Bool("has_endpoint", cfg.S3Endpoint != ""),
			zap.Bool("has_access_key", cfg.S3AccessKey != ""),
			zap.Bool("has_secret_key", cfg.S3SecretKey != ""),
			zap.String("required_env_vars", "S3_ENDPOINT, S3_ACCESS_KEY, S3_SECRET_KEY"),
		)
	}

	if s3ConfigCount == 3 {
		s3Delivery, err = exports.NewS3Delivery(
			cfg.S3Endpoint,
			cfg.S3AccessKey,
			cfg.S3SecretKey,
			cfg.S3Bucket,
			cfg.S3Region,
			cfg.ExportSignedURLTTL,
			logger,
		)
		if err != nil {
			logger.Fatal("failed to initialize S3 delivery adapter", zap.Error(err))
		}

		// Log S3 configuration
		logger.Info("S3 export enabled",
			zap.String("endpoint", cfg.S3Endpoint),
			zap.String("bucket", cfg.S3Bucket),
			zap.String("region", cfg.S3Region),
		)

		// Test S3 connection
		testCtx, testCancel := context.WithTimeout(ctx, 5*time.Second)
		defer testCancel()
		if err := testS3Connection(testCtx, s3Delivery, cfg.S3Bucket); err != nil {
			logger.Fatal("S3 connection test failed",
				zap.Error(err),
				zap.String("endpoint", cfg.S3Endpoint),
				zap.String("bucket", cfg.S3Bucket),
			)
		}
		logger.Info("S3 connection test successful")
	} else {
		logger.Warn("S3 export disabled - export jobs will not be processed",
			zap.String("reason", "S3_ENDPOINT, S3_ACCESS_KEY, and S3_SECRET_KEY not configured"),
			zap.String("impact", "Export job API will accept requests, but jobs will remain pending"),
		)
	}

	// Register exports API routes
	exportsHandler := api.NewExportsHandler(store.Pool(), logger)
	apiServer.RegisterExportsRoutes(exportsHandler)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:      apiServer,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("starting analytics service",
			zap.String("service", cfg.ServiceName),
			zap.String("environment", cfg.Environment),
			zap.Int("port", cfg.HTTPPort),
		)
		serverErrors <- srv.ListenAndServe()
	}()

	// Start rollup worker
	rollupWorker := aggregation.NewWorker(aggregation.Config{
		Store:    store,
		Logger:   logger,
		Interval: cfg.RollupInterval,
		Workers:  cfg.AggregationWorkers,
	})

	go func() {
		if err := rollupWorker.Start(ctx); err != nil {
			logger.Error("rollup worker failed", zap.Error(err))
		}
	}()
	defer rollupWorker.Stop()

	// Start export worker (if S3 delivery is configured)
	var exportWorker *exports.JobRunner
	if s3Delivery != nil {
		exportWorker = exports.NewJobRunner(exports.RunnerConfig{
			Pool:       store.Pool(),
			S3Delivery: s3Delivery,
			Logger:     logger,
			Interval:   cfg.ExportWorkerInterval,
			Workers:    cfg.ExportWorkerConcurrency,
		})

		go func() {
			if err := exportWorker.Start(ctx); err != nil {
				logger.Error("export worker failed", zap.Error(err))
			}
		}()
		defer exportWorker.Stop()
	} else {
		logger.Warn("export worker not started - S3 delivery adapter not configured")
	}

	// Start ingestion consumer
	ingestionConsumer, err := ingestion.NewConsumer(ingestion.Config{
		Brokers:      cfg.KafkaBrokers,
		Topic:        cfg.KafkaTopic,
		GroupID:      cfg.KafkaConsumerGroup,
		BatchSize:    cfg.IngestionBatchSize,
		Workers:      cfg.IngestionWorkers,
		BatchTimeout: cfg.IngestionBatchTimeout,
		Logger:       logger,
		Store:        store,
	})
	if err != nil {
		logger.Warn("failed to create ingestion consumer", zap.Error(err))
		logger.Warn("service will operate in query-only mode (no event ingestion)")
	} else {
		go func() {
			if err := ingestionConsumer.Start(ctx); err != nil {
				logger.Error("ingestion consumer failed", zap.Error(err))
			}
		}()
		defer func() {
			stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := ingestionConsumer.Stop(stopCtx); err != nil {
				logger.Error("failed to stop ingestion consumer", zap.Error(err))
			}
		}()
		logger.Info("ingestion consumer started")
	}

	// Wait for interrupt or server error
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		logger.Fatal("server error", zap.Error(err))

	case sig := <-shutdown:
		logger.Info("shutdown signal received", zap.String("signal", sig.String()))

		// Graceful shutdown
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown failed", zap.Error(err))
			if err := srv.Close(); err != nil {
				logger.Error("force close failed", zap.Error(err))
			}
		}

		logger.Info("shutdown complete")
	}
}
