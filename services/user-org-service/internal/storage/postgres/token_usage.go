package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// RecordTokenUsage adds tokens to all applicable rolling windows for a user.
// This creates or updates window records for 1h, 24h, and 7d windows.
func (s *Store) RecordTokenUsage(ctx context.Context, userID uuid.UUID, tokens int64) error {
	now := time.Now().UTC()

	// Calculate window starts for each type
	windows := []struct {
		windowType  WindowType
		windowStart time.Time
	}{
		{WindowType1h, now.Truncate(time.Hour)},
		{WindowType24h, now.Truncate(24 * time.Hour)},
		{WindowType7d, now.Truncate(24 * time.Hour).AddDate(0, 0, -int(now.Truncate(24*time.Hour).Weekday()))}, // Start of week (Sunday)
	}

	// Use a transaction to update all windows atomically
	return s.withTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		for _, w := range windows {
			_, err := tx.Exec(ctx, `
				INSERT INTO token_usage_windows (user_id, window_type, window_start, tokens_used, updated_at)
				VALUES ($1, $2, $3, $4, now())
				ON CONFLICT (user_id, window_type, window_start)
				DO UPDATE SET tokens_used = token_usage_windows.tokens_used + $4, updated_at = now()
			`, userID, string(w.windowType), w.windowStart, tokens)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// GetTokenUsage returns the current token usage for a user across all windows.
func (s *Store) GetTokenUsage(ctx context.Context, userID uuid.UUID) ([]TokenUsageWindow, error) {
	now := time.Now().UTC()

	// Calculate current window starts
	hour := now.Truncate(time.Hour)
	day := now.Truncate(24 * time.Hour)
	week := day.AddDate(0, 0, -int(day.Weekday()))

	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, window_type, window_start, tokens_used, updated_at
		FROM token_usage_windows
		WHERE user_id = $1 AND (
			(window_type = '1h' AND window_start = $2) OR
			(window_type = '24h' AND window_start = $3) OR
			(window_type = '7d' AND window_start = $4)
		)
	`, userID, hour, day, week)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var windows []TokenUsageWindow
	for rows.Next() {
		var w TokenUsageWindow
		var windowType string
		if err := rows.Scan(&w.ID, &w.UserID, &windowType, &w.WindowStart, &w.TokensUsed, &w.UpdatedAt); err != nil {
			return nil, err
		}
		w.WindowType = WindowType(windowType)
		windows = append(windows, w)
	}

	return windows, rows.Err()
}

// GetTokenUsageForPolicy returns usage status for each window with limits from the policy.
// Only returns windows that have limits defined in the policy.
func (s *Store) GetTokenUsageForPolicy(ctx context.Context, userID uuid.UUID, policy TokenRateLimitPolicy) ([]TokenUsageStatus, error) {
	now := time.Now().UTC()

	// Get current usage
	usage, err := s.GetTokenUsage(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Create a map for easy lookup
	usageMap := make(map[WindowType]int64)
	for _, u := range usage {
		usageMap[u.WindowType] = u.TokensUsed
	}

	var status []TokenUsageStatus

	// 1h window
	if policy.Limit1h != nil {
		used := usageMap[WindowType1h]
		limit := *policy.Limit1h
		remaining := limit - used
		percentage := float64(used) / float64(limit) * 100
		resetsAt := now.Truncate(time.Hour).Add(time.Hour)
		status = append(status, TokenUsageStatus{
			Window:     WindowType1h,
			Limit:      limit,
			Used:       used,
			Remaining:  remaining,
			Percentage: percentage,
			ResetsAt:   resetsAt,
		})
	}

	// 24h window
	if policy.Limit24h != nil {
		used := usageMap[WindowType24h]
		limit := *policy.Limit24h
		remaining := limit - used
		percentage := float64(used) / float64(limit) * 100
		resetsAt := now.Truncate(24 * time.Hour).Add(24 * time.Hour)
		status = append(status, TokenUsageStatus{
			Window:     WindowType24h,
			Limit:      limit,
			Used:       used,
			Remaining:  remaining,
			Percentage: percentage,
			ResetsAt:   resetsAt,
		})
	}

	// 7d window
	if policy.Limit7d != nil {
		used := usageMap[WindowType7d]
		limit := *policy.Limit7d
		remaining := limit - used
		percentage := float64(used) / float64(limit) * 100
		day := now.Truncate(24 * time.Hour)
		weekStart := day.AddDate(0, 0, -int(day.Weekday()))
		resetsAt := weekStart.AddDate(0, 0, 7)
		status = append(status, TokenUsageStatus{
			Window:     WindowType7d,
			Limit:      limit,
			Used:       used,
			Remaining:  remaining,
			Percentage: percentage,
			ResetsAt:   resetsAt,
		})
	}

	return status, nil
}

// CheckRateLimitStatus checks if a user is within their rate limits.
// Returns whether the request is allowed and details about each window.
func (s *Store) CheckRateLimitStatus(ctx context.Context, userID uuid.UUID) (allowed bool, windows []TokenUsageStatus, blockingWindow *TokenUsageStatus, err error) {
	// Get effective policy
	effective, err := s.GetEffectiveTokenPolicyByUserID(ctx, userID)
	if err != nil {
		return false, nil, nil, err
	}

	// No limits = always allowed
	if effective.Policy.ID == NoTokenRateLimitPolicyID {
		return true, nil, nil, nil
	}

	// Get usage status
	windows, err = s.GetTokenUsageForPolicy(ctx, userID, effective.Policy)
	if err != nil {
		return false, nil, nil, err
	}

	// Check each window
	allowed = true
	var earliestReset *TokenUsageStatus
	for i := range windows {
		w := &windows[i]
		if w.Remaining <= 0 {
			allowed = false
			// Track the window that resets soonest
			if earliestReset == nil || w.ResetsAt.Before(earliestReset.ResetsAt) {
				earliestReset = w
			}
		}
	}

	return allowed, windows, earliestReset, nil
}

// CleanupExpiredWindows removes token usage window records older than the retention period.
// Windows older than 30 days are removed.
func (s *Store) CleanupExpiredWindows(ctx context.Context) (int64, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -30)

	result, err := s.pool.Exec(ctx, `
		DELETE FROM token_usage_windows
		WHERE window_start < $1
	`, cutoff)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected(), nil
}

// GetUserTokenUsageHistory returns historical usage data for a user within a time range.
// This is useful for analytics and reporting.
func (s *Store) GetUserTokenUsageHistory(ctx context.Context, userID uuid.UUID, windowType WindowType, start, end time.Time) ([]TokenUsageWindow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, window_type, window_start, tokens_used, updated_at
		FROM token_usage_windows
		WHERE user_id = $1 AND window_type = $2 AND window_start >= $3 AND window_start <= $4
		ORDER BY window_start DESC
	`, userID, string(windowType), start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var windows []TokenUsageWindow
	for rows.Next() {
		var w TokenUsageWindow
		var wt string
		if err := rows.Scan(&w.ID, &w.UserID, &wt, &w.WindowStart, &w.TokensUsed, &w.UpdatedAt); err != nil {
			return nil, err
		}
		w.WindowType = WindowType(wt)
		windows = append(windows, w)
	}

	return windows, rows.Err()
}
