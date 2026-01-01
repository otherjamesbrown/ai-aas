package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/ai-aas/shared-go/auth/apikey"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// TestAPIKeyRevocationInvalidatesCache is a regression test for the bug where
// revoked API keys continue to work because the cache is not invalidated.
//
// Scenario:
// 1. API key is validated and cached by api-router-service
// 2. API key is revoked via user-org-service
// 3. user-org-service publishes cache invalidation event to Redis
// 4. api-router-service subscriber receives event and invalidates cache
// 5. Next authentication attempt re-validates against user-org-service (which returns invalid)
//
// Bug: If the subscriber is not running or Redis pub/sub fails, the cache is not invalidated
// and the revoked key continues to work until cache TTL expires (1 minute).
func TestAPIKeyRevocationInvalidatesCache(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Start miniredis (in-memory Redis for testing)
	mr := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer redisClient.Close()

	// Create a mock user-org-service that simulates validation responses
	validationAttempts := 0
	mockUserOrgService := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		validationAttempts++

		// First validation: key is valid
		// Second validation (after revocation): key is invalid
		isValid := validationAttempts == 1

		resp := map[string]interface{}{
			"valid":          isValid,
			"apiKeyId":       "test-key-id",
			"organizationId": "test-org-id",
			"principalId":    "test-principal-id",
			"principalType":  "user",
			"scopes":         []string{"inference:read"},
			"status":         "active",
		}

		if !isValid {
			resp["message"] = "API key is revoked"
			resp["status"] = "revoked"
		}

		w.Header().Set("Content-Type", "application/json")
		if !isValid {
			w.WriteHeader(http.StatusUnauthorized)
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockUserOrgService.Close()

	// Create authenticator
	authenticator := NewAuthenticator(logger, mockUserOrgService.URL, 2*time.Second)

	// Start cache invalidation subscriber
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	subscriberStarted := make(chan struct{})
	go func() {
		close(subscriberStarted)
		_ = authenticator.StartCacheInvalidationSubscriber(ctx, redisClient)
	}()

	// Wait for subscriber to start
	<-subscriberStarted
	time.Sleep(100 * time.Millisecond)

	// Step 1: Authenticate with API key (should succeed and cache result)
	apiKey := "ai-aas_test-key-12345"
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.Header.Set("X-API-Key", apiKey)

	authCtx1, err := authenticator.Authenticate(req1)
	require.NoError(t, err, "first authentication should succeed")
	require.NotNil(t, authCtx1, "auth context should be returned")
	assert.Equal(t, "test-key-id", authCtx1.APIKeyID)

	// Verify the result is cached
	fingerprint := apikey.ComputeFingerprintBase64URL(apiKey)
	authenticator.cacheMutex.RLock()
	_, cached := authenticator.validationCache[fingerprint]
	authenticator.cacheMutex.RUnlock()
	require.True(t, cached, "authentication result should be cached")

	// Step 2: Simulate revocation by publishing cache invalidation event
	// (This simulates what user-org-service does when revoking a key)
	err = redisClient.Publish(ctx, "apikey:invalidate", fingerprint).Err()
	require.NoError(t, err, "publishing invalidation event should succeed")

	// Wait for subscriber to process the invalidation event
	time.Sleep(200 * time.Millisecond)

	// Verify cache entry is removed
	authenticator.cacheMutex.RLock()
	_, stillCached := authenticator.validationCache[fingerprint]
	authenticator.cacheMutex.RUnlock()
	assert.False(t, stillCached, "cache entry should be removed after invalidation")

	// Step 3: Try to authenticate again with the same key
	// This should trigger re-validation against user-org-service
	// user-org-service will now return invalid (simulating revocation)
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.Header.Set("X-API-Key", apiKey)

	authCtx2, err := authenticator.Authenticate(req2)
	assert.Error(t, err, "authentication with revoked key should fail")
	assert.Nil(t, authCtx2, "auth context should be nil for revoked key")
	assert.Contains(t, err.Error(), "invalid API key", "error should indicate invalid API key")

	// Verify we made exactly 2 calls to user-org-service:
	// 1. Initial validation (cached)
	// 2. Re-validation after cache invalidation (returned invalid)
	assert.Equal(t, 2, validationAttempts, "should have made exactly 2 validation attempts")
}

// TestAPIKeyRevocationWithoutRedis verifies behavior when Redis is not available.
// Without Redis, cache invalidation cannot happen, so revoked keys will continue
// to work until the cache TTL expires (1 minute).
//
// This test documents the degraded behavior when Redis is unavailable.
func TestAPIKeyRevocationWithoutRedis(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Create a mock user-org-service that always returns valid
	mockUserOrgService := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"valid":          true,
			"apiKeyId":       "test-key-id",
			"organizationId": "test-org-id",
			"principalId":    "test-principal-id",
			"principalType":  "user",
			"scopes":         []string{"inference:read"},
			"status":         "active",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockUserOrgService.Close()

	// Create authenticator without Redis
	authenticator := NewAuthenticator(logger, mockUserOrgService.URL, 2*time.Second)

	// Step 1: Authenticate with API key (should succeed and cache result)
	apiKey := "ai-aas_test-key-67890"
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.Header.Set("X-API-Key", apiKey)

	authCtx1, err := authenticator.Authenticate(req1)
	require.NoError(t, err, "first authentication should succeed")
	require.NotNil(t, authCtx1)

	// Step 2: Try to authenticate again immediately
	// Without cache invalidation, this should use the cached result
	// (no call to user-org-service)
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.Header.Set("X-API-Key", apiKey)

	authCtx2, err := authenticator.Authenticate(req2)
	assert.NoError(t, err, "cached authentication should succeed even if key was revoked")
	assert.NotNil(t, authCtx2)

	// NOTE: This is the documented behavior when Redis is unavailable.
	// Revoked keys will continue to work for up to 1 minute (cache TTL).
	// For production systems, Redis should always be available.
}
