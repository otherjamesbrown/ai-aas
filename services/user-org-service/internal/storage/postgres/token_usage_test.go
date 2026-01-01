package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenUsageRepository(t *testing.T) {
	store, cleanup := setupStore(t)
	if store == nil {
		return // Test was skipped
	}
	defer cleanup()

	ctx := context.Background()

	// Create test org
	org, err := store.CreateOrg(ctx, CreateOrgParams{
		Slug:   "token-usage-test-org",
		Name:   "Token Usage Test Org",
		Status: "active",
	})
	require.NoError(t, err)

	// Create test user
	user, err := store.CreateUser(ctx, CreateUserParams{
		OrgID:        org.ID,
		Email:        "usage-test@example.com",
		DisplayName:  "Usage Test User",
		PasswordHash: "test-hash",
		Status:       "active",
	})
	require.NoError(t, err)

	// Create a policy with limits for testing
	limit1h := int64(10000)
	limit24h := int64(100000)
	limit7d := int64(500000)
	policy, err := store.CreateTokenPolicy(ctx, CreateTokenPolicyParams{
		OrgID:    org.ID,
		Name:     "test-policy",
		Limit1h:  &limit1h,
		Limit24h: &limit24h,
		Limit7d:  &limit7d,
	})
	require.NoError(t, err)

	// Set the policy as org default
	err = store.SetOrgDefaultTokenPolicy(ctx, org.ID, policy.ID)
	require.NoError(t, err)

	t.Run("RecordTokenUsage_CreatesWindows", func(t *testing.T) {
		err := store.RecordTokenUsage(ctx, user.ID, 1000)
		require.NoError(t, err)

		// Check usage was recorded
		usage, err := store.GetTokenUsage(ctx, user.ID)
		require.NoError(t, err)

		// Should have records for all windows that apply (1h, 24h, 7d)
		assert.GreaterOrEqual(t, len(usage), 1)

		// At least one window should have tokens_used > 0
		var foundTokens bool
		for _, u := range usage {
			if u.TokensUsed > 0 {
				foundTokens = true
				break
			}
		}
		assert.True(t, foundTokens, "Should have recorded tokens in at least one window")
	})

	t.Run("RecordTokenUsage_AccumulatesTokens", func(t *testing.T) {
		// Record additional usage
		err := store.RecordTokenUsage(ctx, user.ID, 500)
		require.NoError(t, err)

		usage, err := store.GetTokenUsage(ctx, user.ID)
		require.NoError(t, err)

		// Should have accumulated usage
		var total int64
		for _, u := range usage {
			if u.WindowType == WindowType1h {
				total = u.TokensUsed
				break
			}
		}
		// Previous 1000 + 500 = 1500 (at minimum)
		assert.GreaterOrEqual(t, total, int64(1500))
	})

	t.Run("GetTokenUsageForPolicy_ReturnsStatus", func(t *testing.T) {
		status, err := store.GetTokenUsageForPolicy(ctx, user.ID, policy)
		require.NoError(t, err)

		// Should have entries for each window with a limit
		assert.Len(t, status, 3) // 1h, 24h, 7d all have limits

		// Check each window has expected structure
		for _, s := range status {
			assert.True(t, s.Window == WindowType1h || s.Window == WindowType24h || s.Window == WindowType7d)
			assert.Greater(t, s.Limit, int64(0))
			assert.GreaterOrEqual(t, s.Used, int64(0))
			assert.Equal(t, s.Limit-s.Used, s.Remaining)
			assert.GreaterOrEqual(t, s.Percentage, float64(0))
			assert.LessOrEqual(t, s.Percentage, float64(100))
			assert.False(t, s.ResetsAt.IsZero())
		}
	})

	t.Run("GetTokenUsageForPolicy_PartialLimits", func(t *testing.T) {
		// Create policy with only 1h limit
		hourlyLimit := int64(5000)
		partialPolicy, err := store.CreateTokenPolicy(ctx, CreateTokenPolicyParams{
			OrgID:   org.ID,
			Name:    "hourly-only",
			Limit1h: &hourlyLimit,
		})
		require.NoError(t, err)

		status, err := store.GetTokenUsageForPolicy(ctx, user.ID, partialPolicy)
		require.NoError(t, err)

		// Should only have 1h window
		assert.Len(t, status, 1)
		assert.Equal(t, WindowType1h, status[0].Window)
	})

	t.Run("GetTokenUsageForPolicy_NoLimitPolicy", func(t *testing.T) {
		noLimit, err := store.GetBuiltinNoLimitPolicy(ctx)
		require.NoError(t, err)

		status, err := store.GetTokenUsageForPolicy(ctx, user.ID, noLimit)
		require.NoError(t, err)

		// No limits = no status entries
		assert.Empty(t, status)
	})

	t.Run("CheckRateLimitStatus_Allowed", func(t *testing.T) {
		allowed, windows, blocking, err := store.CheckRateLimitStatus(ctx, user.ID)
		require.NoError(t, err)

		assert.True(t, allowed)
		assert.NotEmpty(t, windows)
		assert.Nil(t, blocking)
	})

	t.Run("CheckRateLimitStatus_NoLimitPolicy", func(t *testing.T) {
		// Create user with no limit policy
		noLimitUser, err := store.CreateUser(ctx, CreateUserParams{
			OrgID:        org.ID,
			Email:        "nolimit@example.com",
			DisplayName:  "No Limit User",
			PasswordHash: "test-hash",
			Status:       "active",
		})
		require.NoError(t, err)

		// Set user override to No Token Rate-Limit
		err = store.SetUserTokenPolicyOverride(ctx, org.ID, noLimitUser.ID, NoTokenRateLimitPolicyID)
		require.NoError(t, err)

		allowed, windows, blocking, err := store.CheckRateLimitStatus(ctx, noLimitUser.ID)
		require.NoError(t, err)

		assert.True(t, allowed)
		assert.Empty(t, windows)
		assert.Nil(t, blocking)
	})

	t.Run("CheckRateLimitStatus_Blocked", func(t *testing.T) {
		// Create user with a very low limit
		lowLimitUser, err := store.CreateUser(ctx, CreateUserParams{
			OrgID:        org.ID,
			Email:        "lowlimit@example.com",
			DisplayName:  "Low Limit User",
			PasswordHash: "test-hash",
			Status:       "active",
		})
		require.NoError(t, err)

		// Create policy with very low limit
		tinyLimit := int64(100)
		lowPolicy, err := store.CreateTokenPolicy(ctx, CreateTokenPolicyParams{
			OrgID:   org.ID,
			Name:    "tiny-limit",
			Limit1h: &tinyLimit,
		})
		require.NoError(t, err)

		err = store.SetUserTokenPolicyOverride(ctx, org.ID, lowLimitUser.ID, lowPolicy.ID)
		require.NoError(t, err)

		// Record usage that exceeds limit
		err = store.RecordTokenUsage(ctx, lowLimitUser.ID, 150)
		require.NoError(t, err)

		allowed, windows, blocking, err := store.CheckRateLimitStatus(ctx, lowLimitUser.ID)
		require.NoError(t, err)

		assert.False(t, allowed)
		assert.NotEmpty(t, windows)
		assert.NotNil(t, blocking)
		assert.Equal(t, WindowType1h, blocking.Window)
		assert.LessOrEqual(t, blocking.Remaining, int64(0))
	})

	t.Run("GetUserTokenUsageHistory_ReturnsRecords", func(t *testing.T) {
		now := time.Now().UTC()
		start := now.Add(-24 * time.Hour)
		end := now.Add(time.Hour)

		history, err := store.GetUserTokenUsageHistory(ctx, user.ID, WindowType1h, start, end)
		require.NoError(t, err)

		// Should have at least one record from our previous tests
		assert.NotEmpty(t, history)
		for _, h := range history {
			assert.Equal(t, WindowType1h, h.WindowType)
			assert.Equal(t, user.ID, h.UserID)
		}
	})

	t.Run("CleanupExpiredWindows_RemovesOldRecords", func(t *testing.T) {
		// This is hard to test without manipulating timestamps
		// Just verify the function runs without error
		count, err := store.CleanupExpiredWindows(ctx)
		require.NoError(t, err)
		// Count should be >= 0 (may not delete anything in new DB)
		assert.GreaterOrEqual(t, count, int64(0))
	})
}

func TestTokenUsageRepository_WindowCalculations(t *testing.T) {
	store, cleanup := setupStore(t)
	if store == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()

	org, err := store.CreateOrg(ctx, CreateOrgParams{
		Slug:   "window-calc-org",
		Name:   "Window Calc Org",
		Status: "active",
	})
	require.NoError(t, err)

	user, err := store.CreateUser(ctx, CreateUserParams{
		OrgID:        org.ID,
		Email:        "window@example.com",
		DisplayName:  "Window User",
		PasswordHash: "test-hash",
		Status:       "active",
	})
	require.NoError(t, err)

	// Create policy with all limits
	limit1h := int64(1000)
	limit24h := int64(10000)
	limit7d := int64(50000)
	policy, err := store.CreateTokenPolicy(ctx, CreateTokenPolicyParams{
		OrgID:    org.ID,
		Name:     "all-limits",
		Limit1h:  &limit1h,
		Limit24h: &limit24h,
		Limit7d:  &limit7d,
	})
	require.NoError(t, err)

	err = store.SetOrgDefaultTokenPolicy(ctx, org.ID, policy.ID)
	require.NoError(t, err)

	t.Run("ResetsAt_1h_IsNextHour", func(t *testing.T) {
		err := store.RecordTokenUsage(ctx, user.ID, 100)
		require.NoError(t, err)

		status, err := store.GetTokenUsageForPolicy(ctx, user.ID, policy)
		require.NoError(t, err)

		for _, s := range status {
			if s.Window == WindowType1h {
				// ResetsAt should be at the start of next hour
				now := time.Now().UTC()
				expectedReset := now.Truncate(time.Hour).Add(time.Hour)

				// Allow some tolerance for test execution time
				assert.WithinDuration(t, expectedReset, s.ResetsAt, time.Minute)
				break
			}
		}
	})

	t.Run("Percentage_CalculatedCorrectly", func(t *testing.T) {
		// Record known amount to check percentage
		err := store.RecordTokenUsage(ctx, user.ID, 400) // total ~500 now
		require.NoError(t, err)

		status, err := store.GetTokenUsageForPolicy(ctx, user.ID, policy)
		require.NoError(t, err)

		for _, s := range status {
			if s.Window == WindowType1h {
				// Used should be >= 500, limit is 1000
				// Percentage should be >= 50%
				assert.GreaterOrEqual(t, s.Percentage, float64(50))
				break
			}
		}
	})
}
