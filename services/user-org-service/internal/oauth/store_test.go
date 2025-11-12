package oauth

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/google/uuid"
	"github.com/ory/fosite"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	testcontainers "github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/security"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/storage/postgres"
)

func setupOAuthStore(t *testing.T) (*Store, func()) {
	t.Helper()

	ctx := context.Background()

	provider, err := newDockerProviderSafe()
	if err != nil {
		t.Skipf("skipping OAuth store integration tests: docker unavailable: %v", err)
		return nil, nil
	}
	if provider != nil {
		require.NoError(t, provider.Close())
	}
	container, err := tcpostgres.RunContainer(ctx,
		tcpostgres.WithDatabase("user_org_service"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
	)
	if err != nil {
		t.Skipf("skipping OAuth store integration tests: failed to start postgres container: %v", err)
		return nil, nil
	}

	connString, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	db, err := sql.Open("postgres", connString)
	require.NoError(t, err)

	require.NoError(t, goose.SetDialect("postgres"))

	migrationsDir := "../../../../services/user-org-service/migrations/sql"
	require.NoError(t, goose.Up(db, migrationsDir))

	store, err := postgres.NewStore(ctx, connString)
	require.NoError(t, err)

	return &Store{Store: store}, func() {
		store.Close()
		_ = db.Close()
		require.NoError(t, container.Terminate(ctx))
	}
}

func newDockerProviderSafe() (*testcontainers.DockerProvider, error) {
	var (
		provider *testcontainers.DockerProvider
		err      error
	)
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("docker provider initialization failed: %v", r)
		}
	}()
	provider, err = testcontainers.NewDockerProvider()
	return provider, err
}

func newTestRequest(orgID uuid.UUID) (*fosite.Request, *Session) {
	userID := uuid.New()
	session := &Session{
		OrgID:  orgID.String(),
		UserID: userID.String(),
	}
	session.Subject = "user"
	session.SetExpiresAt(fosite.AccessToken, time.Now().Add(1*time.Hour))
	session.SetExpiresAt(fosite.RefreshToken, time.Now().Add(24*time.Hour))
	session.SetExpiresAt(fosite.AuthorizeCode, time.Now().Add(5*time.Minute))

	req := fosite.NewRequest()
	req.ID = uuid.NewString()
	req.RequestedAt = time.Now().UTC()
	req.Client = &fosite.DefaultClient{ID: "client-123"}
	req.Form = url.Values{"response_type": {"code"}, "redirect_uri": {"https://example.com/callback"}}
	req.Session = session
	req.RequestedScope = fosite.Arguments{"openid", "profile"}
	req.GrantedScope = fosite.Arguments{"openid"}
	req.RequestedAudience = fosite.Arguments{"api"}
	req.GrantedAudience = fosite.Arguments{"api"}

	return req, session
}

func TestAuthorizeCodeLifecycle(t *testing.T) {
	store, cleanup := setupOAuthStore(t)
	defer cleanup()

	orgID := uuid.New()
	req, _ := newTestRequest(orgID)

	err := store.CreateAuthorizeCodeSession(context.Background(), "auth-code", req)
	require.NoError(t, err)

	got, err := store.GetAuthorizeCodeSession(context.Background(), "auth-code", nil)
	require.NoError(t, err)
	require.Equal(t, req.GetID(), got.GetID())
	require.Equal(t, req.GetClient().GetID(), got.GetClient().GetID())

	require.NoError(t, store.InvalidateAuthorizeCodeSession(context.Background(), "auth-code"))

	_, err = store.GetAuthorizeCodeSession(context.Background(), "auth-code", nil)
	require.ErrorIs(t, err, fosite.ErrNotFound)
}

func TestAccessTokenLifecycle(t *testing.T) {
	store, cleanup := setupOAuthStore(t)
	defer cleanup()

	orgID := uuid.New()
	req, _ := newTestRequest(orgID)

	err := store.CreateAccessTokenSession(context.Background(), "access", req)
	require.NoError(t, err)

	got, err := store.GetAccessTokenSession(context.Background(), "access", nil)
	require.NoError(t, err)
	require.Equal(t, req.GetID(), got.GetID())

	require.NoError(t, store.DeleteAccessTokenSession(context.Background(), "access"))

	_, err = store.GetAccessTokenSession(context.Background(), "access", nil)
	require.ErrorIs(t, err, fosite.ErrNotFound)
}

func TestRefreshTokenLifecycle(t *testing.T) {
	store, cleanup := setupOAuthStore(t)
	defer cleanup()

	orgID := uuid.New()
	req, _ := newTestRequest(orgID)

	err := store.CreateRefreshTokenSession(context.Background(), "refresh", req)
	require.NoError(t, err)

	got, err := store.GetRefreshTokenSession(context.Background(), "refresh", nil)
	require.NoError(t, err)
	require.Equal(t, req.GetID(), got.GetID())

	require.NoError(t, store.DeleteRefreshTokenSession(context.Background(), "refresh"))

	_, err = store.GetRefreshTokenSession(context.Background(), "refresh", nil)
	require.ErrorIs(t, err, fosite.ErrNotFound)

	// Re-create and revoke via requestID.
	require.NoError(t, store.CreateRefreshTokenSession(context.Background(), "refresh2", req))
	require.NoError(t, store.RevokeRefreshToken(context.Background(), req.GetID()))
	_, err = store.GetRefreshTokenSession(context.Background(), "refresh2", nil)
	require.ErrorIs(t, err, fosite.ErrNotFound)
}

func TestPKCELifecycle(t *testing.T) {
	store, cleanup := setupOAuthStore(t)
	defer cleanup()

	orgID := uuid.New()
	req, _ := newTestRequest(orgID)
	req.Form.Set("code_challenge", "abc")
	req.Form.Set("code_challenge_method", "S256")

	require.NoError(t, store.CreatePKCERequestSession(context.Background(), "pkce", req))

	got, err := store.GetPKCERequestSession(context.Background(), "pkce", nil)
	require.NoError(t, err)
	require.Equal(t, req.GetID(), got.GetID())

	require.NoError(t, store.DeletePKCERequestSession(context.Background(), "pkce"))
	_, err = store.GetPKCERequestSession(context.Background(), "pkce", nil)
	require.ErrorIs(t, err, fosite.ErrNotFound)
}

func TestSessionRoundTripPreservesFields(t *testing.T) {
	store, cleanup := setupOAuthStore(t)
	defer cleanup()

	orgID := uuid.New()
	req, session := newTestRequest(orgID)
	session.Subject = "user-subject"

	passwordHash, err := security.HashPassword("Sup3rSecret!")
	require.NoError(t, err)

	if typed, ok := req.Session.(*Session); ok {
		typed.DefaultSession.Extra = map[string]any{"password_hash": passwordHash}
		typed.SetExpiresAt(fosite.RefreshToken, time.Now().Add(6*time.Hour))
	}

	require.NoError(t, store.CreateRefreshTokenSession(context.Background(), "refresh-sig", req))

	got, err := store.GetRefreshTokenSession(context.Background(), "refresh-sig", nil)
	require.NoError(t, err)

	gotSession, ok := got.GetSession().(*Session)
	require.True(t, ok)
	require.Equal(t, session.Subject, gotSession.Subject)
	require.Equal(t, session.OrgID, gotSession.OrgID)
	require.Equal(t, session.UserID, gotSession.UserID)
	require.WithinDuration(t, session.GetExpiresAt(fosite.RefreshToken), gotSession.GetExpiresAt(fosite.RefreshToken), time.Second)
}
