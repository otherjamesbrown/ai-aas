package tests

import "testing"

func TestHourlyRollupMatchesUsage(t *testing.T) {
	rollupTestConfig{
		table:            "analytics_hourly_rollups",
		intervalUnit:     "hours",
		defaultWindow:    24,
		windowEnvVar:     "ROLLUP_TEST_WINDOW_HOURS",
		truncExpression:  "date_trunc('hour', occurred_at)",
		resultIdentifier: "hourly rollup mismatch detected",
	}.Run(t)
}
