package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenPolicyRepository(t *testing.T) {
	store, cleanup := setupStore(t)
	if store == nil {
		return // Test was skipped
	}
	defer cleanup()

	ctx := context.Background()

	// Create test org
	org, err := store.CreateOrg(ctx, CreateOrgParams{
		Slug:   "token-policy-test-org",
		Name:   "Token Policy Test Org",
		Status: "active",
	})
	require.NoError(t, err)

	// Create test user
	user, err := store.CreateUser(ctx, CreateUserParams{
		OrgID:        org.ID,
		Email:        "testuser@example.com",
		DisplayName:  "Test User",
		PasswordHash: "test-hash",
		Status:       "active",
	})
	require.NoError(t, err)

	t.Run("GetBuiltinNoLimitPolicy_ReturnsSystemPolicy", func(t *testing.T) {
		policy, err := store.GetBuiltinNoLimitPolicy(ctx)
		require.NoError(t, err)
		assert.Equal(t, NoTokenRateLimitPolicyID, policy.ID)
		assert.Equal(t, "No Token Rate-Limit", policy.Name)
		assert.True(t, policy.IsBuiltin)
		assert.Nil(t, policy.OrgID)
		assert.Nil(t, policy.Limit1h)
		assert.Nil(t, policy.Limit24h)
		assert.Nil(t, policy.Limit7d)
	})

	var createdPolicyID uuid.UUID

	t.Run("CreateTokenPolicy_Success", func(t *testing.T) {
		limit1h := int64(10000)
		limit24h := int64(100000)
		limit7d := int64(500000)
		desc := "Standard usage limits"

		policy, err := store.CreateTokenPolicy(ctx, CreateTokenPolicyParams{
			OrgID:       org.ID,
			Name:        "standard",
			Description: &desc,
			Limit1h:     &limit1h,
			Limit24h:    &limit24h,
			Limit7d:     &limit7d,
		})
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, policy.ID)
		assert.Equal(t, "standard", policy.Name)
		assert.Equal(t, &desc, policy.Description)
		assert.Equal(t, &limit1h, policy.Limit1h)
		assert.Equal(t, &limit24h, policy.Limit24h)
		assert.Equal(t, &limit7d, policy.Limit7d)
		assert.False(t, policy.IsBuiltin)
		assert.Equal(t, &org.ID, policy.OrgID)

		createdPolicyID = policy.ID
	})

	t.Run("CreateTokenPolicy_PartialLimits", func(t *testing.T) {
		limit1h := int64(5000)

		policy, err := store.CreateTokenPolicy(ctx, CreateTokenPolicyParams{
			OrgID:   org.ID,
			Name:    "burst-control",
			Limit1h: &limit1h,
		})
		require.NoError(t, err)
		assert.Equal(t, "burst-control", policy.Name)
		assert.Equal(t, &limit1h, policy.Limit1h)
		assert.Nil(t, policy.Limit24h)
		assert.Nil(t, policy.Limit7d)
	})

	t.Run("CreateTokenPolicy_NoLimits_Fails", func(t *testing.T) {
		_, err := store.CreateTokenPolicy(ctx, CreateTokenPolicyParams{
			OrgID: org.ID,
			Name:  "no-limits-policy",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one limit")
	})

	t.Run("CreateTokenPolicy_ReservedName_Fails", func(t *testing.T) {
		limit1h := int64(1000)
		_, err := store.CreateTokenPolicy(ctx, CreateTokenPolicyParams{
			OrgID:   org.ID,
			Name:    "No Token Rate-Limit",
			Limit1h: &limit1h,
		})
		require.ErrorIs(t, err, ErrConflict)
	})

	t.Run("CreateTokenPolicy_DuplicateName_Fails", func(t *testing.T) {
		limit1h := int64(1000)
		_, err := store.CreateTokenPolicy(ctx, CreateTokenPolicyParams{
			OrgID:   org.ID,
			Name:    "standard", // Already exists
			Limit1h: &limit1h,
		})
		require.ErrorIs(t, err, ErrConflict)
	})

	t.Run("GetTokenPolicy_Success", func(t *testing.T) {
		policy, err := store.GetTokenPolicy(ctx, createdPolicyID)
		require.NoError(t, err)
		assert.Equal(t, createdPolicyID, policy.ID)
		assert.Equal(t, "standard", policy.Name)
	})

	t.Run("GetTokenPolicy_NotFound", func(t *testing.T) {
		_, err := store.GetTokenPolicy(ctx, uuid.New())
		require.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("GetTokenPolicyByName_Success", func(t *testing.T) {
		policy, err := store.GetTokenPolicyByName(ctx, org.ID, "standard")
		require.NoError(t, err)
		assert.Equal(t, "standard", policy.Name)
		assert.Equal(t, createdPolicyID, policy.ID)
	})

	t.Run("GetTokenPolicyByName_BuiltinPolicy", func(t *testing.T) {
		policy, err := store.GetTokenPolicyByName(ctx, org.ID, "No Token Rate-Limit")
		require.NoError(t, err)
		assert.Equal(t, NoTokenRateLimitPolicyID, policy.ID)
		assert.True(t, policy.IsBuiltin)
	})

	t.Run("GetTokenPolicyByName_NotFound", func(t *testing.T) {
		_, err := store.GetTokenPolicyByName(ctx, org.ID, "nonexistent")
		require.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("ListTokenPolicies_IncludesBuiltin", func(t *testing.T) {
		policies, err := store.ListTokenPolicies(ctx, org.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(policies), 3) // builtin + standard + burst-control

		// Built-in should be first (sorted by is_builtin DESC)
		assert.True(t, policies[0].IsBuiltin)
		assert.Equal(t, "No Token Rate-Limit", policies[0].Name)
	})

	t.Run("UpdateTokenPolicy_Success", func(t *testing.T) {
		newLimit1h := int64(20000)
		newName := "standard-v2"

		policy, err := store.UpdateTokenPolicy(ctx, UpdateTokenPolicyParams{
			ID:      createdPolicyID,
			Name:    &newName,
			Limit1h: &newLimit1h,
		})
		require.NoError(t, err)
		assert.Equal(t, "standard-v2", policy.Name)
		assert.Equal(t, &newLimit1h, policy.Limit1h)

		// Restore original name for other tests
		originalName := "standard"
		_, err = store.UpdateTokenPolicy(ctx, UpdateTokenPolicyParams{
			ID:   createdPolicyID,
			Name: &originalName,
		})
		require.NoError(t, err)
	})

	t.Run("UpdateTokenPolicy_ClearLimit", func(t *testing.T) {
		policy, err := store.UpdateTokenPolicy(ctx, UpdateTokenPolicyParams{
			ID:           createdPolicyID,
			ClearLimit7d: true,
		})
		require.NoError(t, err)
		assert.Nil(t, policy.Limit7d)
	})

	t.Run("UpdateTokenPolicy_Builtin_Fails", func(t *testing.T) {
		newName := "Renamed"
		_, err := store.UpdateTokenPolicy(ctx, UpdateTokenPolicyParams{
			ID:   NoTokenRateLimitPolicyID,
			Name: &newName,
		})
		require.ErrorIs(t, err, ErrForbidden)
	})

	t.Run("UpdateTokenPolicy_ReservedName_Fails", func(t *testing.T) {
		reservedName := "No Token Rate-Limit"
		_, err := store.UpdateTokenPolicy(ctx, UpdateTokenPolicyParams{
			ID:   createdPolicyID,
			Name: &reservedName,
		})
		require.ErrorIs(t, err, ErrConflict)
	})

	t.Run("SetOrgDefaultTokenPolicy_Success", func(t *testing.T) {
		err := store.SetOrgDefaultTokenPolicy(ctx, org.ID, createdPolicyID)
		require.NoError(t, err)

		// Verify
		policy, err := store.GetOrgDefaultTokenPolicy(ctx, org.ID)
		require.NoError(t, err)
		assert.Equal(t, createdPolicyID, policy.ID)
	})

	t.Run("GetOrgDefaultTokenPolicy_DefaultsToNoLimit", func(t *testing.T) {
		// Create new org without setting default
		newOrg, err := store.CreateOrg(ctx, CreateOrgParams{
			Slug:   "new-org-no-default",
			Name:   "New Org No Default",
			Status: "active",
		})
		require.NoError(t, err)

		policy, err := store.GetOrgDefaultTokenPolicy(ctx, newOrg.ID)
		require.NoError(t, err)
		assert.Equal(t, NoTokenRateLimitPolicyID, policy.ID)
	})

	t.Run("SetOrgDefaultTokenPolicy_CrossOrgPolicy_Fails", func(t *testing.T) {
		// Create another org with its own policy
		otherOrg, err := store.CreateOrg(ctx, CreateOrgParams{
			Slug:   "other-org",
			Name:   "Other Org",
			Status: "active",
		})
		require.NoError(t, err)

		limit := int64(1000)
		otherPolicy, err := store.CreateTokenPolicy(ctx, CreateTokenPolicyParams{
			OrgID:   otherOrg.ID,
			Name:    "other-policy",
			Limit1h: &limit,
		})
		require.NoError(t, err)

		// Try to set other org's policy as default - should fail
		err = store.SetOrgDefaultTokenPolicy(ctx, org.ID, otherPolicy.ID)
		require.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("SetUserTokenPolicyOverride_Success", func(t *testing.T) {
		err := store.SetUserTokenPolicyOverride(ctx, org.ID, user.ID, createdPolicyID)
		require.NoError(t, err)

		// Verify effective policy
		effective, err := store.GetEffectiveTokenPolicy(ctx, org.ID, user.ID)
		require.NoError(t, err)
		assert.Equal(t, createdPolicyID, effective.Policy.ID)
		assert.Equal(t, "override", effective.Source)
	})

	t.Run("GetEffectiveTokenPolicy_Override", func(t *testing.T) {
		effective, err := store.GetEffectiveTokenPolicy(ctx, org.ID, user.ID)
		require.NoError(t, err)
		assert.Equal(t, createdPolicyID, effective.Policy.ID)
		assert.Equal(t, "override", effective.Source)
	})

	t.Run("ClearUserTokenPolicyOverride_Success", func(t *testing.T) {
		err := store.ClearUserTokenPolicyOverride(ctx, org.ID, user.ID)
		require.NoError(t, err)

		// Should now inherit org default
		effective, err := store.GetEffectiveTokenPolicy(ctx, org.ID, user.ID)
		require.NoError(t, err)
		assert.Equal(t, createdPolicyID, effective.Policy.ID) // Org default is createdPolicyID
		assert.Equal(t, "inherited", effective.Source)
	})

	t.Run("GetEffectiveTokenPolicy_Inherited", func(t *testing.T) {
		effective, err := store.GetEffectiveTokenPolicy(ctx, org.ID, user.ID)
		require.NoError(t, err)
		assert.Equal(t, "inherited", effective.Source)
	})

	t.Run("GetEffectiveTokenPolicyByUserID_Success", func(t *testing.T) {
		effective, err := store.GetEffectiveTokenPolicyByUserID(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, createdPolicyID, effective.Policy.ID)
		assert.Equal(t, "inherited", effective.Source)
	})

	t.Run("SetUserTokenPolicyOverride_BuiltinPolicy", func(t *testing.T) {
		// User can be assigned the "No Token Rate-Limit" policy
		err := store.SetUserTokenPolicyOverride(ctx, org.ID, user.ID, NoTokenRateLimitPolicyID)
		require.NoError(t, err)

		effective, err := store.GetEffectiveTokenPolicy(ctx, org.ID, user.ID)
		require.NoError(t, err)
		assert.Equal(t, NoTokenRateLimitPolicyID, effective.Policy.ID)
		assert.Equal(t, "override", effective.Source)

		// Clear for subsequent tests
		err = store.ClearUserTokenPolicyOverride(ctx, org.ID, user.ID)
		require.NoError(t, err)
	})

	t.Run("DeleteTokenPolicy_InUseAsOrgDefault_Fails", func(t *testing.T) {
		// createdPolicyID is currently org default
		err := store.DeleteTokenPolicy(ctx, createdPolicyID)
		require.ErrorIs(t, err, ErrConflict)
	})

	t.Run("DeleteTokenPolicy_InUseAsUserOverride_Fails", func(t *testing.T) {
		// Set user override to a different policy first
		limit := int64(2000)
		tempPolicy, err := store.CreateTokenPolicy(ctx, CreateTokenPolicyParams{
			OrgID:   org.ID,
			Name:    "temp-policy",
			Limit1h: &limit,
		})
		require.NoError(t, err)

		// Reset org default to builtin
		err = store.SetOrgDefaultTokenPolicy(ctx, org.ID, NoTokenRateLimitPolicyID)
		require.NoError(t, err)

		// Set user override
		err = store.SetUserTokenPolicyOverride(ctx, org.ID, user.ID, tempPolicy.ID)
		require.NoError(t, err)

		// Try to delete - should fail
		err = store.DeleteTokenPolicy(ctx, tempPolicy.ID)
		require.ErrorIs(t, err, ErrConflict)

		// Cleanup
		err = store.ClearUserTokenPolicyOverride(ctx, org.ID, user.ID)
		require.NoError(t, err)
	})

	t.Run("DeleteTokenPolicy_Builtin_Fails", func(t *testing.T) {
		err := store.DeleteTokenPolicy(ctx, NoTokenRateLimitPolicyID)
		require.ErrorIs(t, err, ErrForbidden)
	})

	t.Run("DeleteTokenPolicy_Success", func(t *testing.T) {
		// Create policy to delete
		limit := int64(1000)
		policy, err := store.CreateTokenPolicy(ctx, CreateTokenPolicyParams{
			OrgID:   org.ID,
			Name:    "to-delete",
			Limit1h: &limit,
		})
		require.NoError(t, err)

		err = store.DeleteTokenPolicy(ctx, policy.ID)
		require.NoError(t, err)

		// Verify it's gone
		_, err = store.GetTokenPolicy(ctx, policy.ID)
		require.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("DeleteTokenPolicy_NotFound", func(t *testing.T) {
		err := store.DeleteTokenPolicy(ctx, uuid.New())
		require.ErrorIs(t, err, ErrNotFound)
	})
}

func TestTokenPolicyRepository_TenantIsolation(t *testing.T) {
	store, cleanup := setupStore(t)
	if store == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()

	// Create two orgs
	org1, err := store.CreateOrg(ctx, CreateOrgParams{
		Slug:   "policy-org1",
		Name:   "Policy Org 1",
		Status: "active",
	})
	require.NoError(t, err)

	org2, err := store.CreateOrg(ctx, CreateOrgParams{
		Slug:   "policy-org2",
		Name:   "Policy Org 2",
		Status: "active",
	})
	require.NoError(t, err)

	// Create policies in each org with same name
	limit := int64(5000)
	policy1, err := store.CreateTokenPolicy(ctx, CreateTokenPolicyParams{
		OrgID:   org1.ID,
		Name:    "shared-name",
		Limit1h: &limit,
	})
	require.NoError(t, err)

	policy2, err := store.CreateTokenPolicy(ctx, CreateTokenPolicyParams{
		OrgID:   org2.ID,
		Name:    "shared-name",
		Limit1h: &limit,
	})
	require.NoError(t, err)

	// Policies should have different IDs
	assert.NotEqual(t, policy1.ID, policy2.ID)

	// GetTokenPolicyByName should return the correct org's policy
	found1, err := store.GetTokenPolicyByName(ctx, org1.ID, "shared-name")
	require.NoError(t, err)
	assert.Equal(t, policy1.ID, found1.ID)

	found2, err := store.GetTokenPolicyByName(ctx, org2.ID, "shared-name")
	require.NoError(t, err)
	assert.Equal(t, policy2.ID, found2.ID)

	// ListTokenPolicies should only return org's own policies + builtin
	list1, err := store.ListTokenPolicies(ctx, org1.ID)
	require.NoError(t, err)
	policyIDs := make(map[uuid.UUID]bool)
	for _, p := range list1 {
		policyIDs[p.ID] = true
	}
	assert.True(t, policyIDs[policy1.ID])
	assert.True(t, policyIDs[NoTokenRateLimitPolicyID])
	assert.False(t, policyIDs[policy2.ID])
}
