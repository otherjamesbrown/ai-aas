// Package freshness provides Redis-backed caching for freshness indicators.
//
// Purpose:
//   This package caches freshness status from the freshness_status table in Redis
//   for fast lookups by the API. It syncs with the database periodically and
//   provides TTL-based expiration.
//
package freshness

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Cache provides Redis-backed freshness caching.
type Cache struct {
	client *redis.Client
	logger *zap.Logger
	ttl    time.Duration
}

// Config holds cache configuration.
type Config struct {
	Client *redis.Client
	Logger *zap.Logger
	TTL    time.Duration
}

// NewCache creates a new freshness cache.
func NewCache(cfg Config) *Cache {
	return &Cache{
		client: cfg.Client,
		logger: cfg.Logger,
		ttl:    cfg.TTL,
	}
}

// Indicator represents freshness status for an org/model.
type Indicator struct {
	OrgID        uuid.UUID `json:"org_id"`
	ModelID      *uuid.UUID `json:"model_id,omitempty"`
	LastEventAt  time.Time `json:"last_event_at"`
	LastRollupAt time.Time `json:"last_rollup_at"`
	LagSeconds   int       `json:"lag_seconds"`
	Status       string    `json:"status"`
}

// Get retrieves freshness indicator from cache.
func (c *Cache) Get(ctx context.Context, orgID uuid.UUID, modelID *uuid.UUID) (*Indicator, error) {
	key := c.key(orgID, modelID)

	data, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil // Not found in cache
	}
	if err != nil {
		return nil, fmt.Errorf("redis get: %w", err)
	}

	var indicator Indicator
	if err := json.Unmarshal([]byte(data), &indicator); err != nil {
		return nil, fmt.Errorf("unmarshal indicator: %w", err)
	}

	return &indicator, nil
}

// Set stores freshness indicator in cache.
func (c *Cache) Set(ctx context.Context, indicator *Indicator) error {
	key := c.key(indicator.OrgID, indicator.ModelID)

	data, err := json.Marshal(indicator)
	if err != nil {
		return fmt.Errorf("marshal indicator: %w", err)
	}

	if err := c.client.Set(ctx, key, data, c.ttl).Err(); err != nil {
		return fmt.Errorf("redis set: %w", err)
	}

	return nil
}

// SyncFromDB syncs freshness indicators from database to cache.
func (c *Cache) SyncFromDB(ctx context.Context, store FreshnessRepository) error {
	indicators, err := store.GetAllFreshnessStatus(ctx)
	if err != nil {
		return fmt.Errorf("get freshness status: %w", err)
	}

	for _, indicator := range indicators {
		if err := c.Set(ctx, indicator); err != nil {
			c.logger.Warn("failed to cache freshness indicator",
				zap.String("org_id", indicator.OrgID.String()),
				zap.Error(err),
			)
			// Continue syncing other indicators
		}
	}

	c.logger.Debug("synced freshness cache",
		zap.Int("count", len(indicators)),
	)

	return nil
}

// key generates a Redis key for an org/model combination.
func (c *Cache) key(orgID uuid.UUID, modelID *uuid.UUID) string {
	if modelID != nil {
		return fmt.Sprintf("analytics:freshness:%s:%s", orgID.String(), modelID.String())
	}
	return fmt.Sprintf("analytics:freshness:%s", orgID.String())
}

// FreshnessRepository defines methods for querying freshness status from database.
type FreshnessRepository interface {
	GetAllFreshnessStatus(ctx context.Context) ([]*Indicator, error)
}

