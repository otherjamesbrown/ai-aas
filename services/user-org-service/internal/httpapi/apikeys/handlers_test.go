package apikeys

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/bootstrap"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/httpapi/middleware"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/storage/postgres"
)

// mockPostgresStore is a minimal mock for testing GetAPIKey
type mockPostgresStore struct {
	// Test data
	apiKeys      map[uuid.UUID]postgres.APIKey
	apiKeysByID  map[string]postgres.APIKey // keyId -> APIKey
	orgs         map[uuid.UUID]postgres.Org
	orgsBySlug   map[string]postgres.Org
}

func newMockPostgresStore() *mockPostgresStore {
	return &mockPostgresStore{
		apiKeys:     make(map[uuid.UUID]postgres.APIKey),
		apiKeysByID: make(map[string]postgres.APIKey),
		orgs:        make(map[uuid.UUID]postgres.Organization),
		orgsBySlug:  make(map[string]postgres.Organization),
	}
}

func (m *mockPostgresStore) GetAPIKeyByID(ctx context.Context, id uuid.UUID) (postgres.APIKey, error) {
	key, ok := m.apiKeys[id]
	if !ok {
		return postgres.APIKey{}, postgres.ErrNotFound
	}
	return key, nil
}

func (m *mockPostgresStore) GetAPIKeyByKeyID(ctx context.Context, orgID uuid.UUID, keyID string) (postgres.APIKey, error) {
	key, ok := m.apiKeysByID[keyID]
	if !ok {
		return postgres.APIKey{}, postgres.ErrNotFound
	}
	// Verify org ownership
	if key.OrgID != orgID {
		return postgres.APIKey{}, postgres.ErrNotFound
	}
	return key, nil
}

func (m *mockPostgresStore) GetOrgBySlug(ctx context.Context, slug string) (postgres.Organization, error) {
	org, ok := m.orgsBySlug[slug]
	if !ok {
		return postgres.Organization{}, postgres.ErrNotFound
	}
	return org, nil
}

// TestGetAPIKey_Success tests successful retrieval by UUID
func TestGetAPIKey_Success(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mock := newMockPostgresStore()

	// Setup test data
	orgID := uuid.New()
	apiKeyID := uuid.New()
	principalID := uuid.New()
	now := time.Now().UTC()

	org := postgres.Organization{
		ID:   orgID,
		Slug: "test-org",
		Name: "Test Organization",
	}
	mock.orgs[orgID] = org
	mock.orgsBySlug["test-org"] = org

	apiKey := postgres.APIKey{
		ID:            apiKeyID,
		OrgID:         orgID,
		KeyID:         "ak_test123",
		PrincipalType: postgres.PrincipalTypeUser,
		PrincipalID:   principalID,
		Notes:         "Test API key",
		Fingerprint:   "test-fingerprint",
		Status:        "active",
		Scopes:        []string{"org:read", "model:read"},
		IssuedAt:      now,
	}
	mock.apiKeys[apiKeyID] = apiKey
	mock.apiKeysByID["ak_test123"] = apiKey

	runtime := &bootstrap.Runtime{
		Postgres: mock,
	}

	handler := &Handler{
		runtime: runtime,
		logger:  logger,
	}

	// Create request with authenticated context
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/orgs/%s/api-keys/%s", orgID.String(), apiKeyID.String()), nil)
	ctx := middleware.WithUserID(req.Context(), uuid.New())
	ctx = middleware.WithOrgID(ctx, orgID)
	req = req.WithContext(ctx)

	// Setup router with chi URLParams
	rr := httptest.NewRecorder()
	router := chi.NewRouter()
	router.Get("/v1/orgs/{orgId}/api-keys/{apiKeyId}", handler.GetAPIKey)
	router.ServeHTTP(rr, req)

	// Verify response
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, "ak_test123", resp["keyId"])
	assert.Equal(t, "Test API key", resp["notes"])
	assert.Equal(t, "user", resp["principalType"])
	assert.Equal(t, principalID.String(), resp["principalId"])
	assert.Equal(t, "test-fingerprint", resp["fingerprint"])
	assert.Equal(t, "active", resp["status"])
	assert.Equal(t, []interface{}{"org:read", "model:read"}, resp["scopes"])
	assert.NotEmpty(t, resp["issuedAt"])
}

// TestGetAPIKey_ByKeyID tests successful retrieval using short keyId
func TestGetAPIKey_ByKeyID(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mock := newMockPostgresStore()

	// Setup test data
	orgID := uuid.New()
	apiKeyID := uuid.New()
	principalID := uuid.New()
	now := time.Now().UTC()

	org := postgres.Organization{
		ID:   orgID,
		Slug: "test-org",
	}
	mock.orgs[orgID] = org
	mock.orgsBySlug["test-org"] = org

	apiKey := postgres.APIKey{
		ID:            apiKeyID,
		OrgID:         orgID,
		KeyID:         "ak_short123",
		PrincipalType: postgres.PrincipalTypeServiceAccount,
		PrincipalID:   principalID,
		Notes:         "Service account key",
		Fingerprint:   "fingerprint-123",
		Status:        "active",
		Scopes:        []string{"apikey:manage"},
		IssuedAt:      now,
	}
	mock.apiKeys[apiKeyID] = apiKey
	mock.apiKeysByID["ak_short123"] = apiKey

	runtime := &bootstrap.Runtime{
		Postgres: mock,
	}

	handler := &Handler{
		runtime: runtime,
		logger:  logger,
	}

	// Create request using keyId instead of UUID
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/orgs/%s/api-keys/ak_short123", orgID.String()), nil)
	ctx := middleware.WithUserID(req.Context(), uuid.New())
	ctx = middleware.WithOrgID(ctx, orgID)
	req = req.WithContext(ctx)

	// Setup router
	rr := httptest.NewRecorder()
	router := chi.NewRouter()
	router.Get("/v1/orgs/{orgId}/api-keys/{apiKeyId}", handler.GetAPIKey)
	router.ServeHTTP(rr, req)

	// Verify response
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, "ak_short123", resp["keyId"])
	assert.Equal(t, "service_account", resp["principalType"])
	assert.Equal(t, "fingerprint-123", resp["fingerprint"])
}

// TestGetAPIKey_ByOrgSlug tests retrieval using org slug instead of UUID
func TestGetAPIKey_ByOrgSlug(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mock := newMockPostgresStore()

	// Setup test data
	orgID := uuid.New()
	apiKeyID := uuid.New()
	principalID := uuid.New()

	org := postgres.Organization{
		ID:   orgID,
		Slug: "my-org-slug",
	}
	mock.orgs[orgID] = org
	mock.orgsBySlug["my-org-slug"] = org

	apiKey := postgres.APIKey{
		ID:            apiKeyID,
		OrgID:         orgID,
		KeyID:         "ak_slug123",
		PrincipalType: postgres.PrincipalTypeUser,
		PrincipalID:   principalID,
		Status:        "active",
		Scopes:        []string{},
		IssuedAt:      time.Now().UTC(),
	}
	mock.apiKeys[apiKeyID] = apiKey

	runtime := &bootstrap.Runtime{
		Postgres: mock,
	}

	handler := &Handler{
		runtime: runtime,
		logger:  logger,
	}

	// Create request using org slug
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/orgs/my-org-slug/api-keys/%s", apiKeyID.String()), nil)
	ctx := middleware.WithUserID(req.Context(), uuid.New())
	ctx = middleware.WithOrgID(ctx, orgID)
	req = req.WithContext(ctx)

	// Setup router
	rr := httptest.NewRecorder()
	router := chi.NewRouter()
	router.Get("/v1/orgs/{orgId}/api-keys/{apiKeyId}", handler.GetAPIKey)
	router.ServeHTTP(rr, req)

	// Verify response
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, "ak_slug123", resp["keyId"])
}

// TestGetAPIKey_NotFound tests 404 when API key doesn't exist
func TestGetAPIKey_NotFound(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mock := newMockPostgresStore()

	orgID := uuid.New()
	apiKeyID := uuid.New()

	org := postgres.Organization{
		ID:   orgID,
		Slug: "test-org",
	}
	mock.orgs[orgID] = org
	mock.orgsBySlug["test-org"] = org

	runtime := &bootstrap.Runtime{
		Postgres: mock,
	}

	handler := &Handler{
		runtime: runtime,
		logger:  logger,
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/orgs/%s/api-keys/%s", orgID.String(), apiKeyID.String()), nil)
	ctx := middleware.WithUserID(req.Context(), uuid.New())
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	router := chi.NewRouter()
	router.Get("/v1/orgs/{orgId}/api-keys/{apiKeyId}", handler.GetAPIKey)
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// TestGetAPIKey_WrongOrg tests 404 when API key belongs to different org
func TestGetAPIKey_WrongOrg(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mock := newMockPostgresStore()

	// Create two orgs
	org1ID := uuid.New()
	org2ID := uuid.New()

	org1 := postgres.Organization{ID: org1ID, Slug: "org1"}
	org2 := postgres.Organization{ID: org2ID, Slug: "org2"}
	mock.orgs[org1ID] = org1
	mock.orgs[org2ID] = org2
	mock.orgsBySlug["org1"] = org1
	mock.orgsBySlug["org2"] = org2

	// Create API key belonging to org1
	apiKeyID := uuid.New()
	apiKey := postgres.APIKey{
		ID:            apiKeyID,
		OrgID:         org1ID,
		KeyID:         "ak_org1key",
		PrincipalType: postgres.PrincipalTypeUser,
		PrincipalID:   uuid.New(),
		Status:        "active",
		Scopes:        []string{},
		IssuedAt:      time.Now().UTC(),
	}
	mock.apiKeys[apiKeyID] = apiKey

	runtime := &bootstrap.Runtime{
		Postgres: mock,
	}

	handler := &Handler{
		runtime: runtime,
		logger:  logger,
	}

	// Try to access org1's key via org2 endpoint
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/orgs/%s/api-keys/%s", org2ID.String(), apiKeyID.String()), nil)
	ctx := middleware.WithUserID(req.Context(), uuid.New())
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	router := chi.NewRouter()
	router.Get("/v1/orgs/{orgId}/api-keys/{apiKeyId}", handler.GetAPIKey)
	router.ServeHTTP(rr, req)

	// Should return 404 (not 403) to avoid leaking existence of keys
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// TestGetAPIKey_WithExpiration tests response includes expiration date
func TestGetAPIKey_WithExpiration(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mock := newMockPostgresStore()

	orgID := uuid.New()
	apiKeyID := uuid.New()
	now := time.Now().UTC()
	expiresAt := now.Add(30 * 24 * time.Hour)

	org := postgres.Organization{ID: orgID, Slug: "test-org"}
	mock.orgs[orgID] = org
	mock.orgsBySlug["test-org"] = org

	apiKey := postgres.APIKey{
		ID:            apiKeyID,
		OrgID:         orgID,
		KeyID:         "ak_expiring",
		PrincipalType: postgres.PrincipalTypeUser,
		PrincipalID:   uuid.New(),
		Status:        "active",
		Scopes:        []string{},
		IssuedAt:      now,
		ExpiresAt:     &expiresAt,
	}
	mock.apiKeys[apiKeyID] = apiKey

	runtime := &bootstrap.Runtime{
		Postgres: mock,
	}

	handler := &Handler{
		runtime: runtime,
		logger:  logger,
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/orgs/%s/api-keys/%s", orgID.String(), apiKeyID.String()), nil)
	ctx := middleware.WithUserID(req.Context(), uuid.New())
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	router := chi.NewRouter()
	router.Get("/v1/orgs/{orgId}/api-keys/{apiKeyId}", handler.GetAPIKey)
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)

	assert.NotNil(t, resp["expiresAt"])
	expiresAtStr, ok := resp["expiresAt"].(string)
	require.True(t, ok)

	parsedExpiry, err := time.Parse(time.RFC3339, expiresAtStr)
	require.NoError(t, err)
	assert.True(t, parsedExpiry.Equal(expiresAt) || parsedExpiry.Sub(expiresAt).Abs() < time.Second)
}

// TestGetAPIKey_RevokedKey tests retrieval of revoked API key
func TestGetAPIKey_RevokedKey(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mock := newMockPostgresStore()

	orgID := uuid.New()
	apiKeyID := uuid.New()
	now := time.Now().UTC()
	revokedAt := now.Add(-1 * time.Hour)

	org := postgres.Organization{ID: orgID, Slug: "test-org"}
	mock.orgs[orgID] = org
	mock.orgsBySlug["test-org"] = org

	apiKey := postgres.APIKey{
		ID:            apiKeyID,
		OrgID:         orgID,
		KeyID:         "ak_revoked",
		PrincipalType: postgres.PrincipalTypeUser,
		PrincipalID:   uuid.New(),
		Status:        "revoked",
		Scopes:        []string{},
		IssuedAt:      now.Add(-24 * time.Hour),
		RevokedAt:     &revokedAt,
	}
	mock.apiKeys[apiKeyID] = apiKey

	runtime := &bootstrap.Runtime{
		Postgres: mock,
	}

	handler := &Handler{
		runtime: runtime,
		logger:  logger,
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/orgs/%s/api-keys/%s", orgID.String(), apiKeyID.String()), nil)
	ctx := middleware.WithUserID(req.Context(), uuid.New())
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	router := chi.NewRouter()
	router.Get("/v1/orgs/{orgId}/api-keys/{apiKeyId}", handler.GetAPIKey)
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, "revoked", resp["status"])
}

// TestGetAPIKey_OrgNotFound tests 404 when organization doesn't exist
func TestGetAPIKey_OrgNotFound(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mock := newMockPostgresStore()

	runtime := &bootstrap.Runtime{
		Postgres: mock,
	}

	handler := &Handler{
		runtime: runtime,
		logger:  logger,
	}

	// Use non-existent org slug
	req := httptest.NewRequest(http.MethodGet, "/v1/orgs/nonexistent-org/api-keys/ak_test123", nil)
	ctx := middleware.WithUserID(req.Context(), uuid.New())
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	router := chi.NewRouter()
	router.Get("/v1/orgs/{orgId}/api-keys/{apiKeyId}", handler.GetAPIKey)
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}
