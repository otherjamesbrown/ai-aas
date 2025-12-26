package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/config"
	modelsHandler "github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/handlers/models"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/repository"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/worker"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load configuration", zap.Error(err))
	}

	// Initialize database connection
	db, err := repository.NewDB(cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	// Create router with all handlers
	router := api.NewRouter(cfg, db, logger)

	// Create HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start background worker if enabled
	var pullWorker *worker.Worker
	if cfg.WorkerEnabled {
		// Create credential getter from models service
		modelsSvc := modelsHandler.CreateModelsService(db.ConcretePool())
		credGetter := &credentialGetterAdapter{svc: modelsSvc}

		pullWorker = worker.New(
			db.ConcretePool(),
			credGetter,
			worker.Config{
				PollInterval:           cfg.WorkerPollInterval,
				ProgressUpdateInterval: cfg.WorkerProgressUpdateInterval,
			},
			logger,
		)

		if err := pullWorker.Start(context.Background()); err != nil {
			logger.Error("failed to start worker", zap.Error(err))
		} else {
			logger.Info("pull job worker started",
				zap.Duration("poll_interval", cfg.WorkerPollInterval),
			)
		}
	}

	// Start server in goroutine
	go func() {
		logger.Info("starting admin-api-service",
			zap.Int("port", cfg.Port),
			zap.String("version", cfg.Version),
			zap.Bool("worker_enabled", cfg.WorkerEnabled),
		)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server failed", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")

	// Stop worker first (gracefully finishes current job)
	if pullWorker != nil {
		pullWorker.Stop()
	}

	// Graceful shutdown with 30s timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("server forced to shutdown", zap.Error(err))
	}

	logger.Info("server exited")
}

// credentialGetterAdapter adapts the models service to the worker's CredentialGetter interface
type credentialGetterAdapter struct {
	svc interface {
		GetCredential(ctx context.Context, credType string) (string, error)
	}
}

func (a *credentialGetterAdapter) GetCredential(ctx context.Context, credType string) (string, error) {
	return a.svc.GetCredential(ctx, credType)
}

