package postgres

import (
	"context"
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/security"
)

func setupStore(t *testing.T) (*Store, func()) {
	t.Helper()

	ctx := context.Background()

	container, err := tcpostgres.RunContainer(ctx,
		tcpostgres.WithDatabase("user_org_service"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		tcpostgres.WithWaitStrategy(wait.ForListeningPort("5432/tcp")),
	)
	require.NoError(t, err)

	connString, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	db, err := sql.Open("postgres", connString)
	require.NoError(t, err)

	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(filename), "..", "..", "..", "..")
	migrationsDir := filepath.Join(projectRoot, "services", "user-org-service", "migrations", "sql")

	require.NoError(t, goose.SetDialect("postgres"))
	require.NoError(t, goose.Up(db, migrationsDir))

	pool, err := pgxpool.New(ctx, connString)
	require.NoError(t, err)

	store := NewStoreFromPool(pool)

	cleanup := func() {
		store.Close()
		_ = db.Close()
		require.NoError(t, container.Terminate(ctx))
	}

	return store, cleanup
}

func TestStoreCreateOrgOptimisticLock(t *testing.T) {
	store, cleanup := setupStore(t)
	defer cleanup()

	ctx := context.Background()

	org, err := store.CreateOrg(ctx, CreateOrgParams{
		Slug:   "acme",
		Name:   "Acme Inc",
		Status: "active",
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), org.Version)

	updated, err := store.UpdateOrg(ctx, UpdateOrgParams{
		ID:      org.ID,
		Version: org.Version,
		Name:    "Acme Corporation",
		Status:  "active",
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), updated.Version)
	require.Equal(t, "Acme Corporation", updated.Name)

	_, err = store.UpdateOrg(ctx, UpdateOrgParams{
		ID:      org.ID,
		Version: org.Version,
		Name:    "Stale Update",
		Status:  "active",
	})
	require.ErrorIs(t, err, ErrOptimisticLock)
}

func TestStoreSessionLifecycle(t *testing.T) {
	store, cleanup := setupStore(t)
	defer cleanup()

	ctx := context.Background()

	org, err := store.CreateOrg(ctx, CreateOrgParams{
		Slug:   "orbit",
		Name:   "Orbit Labs",
		Status: "active",
	})
	require.NoError(t, err)

	passwordHash, err := security.HashPassword("OrbitP@ss!")
	require.NoError(t, err)

	user, err := store.CreateUser(ctx, CreateUserParams{
		OrgID:        org.ID,
		PasswordHash: passwordHash,
		Email:        "user@orbit.io",
		DisplayName:  "Orbit User",
		Status:       "active",
	})
	require.NoError(t, err)

	session, err := store.CreateSession(ctx, CreateSessionParams{
		OrgID:            org.ID,
		UserID:           user.ID,
		RefreshTokenHash: "token-hash",
		ExpiresAt:        time.Now().Add(1 * time.Hour),
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), session.Version)

	now := time.Now().UTC()
	err = store.RevokeSession(ctx, RevokeSessionParams{
		ID:      session.ID,
		Version: session.Version,
		Time:    now,
	}, org.ID)
	require.NoError(t, err)

	err = store.RevokeSession(ctx, RevokeSessionParams{
		ID:      session.ID,
		Version: session.Version,
		Time:    now,
	}, org.ID)
	require.ErrorIs(t, err, ErrOptimisticLock)
}

func TestStoreAPIKeyLifecycle(t *testing.T) {
	store, cleanup := setupStore(t)
	defer cleanup()

	ctx := context.Background()

	org, err := store.CreateOrg(ctx, CreateOrgParams{
		Slug:   "polar",
		Name:   "Polar Industries",
		Status: "active",
	})
	require.NoError(t, err)

	userID := uuid.New()
	passwordHash, err := security.HashPassword("PolarP@ss!")
	require.NoError(t, err)

	user, err := store.CreateUser(ctx, CreateUserParams{
		ID:           userID,
		OrgID:        org.ID,
		PasswordHash: passwordHash,
		Email:        "admin@polar.io",
		DisplayName:  "Polar Admin",
		Status:       "active",
	})
	require.NoError(t, err)
	require.Equal(t, userID, user.ID)

	key, err := store.CreateAPIKey(ctx, CreateAPIKeyParams{
		OrgID:         org.ID,
		PrincipalType: PrincipalTypeUser,
		PrincipalID:   user.ID,
		Fingerprint:   "fp-123",
		Status:        "active",
		Scopes:        []string{"billing.read"},
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), key.Version)
	require.Equal(t, []string{"billing.read"}, key.Scopes)

	revokedAt := time.Now().UTC()
	key, err = store.RevokeAPIKey(ctx, RevokeAPIKeyParams{
		ID:        key.ID,
		Version:   key.Version,
		Status:    "revoked",
		RevokedAt: revokedAt,
	}, org.ID)
	require.NoError(t, err)
	require.Equal(t, int64(2), key.Version)
	require.NotNil(t, key.RevokedAt)

	_, err = store.RevokeAPIKey(ctx, RevokeAPIKeyParams{
		ID:        key.ID,
		Version:   1,
		Status:    "revoked",
		RevokedAt: revokedAt,
	}, org.ID)
	require.ErrorIs(t, err, ErrOptimisticLock)
}
