package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	testcontainers "github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/audit"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/bootstrap"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/httpapi/middleware"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/httpapi/orgs"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/storage/postgres"
)

// TestScopeEnforcement_Integration tests that API key scopes are properly enforced
// across the full middleware chain (RequireAuth + RequireAdminScope).
//
// This test validates that:
// 1. API keys with inference:read scope CAN access inference endpoints (if they existed)
// 2. API keys with inference:read scope CANNOT create organizations (requires org:admin)
// 3. API keys with org:admin scope CAN create organizations
// 4. API keys with admin scope have full access (universal grant)
//
// This is an integration test that uses a real PostgreSQL database via testcontainers
// and the full HTTP handler stack including authentication middleware.
func TestScopeEnforcement_Integration(t *testing.T) {
	// Skip in short mode or if Docker/testcontainers unavailable
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	logger := zap.NewNop()

	// Step 1: Setup PostgreSQL testcontainer
	container, err := tcpostgres.RunContainer(ctx,
		tcpostgres.WithDatabase("user_org_service"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForSQL("5432/tcp", "postgres", func(host string, port nat.Port) string {
				return fmt.Sprintf("postgres://postgres:postgres@%s:%s/user_org_service?sslmode=disable", host, port.Port())
			}).WithStartupTimeout(30*time.Second).WithPollInterval(500*time.Millisecond),
		),
	)
	if err != nil {
		t.Skipf("skipping scope enforcement integration test: testcontainers unavailable: %v", err)
		return
	}
	defer func() {
		_ = container.Terminate(ctx)
	}()

	connString, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// Step 2: Run database migrations
	db, err := sql.Open("postgres", connString)
	require.NoError(t, err)
	defer db.Close()

	_, filename, _, _ := runtime.Caller(0)
	// Path: internal/httpapi/scope_enforcement_test.go -> go up 2 to user-org-service root
	serviceRoot := filepath.Join(filepath.Dir(filename), "..", "..")
	migrationsDir := filepath.Join(serviceRoot, "migrations", "sql")

	require.NoError(t, goose.SetDialect("postgres"))
	require.NoError(t, goose.Up(db, migrationsDir))

	// Step 3: Create runtime with database connection
	pool, err := pgxpool.New(ctx, connString)
	require.NoError(t, err)
	defer pool.Close()

	store := postgres.NewStoreFromPool(pool)

	// For this test, we only need the Postgres store (not the OAuth provider)
	// because we're testing API key authentication (api_keys table), not OAuth2 flows
	rt := &bootstrap.Runtime{
		Postgres: store,
		Provider: nil,                        // API key auth doesn't require fosite provider
		Audit:    audit.NewLoggerEmitter(logger), // Logger-based audit emitter for tests
	}

	// Step 4: Create a test organization and service account
	org, err := store.CreateOrg(ctx, postgres.CreateOrgParams{
		Slug:   "test-org-" + uuid.New().String()[:8],
		Name:   "Test Organization",
		Status: "active",
	})
	require.NoError(t, err)

	desc := "Test service account"
	serviceAccount, err := store.CreateServiceAccount(ctx, postgres.CreateServiceAccountParams{
		OrgID:       org.ID,
		Name:        "test-sa",
		Description: &desc,
		Status:      "active",
	})
	require.NoError(t, err)

	// Step 5: Create API keys with different scopes
	apiKeyInferenceRead := createAPIKey(t, ctx, store, org.ID, serviceAccount.ID, []string{"inference:read"})
	apiKeyOrgAdmin := createAPIKey(t, ctx, store, org.ID, serviceAccount.ID, []string{"org:admin"})
	apiKeyAdmin := createAPIKey(t, ctx, store, org.ID, serviceAccount.ID, []string{"admin"})

	// Step 6: Setup HTTP router with authentication middleware
	router := chi.NewRouter()
	router.Use(middleware.RequireAuth(rt, logger))
	orgs.RegisterRoutes(router, rt, logger)

	// Step 7: Run test cases
	tests := []struct {
		name           string
		apiKey         string
		method         string
		path           string
		body           map[string]interface{}
		expectStatus   int
		expectSuccess  bool
		description    string
	}{
		{
			name:          "inference:read scope cannot create organization",
			apiKey:        apiKeyInferenceRead,
			method:        "POST",
			path:          "/v1/orgs",
			body:          map[string]interface{}{"name": "Blocked Org", "slug": "blocked-org"},
			expectStatus:  http.StatusForbidden,
			expectSuccess: false,
			description:   "API key with only inference:read scope should be denied access to create organizations",
		},
		{
			name:          "org:admin scope can create organization",
			apiKey:        apiKeyOrgAdmin,
			method:        "POST",
			path:          "/v1/orgs",
			body:          map[string]interface{}{"name": "Allowed Org", "slug": "allowed-org-" + uuid.New().String()[:8]},
			expectStatus:  http.StatusCreated,
			expectSuccess: true,
			description:   "API key with org:admin scope should be allowed to create organizations",
		},
		{
			name:          "admin scope can create organization (universal grant)",
			apiKey:        apiKeyAdmin,
			method:        "POST",
			path:          "/v1/orgs",
			body:          map[string]interface{}{"name": "Admin Org", "slug": "admin-org-" + uuid.New().String()[:8]},
			expectStatus:  http.StatusCreated,
			expectSuccess: true,
			description:   "API key with admin scope should have full access (universal grant)",
		},
		{
			name:          "inference:read scope cannot list organizations",
			apiKey:        apiKeyInferenceRead,
			method:        "GET",
			path:          "/v1/orgs",
			body:          nil,
			expectStatus:  http.StatusForbidden,
			expectSuccess: false,
			description:   "API key with only inference:read scope should be denied access to list organizations",
		},
		{
			name:          "org:admin scope can list organizations",
			apiKey:        apiKeyOrgAdmin,
			method:        "GET",
			path:          "/v1/orgs",
			body:          nil,
			expectStatus:  http.StatusOK,
			expectSuccess: true,
			description:   "API key with org:admin scope should be allowed to list organizations",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build request
			var reqBody []byte
			if tt.body != nil {
				reqBody, err = json.Marshal(tt.body)
				require.NoError(t, err)
			}

			req := httptest.NewRequest(tt.method, tt.path, bytes.NewReader(reqBody))
			req.Header.Set("Authorization", "Bearer "+tt.apiKey)
			req.Header.Set("Content-Type", "application/json")

			// Execute request
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			// Verify status code
			assert.Equal(t, tt.expectStatus, rr.Code, tt.description)

			// For forbidden requests, verify error response contains authorization info
			if tt.expectStatus == http.StatusForbidden {
				var errorResp map[string]interface{}
				err := json.Unmarshal(rr.Body.Bytes(), &errorResp)
				require.NoError(t, err, "Expected JSON error response")

				// Verify error message indicates authorization failure (case-insensitive)
				if msg, ok := errorResp["error"].(string); ok {
					msgLower := strings.ToLower(msg)
					assert.Contains(t, msgLower, "forbidden", "Error message should indicate forbidden access")
				}
			}

			// For successful requests, verify response structure
			if tt.expectSuccess {
				assert.Greater(t, rr.Body.Len(), 0, "Expected non-empty response body")
			}
		})
	}
}

// createAPIKey is a helper function to create an API key with specified scopes.
// Returns the plaintext API key (not the database record) for use in authentication headers.
func createAPIKey(t *testing.T, ctx context.Context, store *postgres.Store, orgID, serviceAccountID uuid.UUID, scopes []string) string {
	t.Helper()

	// Generate a random API key (32 bytes = 256 bits of entropy)
	keyBytes := make([]byte, 32)
	_, err := rand.Read(keyBytes)
	require.NoError(t, err)

	// Encode as base64url for API key format
	keyPlaintext := base64.URLEncoding.EncodeToString(keyBytes)
	keyPlaintext = strings.TrimRight(keyPlaintext, "=")

	// Compute SHA-256 fingerprint for storage (same as middleware does)
	hash := sha256.Sum256([]byte(keyPlaintext))
	fingerprint := base64.URLEncoding.EncodeToString(hash[:])
	fingerprint = strings.TrimRight(fingerprint, "=")

	// Store API key in database
	_, err = store.CreateAPIKey(ctx, postgres.CreateAPIKeyParams{
		OrgID:         orgID,
		PrincipalType: postgres.PrincipalTypeServiceAccount,
		PrincipalID:   serviceAccountID,
		Notes:         "test-key-" + uuid.New().String()[:8],
		Fingerprint:   fingerprint,
		Status:        "active",
		Scopes:        scopes,
	})
	require.NoError(t, err)

	// Return plaintext key for use in Authorization headers
	return keyPlaintext
}
