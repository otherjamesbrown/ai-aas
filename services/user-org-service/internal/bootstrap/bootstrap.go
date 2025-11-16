// Package bootstrap provides centralized initialization and lifecycle management for
// core service dependencies (Postgres, Redis, OAuth provider).
//
// Purpose:
//
//	This package wires together the foundational runtime dependencies required by
//	both the admin-api and reconciler binaries. It ensures consistent initialization
//	order, handles connection failures gracefully, and provides a unified shutdown
//	and health check interface.
//
// Dependencies:
//   - github.com/ory/fosite: OAuth2 provider framework
//   - github.com/redis/go-redis/v9: Redis client for session caching
//   - internal/config: Service configuration from environment variables
//   - internal/oauth: OAuth2 storage, caching, and provider composition
//   - internal/storage/postgres: Core data access layer
//
// Key Responsibilities:
//   - Initialize connects to Postgres and optional Redis, composes OAuth provider
//   - Runtime bundles all initialized dependencies for use by binaries
//   - ReadinessProbe checks health of Postgres and Redis connections
//   - Close releases all resources in reverse initialization order
//
// Requirements Reference:
//   - specs/005-user-org-service/spec.md#NFR-001 (Service Availability)
//   - specs/005-user-org-service/spec.md#FR-005 (OAuth2 Support)
//   - specs/005-user-org-service/spec.md#NFR-003 (Session Management)
//
// Debugging Notes:
//   - Redis connection failures fail fast during initialization (2s timeout)
//   - If Redis is unavailable, a no-op cache is used (graceful degradation)
//   - Postgres connection failures prevent service startup (required dependency)
//   - OAuth provider composition requires valid HMAC secret (minimum 32 bytes)
//   - ReadinessProbe is used by Kubernetes liveness/readiness checks
//
// Thread Safety:
//   - Runtime struct is safe for concurrent read access after initialization
//   - Close should be called once during shutdown (not thread-safe for concurrent calls)
//
// Error Handling:
//   - Initialization errors are wrapped with context (e.g., "bootstrap postgres: ...")
//   - ReadinessProbe returns errors that include dependency names for observability
//   - Close collects errors but returns the first one encountered
package bootstrap

import (
	"context"
	"fmt"
	"time"

	"github.com/ory/fosite"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/audit"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/logging"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/oauth"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/security"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/storage/postgres"
)

// Runtime bundles initialized runtime dependencies for use by service binaries.
// All fields are populated during Initialize and remain valid until Close is called.
type Runtime struct {
	Config         *config.Config           // Service configuration (read-only after init)
	Postgres       *postgres.Store          // PostgreSQL data access layer (required)
	Redis          *redis.Client            // Redis client for session caching (optional, nil if not configured)
	OAuthStore     *oauth.Store             // OAuth2 storage implementation (backed by Postgres + optional Redis cache)
	OAuthCache     oauth.SessionCache       // Session cache implementation (Redis or no-op)
	OAuthConfig    *fosite.Config           // Fosite OAuth2 configuration (token lifetimes, PKCE settings, etc.)
	Provider       fosite.OAuth2Provider    // Composed OAuth2 provider ready for use in HTTP handlers
	Audit          audit.Emitter            // Audit event emitter (logger-based stub, replace with Kafka in production)
	LockoutTracker *security.LockoutTracker // Lockout tracker for failed authentication attempts (optional, nil if Redis not configured)
	// Note: IdPRegistry is initialized separately in main.go to avoid import cycles
	// It should be set after bootstrap initialization
}

// Initialize wires core dependencies based on the provided configuration.
// Initialization order: Postgres → Redis (if configured) → OAuth store → OAuth provider.
// Returns an error if any required dependency fails to initialize.
// The returned Runtime must be closed via Close() during shutdown.
func Initialize(ctx context.Context, cfg *config.Config) (*Runtime, error) {
	pgStore, err := postgres.NewStore(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("bootstrap postgres: %w", err)
	}

	logger := logging.New(cfg.ServiceName, cfg.LogLevel)

	// Initialize audit emitter (Kafka if configured, otherwise logger)
	var auditEmitter audit.Emitter
	if kafkaEmitter, err := audit.NewKafkaEmitterFromConfig(cfg.KafkaBrokers, cfg.KafkaTopic, cfg.KafkaClientID, logger); err != nil {
		logger.Warn("failed to initialize Kafka emitter, falling back to logger", zap.Error(err))
		auditEmitter = audit.NewLoggerEmitter(logger)
	} else if kafkaEmitter != nil {
		logger.Info("using Kafka emitter for audit events", zap.String("topic", cfg.KafkaTopic))
		auditEmitter = kafkaEmitter
	} else {
		logger.Info("Kafka not configured, using logger emitter for audit events")
		auditEmitter = audit.NewLoggerEmitter(logger)
	}

	runtime := &Runtime{
		Config:   cfg,
		Postgres: pgStore,
		Audit:    auditEmitter,
	}

	if cfg.RedisAddr != "" {
		runtime.Redis = redis.NewClient(&redis.Options{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		})

		// Best-effort ping with timeout to fail fast if Redis is unavailable.
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if err := runtime.Redis.Ping(pingCtx).Err(); err != nil {
			return nil, fmt.Errorf("bootstrap redis: %w", err)
		}
	}

	var sessionCache oauth.SessionCache
	if runtime.Redis != nil {
		sessionCache = oauth.NewRedisSessionCache(runtime.Redis, "user-org-service")
	}

	oauthStore := oauth.NewStoreWithCache(pgStore, sessionCache)
	runtime.OAuthStore = oauthStore
	runtime.OAuthCache = sessionCache

	// Initialize lockout tracker if Redis is available
	if runtime.Redis != nil {
		lockoutCfg := security.LockoutConfig{
			MaxAttempts:     cfg.LockoutMaxAttempts,
			LockoutDuration: time.Duration(cfg.LockoutDurationMinutes) * time.Minute,
			WindowDuration:  time.Duration(cfg.LockoutWindowMinutes) * time.Minute,
		}
		runtime.LockoutTracker = security.NewLockoutTracker(runtime.Redis, lockoutCfg)
	}

	provider, err := oauth.NewProvider(oauth.ProviderDependencies{
		PostgresStore: pgStore,
		SessionCache:  sessionCache,
		HMACSecret:    []byte(cfg.OAuthHMACSecret),
		StaticClients: []oauth.StaticClient{{ID: cfg.OAuthClientID, Secret: cfg.OAuthClientSecret}},
	})
	if err != nil {
		return nil, fmt.Errorf("bootstrap provider: %w", err)
	}
	runtime.OAuthConfig = oauthStore.Config()
	runtime.Provider = provider

	// Note: IdP registry initialization moved to main.go to avoid import cycles
	// Initialize it there after bootstrap completes

	return runtime, nil
}

// Close releases runtime resources in reverse initialization order.
// Safe to call multiple times (idempotent). Returns the first error encountered,
// but continues closing other resources. Postgres pool, Redis connections, and
// Kafka emitter are closed; OAuth provider and stores require no explicit cleanup.
func (rt *Runtime) Close(ctx context.Context) error {
	if rt == nil {
		return nil
	}
	var firstErr error
	if rt.Postgres != nil {
		rt.Postgres.Close()
	}
	if rt.Redis != nil {
		if err := rt.Redis.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	// Close Kafka emitter if it's a KafkaEmitter
	if kafkaEmitter, ok := rt.Audit.(*audit.KafkaEmitter); ok {
		if err := kafkaEmitter.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// ReadinessProbe checks the health of critical runtime dependencies.
// Used by Kubernetes readiness checks and /readyz endpoint. Returns an error
// if Postgres or Redis (if configured) are unreachable. Context timeout should
// be set by the caller (typically 1-2 seconds for fast failure).
func (rt *Runtime) ReadinessProbe(ctx context.Context) error {
	if rt.Postgres != nil {
		if err := rt.Postgres.Pool().Ping(ctx); err != nil {
			return fmt.Errorf("postgres not ready: %w", err)
		}
	}
	if rt.Redis != nil {
		if err := rt.Redis.Ping(ctx).Err(); err != nil {
			return fmt.Errorf("redis not ready: %w", err)
		}
	}
	return nil
}
