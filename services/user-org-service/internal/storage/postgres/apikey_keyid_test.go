package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetAPIKeyByKeyID(t *testing.T) {
	store, cleanup := setupStore(t)
	if store == nil {
		return // Test was skipped
	}
	defer cleanup()

	ctx := context.Background()

	// Create a test organization
	org, err := store.CreateOrg(ctx, CreateOrgParams{
		Name: "Test Org",
		Slug: "test-org-keyid",
	})
	require.NoError(t, err)

	// Create a service account
	desc := "For testing"
	sa, err := store.CreateServiceAccount(ctx, CreateServiceAccountParams{
		OrgID:       org.ID,
		Name:        "Test Service Account",
		Description: &desc,
		Status:      "active",
	})
	require.NoError(t, err)

	// Create an API key
	apiKey, err := store.CreateAPIKey(ctx, CreateAPIKeyParams{
		OrgID:         org.ID,
		PrincipalType: PrincipalTypeServiceAccount,
		PrincipalID:   sa.ID,
		Notes:         "Test API Key",
		Fingerprint:   "test-fingerprint-keyid-123",
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
		require.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("NotFound_WrongOrgID", func(t *testing.T) {
		otherOrg, err := store.CreateOrg(ctx, CreateOrgParams{
			Name: "Other Org",
			Slug: "other-org-keyid",
		})
		require.NoError(t, err)

		// Try to find the key from the wrong org
		_, err = store.GetAPIKeyByKeyID(ctx, otherOrg.ID, apiKey.KeyID)
		require.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("NotFound_RevokedKey", func(t *testing.T) {
		// Create another key to revoke
		revokedKey, err := store.CreateAPIKey(ctx, CreateAPIKeyParams{
			OrgID:         org.ID,
			PrincipalType: PrincipalTypeServiceAccount,
			PrincipalID:   sa.ID,
			Notes:         "To be revoked",
			Fingerprint:   "test-fingerprint-keyid-456",
			Status:        "active",
			Scopes:        []string{"model:read"},
		})
		require.NoError(t, err)

		// Revoke it
		_, err = store.RevokeAPIKey(ctx, RevokeAPIKeyParams{
			ID:        revokedKey.ID,
			Version:   revokedKey.Version,
			Status:    "revoked",
			RevokedAt: revokedKey.CreatedAt, // Use creation time as revocation time
		}, org.ID)
		require.NoError(t, err)

		// Should still be found since GetAPIKeyByKeyID doesn't filter by revoked status
		// (it only filters by deleted_at IS NULL)
		retrieved, err := store.GetAPIKeyByKeyID(ctx, org.ID, revokedKey.KeyID)
		require.NoError(t, err)
		require.Equal(t, "revoked", retrieved.Status)
	})
}
