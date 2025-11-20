// Package routing provides model registry integration for dynamic routing.
//
// Purpose:
//   This module integrates with the model_registry_entries table to enable
//   dynamic routing to vLLM deployments based on model name and environment.
//   It supports Redis caching for performance and automatic invalidation.
//
// Key Responsibilities:
//   - Query model registry for deployment endpoints
//   - Cache registry lookups in Redis (TTL: 2 minutes)
//   - Filter by deployment status (only 'ready' models are routable)
//   - Support environment-based routing
//
// Requirements Reference:
//   - specs/010-vllm-deployment/spec.md#US-002 (Register models for routing)
//   - specs/010-vllm-deployment/tasks.md#T-S010-P04-036 (API Router integration)
//
package routing

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	_ "github.com/lib/pq" // PostgreSQL driver
	"go.uber.org/zap"
)

// ModelRegistryEntry represents a model deployment in the registry.
type ModelRegistryEntry struct {
	ModelID               string
	ModelName             string
	DeploymentEndpoint    string
	DeploymentStatus      string
	DeploymentEnvironment string
	DeploymentNamespace   string
	LastHealthCheckAt     *time.Time
	UpdatedAt             time.Time
}

// Registry provides model registry lookups with Redis caching.
type Registry struct {
	db          *sql.DB
	redis       *redis.Client
	logger      *zap.Logger
	cacheTTL    time.Duration
	environment string
}

// RegistryConfig configures the model registry.
type RegistryConfig struct {
	DatabaseURL string
	RedisAddr   string
	RedisPassword string
	RedisDB     int
	CacheTTL    time.Duration
	Environment string
}

// NewRegistry creates a new model registry with database and Redis connections.
func NewRegistry(cfg *RegistryConfig, logger *zap.Logger) (*Registry, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	// Connect to PostgreSQL
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Connect to Redis (optional - if not configured, caching is disabled)
	var redisClient *redis.Client
	if cfg.RedisAddr != "" {
		redisClient = redis.NewClient(&redis.Options{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		})

		// Test Redis connection
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := redisClient.Ping(ctx).Err(); err != nil {
			logger.Warn("redis connection failed, caching disabled", zap.Error(err))
			_ = redisClient.Close()
			redisClient = nil
		} else {
			logger.Info("connected to redis for model registry caching")
		}
	}

	cacheTTL := cfg.CacheTTL
	if cacheTTL == 0 {
		cacheTTL = 2 * time.Minute
	}

	environment := cfg.Environment
	if environment == "" {
		environment = "development"
	}

	logger.Info("model registry initialized",
		zap.String("environment", environment),
		zap.Duration("cache_ttl", cacheTTL),
		zap.Bool("redis_enabled", redisClient != nil),
	)

	return &Registry{
		db:          db,
		redis:       redisClient,
		logger:      logger,
		cacheTTL:    cacheTTL,
		environment: environment,
	}, nil
}

// Close closes database and Redis connections.
func (r *Registry) Close() error {
	if r.redis != nil {
		if err := r.redis.Close(); err != nil {
			r.logger.Warn("failed to close redis connection", zap.Error(err))
		}
	}
	if r.db != nil {
		return r.db.Close()
	}
	return nil
}

// LookupModel finds a model deployment by name and returns its endpoint.
// Returns nil if the model is not found or not in 'ready' status.
func (r *Registry) LookupModel(ctx context.Context, modelName string) (*ModelRegistryEntry, error) {
	return r.LookupModelInEnvironment(ctx, modelName, r.environment)
}

// LookupModelInEnvironment finds a model deployment by name and environment.
func (r *Registry) LookupModelInEnvironment(ctx context.Context, modelName, environment string) (*ModelRegistryEntry, error) {
	// Try cache first
	if r.redis != nil {
		if entry, err := r.getFromCache(ctx, modelName, environment); err == nil && entry != nil {
			r.logger.Debug("model registry cache hit",
				zap.String("model_name", modelName),
				zap.String("environment", environment),
			)
			return entry, nil
		}
	}

	// Query database
	entry, err := r.queryDatabase(ctx, modelName, environment)
	if err != nil {
		return nil, err
	}

	// Store in cache
	if r.redis != nil && entry != nil {
		if err := r.putInCache(ctx, modelName, environment, entry); err != nil {
			r.logger.Warn("failed to cache model registry entry",
				zap.Error(err),
				zap.String("model_name", modelName),
			)
		}
	}

	return entry, nil
}

// queryDatabase queries the model registry database for a model deployment.
func (r *Registry) queryDatabase(ctx context.Context, modelName, environment string) (*ModelRegistryEntry, error) {
	query := `
		SELECT
			model_id,
			model_name,
			deployment_endpoint,
			deployment_status,
			deployment_environment,
			deployment_namespace,
			last_health_check_at,
			updated_at
		FROM model_registry_entries
		WHERE model_name = $1
		  AND deployment_environment = $2
		  AND deployment_status = 'ready'
		  AND deployment_endpoint IS NOT NULL
		LIMIT 1
	`

	var entry ModelRegistryEntry
	var lastHealthCheck sql.NullTime

	err := r.db.QueryRowContext(ctx, query, modelName, environment).Scan(
		&entry.ModelID,
		&entry.ModelName,
		&entry.DeploymentEndpoint,
		&entry.DeploymentStatus,
		&entry.DeploymentEnvironment,
		&entry.DeploymentNamespace,
		&lastHealthCheck,
		&entry.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		r.logger.Debug("model not found in registry",
			zap.String("model_name", modelName),
			zap.String("environment", environment),
		)
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("query model registry: %w", err)
	}

	if lastHealthCheck.Valid {
		entry.LastHealthCheckAt = &lastHealthCheck.Time
	}

	r.logger.Debug("model found in registry",
		zap.String("model_name", modelName),
		zap.String("environment", environment),
		zap.String("endpoint", entry.DeploymentEndpoint),
		zap.String("status", entry.DeploymentStatus),
	)

	return &entry, nil
}

// getFromCache retrieves a model registry entry from Redis cache.
func (r *Registry) getFromCache(ctx context.Context, modelName, environment string) (*ModelRegistryEntry, error) {
	key := r.cacheKey(modelName, environment)

	data, err := r.redis.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, fmt.Errorf("cache miss")
	}
	if err != nil {
		return nil, fmt.Errorf("redis get: %w", err)
	}

	var entry ModelRegistryEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("unmarshal cache entry: %w", err)
	}

	return &entry, nil
}

// putInCache stores a model registry entry in Redis cache.
func (r *Registry) putInCache(ctx context.Context, modelName, environment string, entry *ModelRegistryEntry) error {
	key := r.cacheKey(modelName, environment)

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal cache entry: %w", err)
	}

	if err := r.redis.Set(ctx, key, data, r.cacheTTL).Err(); err != nil {
		return fmt.Errorf("redis set: %w", err)
	}

	return nil
}

// InvalidateCache removes a model from the Redis cache.
func (r *Registry) InvalidateCache(ctx context.Context, modelName, environment string) error {
	if r.redis == nil {
		return nil
	}

	key := r.cacheKey(modelName, environment)
	if err := r.redis.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("redis del: %w", err)
	}

	r.logger.Debug("invalidated cache entry",
		zap.String("model_name", modelName),
		zap.String("environment", environment),
	)

	return nil
}

// cacheKey generates a Redis cache key for a model and environment.
func (r *Registry) cacheKey(modelName, environment string) string {
	return fmt.Sprintf("model_registry:%s:%s", environment, modelName)
}

// ListReadyModels returns all models in 'ready' status for the current environment.
func (r *Registry) ListReadyModels(ctx context.Context) ([]*ModelRegistryEntry, error) {
	return r.ListReadyModelsInEnvironment(ctx, r.environment)
}

// ListReadyModelsInEnvironment returns all ready models for a specific environment.
func (r *Registry) ListReadyModelsInEnvironment(ctx context.Context, environment string) ([]*ModelRegistryEntry, error) {
	query := `
		SELECT
			model_id,
			model_name,
			deployment_endpoint,
			deployment_status,
			deployment_environment,
			deployment_namespace,
			last_health_check_at,
			updated_at
		FROM model_registry_entries
		WHERE deployment_environment = $1
		  AND deployment_status = 'ready'
		  AND deployment_endpoint IS NOT NULL
		ORDER BY model_name
	`

	rows, err := r.db.QueryContext(ctx, query, environment)
	if err != nil {
		return nil, fmt.Errorf("query ready models: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var entries []*ModelRegistryEntry
	for rows.Next() {
		var entry ModelRegistryEntry
		var lastHealthCheck sql.NullTime

		err := rows.Scan(
			&entry.ModelID,
			&entry.ModelName,
			&entry.DeploymentEndpoint,
			&entry.DeploymentStatus,
			&entry.DeploymentEnvironment,
			&entry.DeploymentNamespace,
			&lastHealthCheck,
			&entry.UpdatedAt,
		)
		if err != nil {
			r.logger.Warn("failed to scan model registry entry", zap.Error(err))
			continue
		}

		if lastHealthCheck.Valid {
			entry.LastHealthCheckAt = &lastHealthCheck.Time
		}

		entries = append(entries, &entry)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}

	return entries, nil
}
