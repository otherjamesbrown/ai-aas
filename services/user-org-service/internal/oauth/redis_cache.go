package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// RedisSessionCache implements SessionCache backed by Redis.
type RedisSessionCache struct {
	client *redis.Client
	prefix string
}

// NewRedisSessionCache creates a redis-backed session cache.
func NewRedisSessionCache(client *redis.Client, prefix string) *RedisSessionCache {
	if prefix == "" {
		prefix = "oauth"
	}
	return &RedisSessionCache{
		client: client,
		prefix: prefix,
	}
}

func (c *RedisSessionCache) Get(ctx context.Context, tokenType, signature string) (*cachedSessionEntry, error) {
	data, err := c.client.Get(ctx, c.signatureKey(tokenType, signature)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	var entry cachedSessionEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

func (c *RedisSessionCache) Set(ctx context.Context, tokenType, signature string, req *storedRequest, expiresAt time.Time, ttl time.Duration) error {
	if req == nil {
		return nil
	}

	entry := cachedSessionEntry{
		Request:   *req,
		ExpiresAt: expiresAt,
	}
	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	sigKey := c.signatureKey(tokenType, signature)
	reqKey := c.requestKey(tokenType, req.RequestID)

	pipe := c.client.TxPipeline()
	pipe.Set(ctx, sigKey, payload, ttl)
	pipe.SAdd(ctx, reqKey, signature)
	pipe.Expire(ctx, reqKey, ttl)
	_, err = pipe.Exec(ctx)
	return err
}

func (c *RedisSessionCache) Delete(ctx context.Context, tokenType, signature string) error {
	sigKey := c.signatureKey(tokenType, signature)

	data, err := c.client.Get(ctx, sigKey).Bytes()
	if err != nil && err != redis.Nil {
		return err
	}

	pipe := c.client.TxPipeline()
	pipe.Del(ctx, sigKey)

	if err == nil {
		var entry cachedSessionEntry
		if json.Unmarshal(data, &entry) == nil {
			reqKey := c.requestKey(tokenType, entry.Request.RequestID)
			pipe.SRem(ctx, reqKey, signature)
		}
	}

	_, execErr := pipe.Exec(ctx)
	return execErr
}

func (c *RedisSessionCache) DeleteByRequestID(ctx context.Context, tokenType string, requestID uuid.UUID) error {
	reqKey := c.requestKey(tokenType, requestID)
	signatures, err := c.client.SMembers(ctx, reqKey).Result()
	if err != nil && err != redis.Nil {
		return err
	}

	pipe := c.client.TxPipeline()
	for _, sig := range signatures {
		pipe.Del(ctx, c.signatureKey(tokenType, sig))
	}
	pipe.Del(ctx, reqKey)
	_, execErr := pipe.Exec(ctx)
	return execErr
}

func (c *RedisSessionCache) signatureKey(tokenType, signature string) string {
	return fmt.Sprintf("%s:%s:%s", c.prefix, tokenType, signature)
}

func (c *RedisSessionCache) requestKey(tokenType string, requestID interface{}) string {
	return fmt.Sprintf("%s:req:%s:%v", c.prefix, tokenType, requestID)
}
