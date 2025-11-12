package oauth

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// SessionCache provides optional caching for OAuth sessions keyed by token signature.
type SessionCache interface {
	Get(ctx context.Context, tokenType, signature string) (*cachedSessionEntry, error)
	Set(ctx context.Context, tokenType, signature string, req *storedRequest, expiresAt time.Time, ttl time.Duration) error
	Delete(ctx context.Context, tokenType, signature string) error
	DeleteByRequestID(ctx context.Context, tokenType string, requestID uuid.UUID) error
}

type cachedSessionEntry struct {
	Request   storedRequest `json:"request"`
	ExpiresAt time.Time     `json:"expires_at"`
}

type noopSessionCache struct{}

func (noopSessionCache) Get(context.Context, string, string) (*cachedSessionEntry, error) {
	return nil, nil
}

func (noopSessionCache) Set(context.Context, string, string, *storedRequest, time.Time, time.Duration) error {
	return nil
}

func (noopSessionCache) Delete(context.Context, string, string) error {
	return nil
}

func (noopSessionCache) DeleteByRequestID(context.Context, string, uuid.UUID) error {
	return nil
}
