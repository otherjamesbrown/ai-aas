package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModelAccessRepository(t *testing.T) {
	store, cleanup := setupStore(t)
	if store == nil {
		return // Test was skipped
	}
	defer cleanup()

	ctx := context.Background()

	// Create test org
	org, err := store.CreateOrg(ctx, CreateOrgParams{
		Slug:   "model-access-test-org",
		Name:   "Model Access Test Org",
		Status: "active",
	})
	require.NoError(t, err)

	// Create test user
	user, err := store.CreateUser(ctx, org.ID, CreateUserParams{
		Email:       "testuser@example.com",
		DisplayName: "Test User",
		Status:      "active",
	})
	require.NoError(t, err)

	t.Run("GetUserModelAccess_DefaultsToRestricted", func(t *testing.T) {
		info, err := store.GetUserModelAccess(ctx, org.ID, user.ID)
		require.NoError(t, err)
		assert.Equal(t, "restricted", info.AccessMode)
		assert.Empty(t, info.GrantedModels)
	})

	t.Run("SetUserAccessMode_AutoGrant", func(t *testing.T) {
		access, err := store.SetUserAccessMode(ctx, org.ID, user.ID, "auto_grant")
		require.NoError(t, err)
		assert.Equal(t, "auto_grant", access.AccessMode)
		assert.Equal(t, int64(1), access.Version)

		// Verify via GetUserModelAccess
		info, err := store.GetUserModelAccess(ctx, org.ID, user.ID)
		require.NoError(t, err)
		assert.Equal(t, "auto_grant", info.AccessMode)
	})

	t.Run("SetUserAccessMode_Restricted", func(t *testing.T) {
		access, err := store.SetUserAccessMode(ctx, org.ID, user.ID, "restricted")
		require.NoError(t, err)
		assert.Equal(t, "restricted", access.AccessMode)
		assert.Equal(t, int64(2), access.Version) // Version incremented
	})

	t.Run("CreateModelGrant_Success", func(t *testing.T) {
		grantedBy := user.ID
		grant, err := store.CreateModelGrant(ctx, CreateModelGrantParams{
			OrgID:     org.ID,
			UserID:    user.ID,
			ModelName: "gpt-4",
			GrantedBy: &grantedBy,
		})
		require.NoError(t, err)
		assert.Equal(t, "gpt-4", grant.ModelName)
		assert.NotEqual(t, uuid.Nil, grant.ID)
		assert.NotNil(t, grant.GrantedBy)
		assert.Nil(t, grant.ExpiresAt)
	})

	t.Run("CreateModelGrant_WithExpiry", func(t *testing.T) {
		expiresAt := time.Now().Add(24 * time.Hour)
		grant, err := store.CreateModelGrant(ctx, CreateModelGrantParams{
			OrgID:     org.ID,
			UserID:    user.ID,
			ModelName: "gpt-3.5-turbo",
			ExpiresAt: &expiresAt,
		})
		require.NoError(t, err)
		assert.Equal(t, "gpt-3.5-turbo", grant.ModelName)
		assert.NotNil(t, grant.ExpiresAt)
	})

	t.Run("CreateModelGrant_Conflict", func(t *testing.T) {
		// Try to create duplicate grant
		_, err := store.CreateModelGrant(ctx, CreateModelGrantParams{
			OrgID:     org.ID,
			UserID:    user.ID,
			ModelName: "gpt-4", // Already granted above
		})
		assert.ErrorIs(t, err, ErrConflict)
	})

	t.Run("ListModelGrants_ReturnsActiveGrants", func(t *testing.T) {
		grants, err := store.ListModelGrants(ctx, org.ID, user.ID)
		require.NoError(t, err)
		assert.Len(t, grants, 2) // gpt-4 and gpt-3.5-turbo
	})

	t.Run("CheckModelAccess_RestrictedWithGrant", func(t *testing.T) {
		hasAccess, mode, err := store.CheckModelAccess(ctx, org.ID, user.ID, "gpt-4")
		require.NoError(t, err)
		assert.True(t, hasAccess)
		assert.Equal(t, "restricted", mode)
	})

	t.Run("CheckModelAccess_RestrictedWithoutGrant", func(t *testing.T) {
		hasAccess, mode, err := store.CheckModelAccess(ctx, org.ID, user.ID, "claude-3")
		require.NoError(t, err)
		assert.False(t, hasAccess)
		assert.Equal(t, "restricted", mode)
	})

	t.Run("CheckModelAccess_AutoGrantMode", func(t *testing.T) {
		// Set to auto_grant
		_, err := store.SetUserAccessMode(ctx, org.ID, user.ID, "auto_grant")
		require.NoError(t, err)

		// Any model should be accessible
		hasAccess, mode, err := store.CheckModelAccess(ctx, org.ID, user.ID, "any-model")
		require.NoError(t, err)
		assert.True(t, hasAccess)
		assert.Equal(t, "auto_grant", mode)

		// Reset to restricted for remaining tests
		_, err = store.SetUserAccessMode(ctx, org.ID, user.ID, "restricted")
		require.NoError(t, err)
	})

	t.Run("DeleteModelGrant_Success", func(t *testing.T) {
		err := store.DeleteModelGrant(ctx, org.ID, user.ID, "gpt-3.5-turbo")
		require.NoError(t, err)

		// Verify it's gone
		grants, err := store.ListModelGrants(ctx, org.ID, user.ID)
		require.NoError(t, err)
		assert.Len(t, grants, 1)
		assert.Equal(t, "gpt-4", grants[0].ModelName)
	})

	t.Run("DeleteModelGrant_NotFound", func(t *testing.T) {
		err := store.DeleteModelGrant(ctx, org.ID, user.ID, "nonexistent-model")
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("GrantAllCurrentModels_Success", func(t *testing.T) {
		models := []string{"llama-2", "mistral-7b", "claude-2"}
		grantedBy := user.ID
		count, err := store.GrantAllCurrentModels(ctx, org.ID, user.ID, &grantedBy, models)
		require.NoError(t, err)
		assert.Equal(t, 3, count)

		// Verify grants exist
		grants, err := store.ListModelGrants(ctx, org.ID, user.ID)
		require.NoError(t, err)
		assert.Len(t, grants, 4) // gpt-4 + 3 new models
	})

	t.Run("GrantAllCurrentModels_SkipsDuplicates", func(t *testing.T) {
		models := []string{"gpt-4", "new-model"} // gpt-4 already exists
		count, err := store.GrantAllCurrentModels(ctx, org.ID, user.ID, nil, models)
		require.NoError(t, err)
		assert.Equal(t, 2, count) // Both attempted
	})

	t.Run("GetGrantedModelNames", func(t *testing.T) {
		names, err := store.GetGrantedModelNames(ctx, org.ID, user.ID)
		require.NoError(t, err)
		assert.Contains(t, names, "gpt-4")
		assert.Contains(t, names, "llama-2")
	})

	t.Run("GetUserAccessMode_ReturnsMode", func(t *testing.T) {
		mode, err := store.GetUserAccessMode(ctx, org.ID, user.ID)
		require.NoError(t, err)
		assert.Equal(t, "restricted", mode)
	})

	t.Run("GetUserAccessMode_DefaultsToRestricted", func(t *testing.T) {
		// Create new user without access mode set
		newUser, err := store.CreateUser(ctx, org.ID, CreateUserParams{
			Email:       "newuser@example.com",
			DisplayName: "New User",
			Status:      "active",
		})
		require.NoError(t, err)

		mode, err := store.GetUserAccessMode(ctx, org.ID, newUser.ID)
		require.NoError(t, err)
		assert.Equal(t, "restricted", mode)
	})
}

func TestModelAccessRepository_TenantIsolation(t *testing.T) {
	store, cleanup := setupStore(t)
	if store == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()

	// Create two orgs
	org1, err := store.CreateOrg(ctx, CreateOrgParams{
		Slug:   "org1",
		Name:   "Organization 1",
		Status: "active",
	})
	require.NoError(t, err)

	org2, err := store.CreateOrg(ctx, CreateOrgParams{
		Slug:   "org2",
		Name:   "Organization 2",
		Status: "active",
	})
	require.NoError(t, err)

	// Create users in each org
	user1, err := store.CreateUser(ctx, org1.ID, CreateUserParams{
		Email:       "user@org1.com",
		DisplayName: "User Org1",
		Status:      "active",
	})
	require.NoError(t, err)

	user2, err := store.CreateUser(ctx, org2.ID, CreateUserParams{
		Email:       "user@org2.com",
		DisplayName: "User Org2",
		Status:      "active",
	})
	require.NoError(t, err)

	// Grant model to user1
	_, err = store.CreateModelGrant(ctx, CreateModelGrantParams{
		OrgID:     org1.ID,
		UserID:    user1.ID,
		ModelName: "shared-model",
	})
	require.NoError(t, err)

	// user1 should have access
	hasAccess, _, err := store.CheckModelAccess(ctx, org1.ID, user1.ID, "shared-model")
	require.NoError(t, err)
	assert.True(t, hasAccess)

	// user2 should NOT have access (different org)
	hasAccess, _, err = store.CheckModelAccess(ctx, org2.ID, user2.ID, "shared-model")
	require.NoError(t, err)
	assert.False(t, hasAccess)

	// Listing grants for user2 should be empty
	grants, err := store.ListModelGrants(ctx, org2.ID, user2.ID)
	require.NoError(t, err)
	assert.Empty(t, grants)
}
