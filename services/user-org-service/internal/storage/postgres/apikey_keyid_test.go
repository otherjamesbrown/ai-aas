package postgres_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/storage/postgres"
)

func TestGetAPIKeyByKeyID(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create a test organization
	org, err := store.CreateOrg(ctx, postgres.CreateOrgParams{
		Name: "Test Org",
		Slug: "test-org",
	})
	require.NoError(t, err)

	// Create a service account
	sa, err := store.CreateServiceAccount(ctx, postgres.CreateServiceAccountParams{
		OrgID:       org.ID,
		Name:        "Test Service Account",
		Description: "For testing",
		Status:      "active",
	})
	require.NoError(t, err)

	// Create an API key
	apiKey, err := store.CreateAPIKey(ctx, postgres.CreateAPIKeyParams{
		OrgID:         org.ID,
		PrincipalType: postgres.PrincipalTypeServiceAccount,
		PrincipalID:   sa.ID,
		Notes:         "Test API Key",
		Fingerprint:   "test-fingerprint-123",
		Status:        "active",
		Scopes:        []string{"model:read", "model:write"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, apiKey.KeyID, "KeyID should be generated")

	t.Run("Success", func(t *testing.T) {
		// Look up by KeyID
		retrieved, err := store.GetAPIKeyByKeyID(ctx, org.ID, apiKey.KeyID)
		require.NoError(t, err)
		require.Equal(t, apiKey.ID, retrieved.ID)
		require.Equal(t, apiKey.KeyID, retrieved.KeyID)
		require.Equal(t, apiKey.Fingerprint, retrieved.Fingerprint)
		require.Equal(t, apiKey.Status, retrieved.Status)
		require.Equal(t, apiKey.OrgID, retrieved.OrgID)
	})

	t.Run("NotFound_WrongKeyID", func(t *testing.T) {
		_, err := store.GetAPIKeyByKeyID(ctx, org.ID, "ak_nonexistent")
		require.ErrorIs(t, err, postgres.ErrNotFound)
	})

	t.Run("NotFound_WrongOrgID", func(t *testing.T) {
		otherOrg, err := store.CreateOrg(ctx, postgres.CreateOrgParams{
			Name: "Other Org",
			Slug: "other-org",
		})
		require.NoError(t, err)

		// Try to find the key from the wrong org
		_, err = store.GetAPIKeyByKeyID(ctx, otherOrg.ID, apiKey.KeyID)
		require.ErrorIs(t, err, postgres.ErrNotFound)
	})

	t.Run("NotFound_DeletedKey", func(t *testing.T) {
		// Create another key to delete
		deletedKey, err := store.CreateAPIKey(ctx, postgres.CreateAPIKeyParams{
			OrgID:         org.ID,
			PrincipalType: postgres.PrincipalTypeServiceAccount,
			PrincipalID:   sa.ID,
			Notes:         "To be deleted",
			Fingerprint:   "test-fingerprint-456",
			Status:        "active",
			Scopes:        []string{"model:read"},
		})
		require.NoError(t, err)

		// Soft-delete it
		err = store.DeleteAPIKey(ctx, deletedKey.ID, org.ID)
		require.NoError(t, err)

		// Should not be found by KeyID
		_, err = store.GetAPIKeyByKeyID(ctx, org.ID, deletedKey.KeyID)
		require.ErrorIs(t, err, postgres.ErrNotFound)
	})
}
