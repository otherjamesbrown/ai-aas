package tests

import "testing"

func TestDailyRollupMatchesUsage(t *testing.T) {
	rollupTestConfig{
		table:            "analytics_daily_rollups",
		intervalUnit:     "days",
		defaultWindow:    30,
		windowEnvVar:     "ROLLUP_TEST_WINDOW_DAYS",
		truncExpression:  "date_trunc('day', occurred_at)::date",
		resultIdentifier: "daily rollup mismatch detected",
	}.Run(t)
}
