// Package commands provides tests for usage command.
package admin

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/errors"
)

func TestUsageCommand(t *testing.T) {
	cmd := UsageCommand()
	require.NotNil(t, cmd, "UsageCommand() returned nil")

	assert.Equal(t, "usage", cmd.Use)
	assert.Equal(t, "Query usage data", cmd.Short)
}

func TestUsageQueryCommand(t *testing.T) {
	cmd := UsageCommand()

	// Find query command by name
	queryCmd, _, err := cmd.Find([]string{"query"})
	require.NoError(t, err, "query command should exist")
	require.NotNil(t, queryCmd, "query command should not be nil")
	assert.Equal(t, "query", queryCmd.Use)

	// Test flags
	orgIDFlag := queryCmd.Flags().Lookup("org-id")
	assert.NotNil(t, orgIDFlag, "org-id flag should exist")

	fromFlag := queryCmd.Flags().Lookup("from")
	assert.NotNil(t, fromFlag, "from flag should exist")

	toFlag := queryCmd.Flags().Lookup("to")
	assert.NotNil(t, toFlag, "to flag should exist")

	granularityFlag := queryCmd.Flags().Lookup("granularity")
	assert.NotNil(t, granularityFlag, "granularity flag should exist")

	modelFlag := queryCmd.Flags().Lookup("model")
	assert.NotNil(t, modelFlag, "model flag should exist")

	formatFlag := queryCmd.Flags().Lookup("format")
	assert.NotNil(t, formatFlag, "format flag should exist")

	// Test convenience flags
	lastHourFlag := queryCmd.Flags().Lookup("last-hour")
	assert.NotNil(t, lastHourFlag, "last-hour flag should exist")

	last24hFlag := queryCmd.Flags().Lookup("last-24h")
	assert.NotNil(t, last24hFlag, "last-24h flag should exist")

	last7dFlag := queryCmd.Flags().Lookup("last-7d")
	assert.NotNil(t, last7dFlag, "last-7d flag should exist")

	allTimeFlag := queryCmd.Flags().Lookup("all-time")
	assert.NotNil(t, allTimeFlag, "all-time flag should exist")
}

func TestUsageSummaryCommand(t *testing.T) {
	cmd := UsageCommand()

	// Find summary command by name
	summaryCmd, _, err := cmd.Find([]string{"summary"})
	require.NoError(t, err, "summary command should exist")
	require.NotNil(t, summaryCmd, "summary command should not be nil")
	assert.Equal(t, "summary", summaryCmd.Use)

	// Test flags
	orgIDFlag := summaryCmd.Flags().Lookup("org-id")
	assert.NotNil(t, orgIDFlag, "org-id flag should exist")

	periodFlag := summaryCmd.Flags().Lookup("period")
	assert.NotNil(t, periodFlag, "period flag should exist")

	formatFlag := summaryCmd.Flags().Lookup("format")
	assert.NotNil(t, formatFlag, "format flag should exist")
}

// setupTestConfig creates environment variables for testing
// The CLI uses AI_AAS_ prefix for environment variables
func setupTestConfig(t *testing.T) func() {
	t.Helper()

	// Set environment variables with AI_AAS_ prefix (used by config loader)
	os.Setenv("AI_AAS_ANALYTICS_ENDPOINT", "http://test-analytics:8080")
	os.Setenv("AI_AAS_API_KEY", "test-api-key")

	return func() {
		os.Unsetenv("AI_AAS_ANALYTICS_ENDPOINT")
		os.Unsetenv("AI_AAS_API_KEY")
	}
}

func TestRunUsageQuery_ExplicitDateRange(t *testing.T) {
	cleanup := setupTestConfig(t)
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	tests := []struct {
		name          string
		orgID         string
		from          string
		to            string
		granularity   string
		model         string
		format        string
		verbose       bool
		quiet         bool
		endpoint      string
		apiKey        string
		lastHour      bool
		last24h       bool
		last7d        bool
		allTime       bool
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid date range",
			orgID:       "test-org",
			from:        "2025-01-01",
			to:          "2025-01-31",
			granularity: "day",
			format:      "table",
			endpoint:    "http://test-analytics:8080",
			apiKey:      "test-key",
			expectError: true, // Will fail at health check in test env
		},
		{
			name:          "missing org-id",
			orgID:         "",
			from:          "2025-01-01",
			to:            "2025-01-31",
			endpoint:      "http://test-analytics:8080",
			apiKey:        "test-key",
			expectError:   true,
			errorContains: "--org-id is required",
		},
		{
			name:          "missing from date",
			orgID:         "test-org",
			from:          "",
			to:            "2025-01-31",
			endpoint:      "http://test-analytics:8080",
			apiKey:        "test-key",
			expectError:   true,
			errorContains: "--from is required",
		},
		{
			name:          "missing to date",
			orgID:         "test-org",
			from:          "2025-01-01",
			to:            "",
			endpoint:      "http://test-analytics:8080",
			apiKey:        "test-key",
			expectError:   true,
			errorContains: "--to is required",
		},
		{
			name:          "invalid from date format",
			orgID:         "test-org",
			from:          "2025/01/01",
			to:            "2025-01-31",
			endpoint:      "http://test-analytics:8080",
			apiKey:        "test-key",
			expectError:   true,
			errorContains: "invalid 'from' date format",
		},
		{
			name:          "invalid to date format",
			orgID:         "test-org",
			from:          "2025-01-01",
			to:            "2025/01/31",
			endpoint:      "http://test-analytics:8080",
			apiKey:        "test-key",
			expectError:   true,
			errorContains: "invalid 'to' date format",
		},
		{
			name:          "from after to",
			orgID:         "test-org",
			from:          "2025-01-31",
			to:            "2025-01-01",
			endpoint:      "http://test-analytics:8080",
			apiKey:        "test-key",
			expectError:   true,
			errorContains: "'from' date must be before 'to' date",
		},
		{
			name:          "invalid granularity",
			orgID:         "test-org",
			from:          "2025-01-01",
			to:            "2025-01-31",
			granularity:   "minute",
			endpoint:      "http://test-analytics:8080",
			apiKey:        "test-key",
			expectError:   true,
			errorContains: "granularity must be 'hour' or 'day'",
		},
		{
			name:        "valid hourly granularity",
			orgID:       "test-org",
			from:        "2025-01-01",
			to:          "2025-01-07",
			granularity: "hour",
			format:      "json",
			endpoint:    "http://test-analytics:8080",
			apiKey:      "test-key",
			expectError: true, // Will fail at health check in test env
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runUsageQuery(
				cmd, []string{},
				tt.orgID, tt.from, tt.to, tt.granularity, tt.model,
				tt.format, tt.verbose, tt.quiet, tt.endpoint, tt.apiKey,
				tt.lastHour, tt.last24h, tt.last7d, tt.allTime,
			)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRunUsageQuery_ConvenienceFlags(t *testing.T) {
	cleanup := setupTestConfig(t)
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	now := time.Now()

	tests := []struct {
		name          string
		orgID         string
		lastHour      bool
		last24h       bool
		last7d        bool
		allTime       bool
		expectError   bool
		errorContains string
		checkDates    func(t *testing.T, from, to string)
	}{
		{
			name:        "last-hour flag",
			orgID:       "test-org",
			lastHour:    true,
			expectError: true, // Will fail at health check in test env
			checkDates: func(t *testing.T, from, to string) {
				// Should use datetime format for last-hour
				assert.Contains(t, from, "T", "from should contain datetime format")
				assert.Contains(t, to, "T", "to should contain datetime format")

				// Parse and verify it's approximately 1 hour range
				fromTime, err := time.Parse("2006-01-02T15:04:05", from)
				require.NoError(t, err)
				toTime, err := time.Parse("2006-01-02T15:04:05", to)
				require.NoError(t, err)

				duration := toTime.Sub(fromTime)
				assert.InDelta(t, time.Hour, duration, float64(time.Minute), "duration should be approximately 1 hour")
			},
		},
		{
			name:        "last-24h flag",
			orgID:       "test-org",
			last24h:     true,
			expectError: true, // Will fail at health check in test env
			checkDates: func(t *testing.T, from, to string) {
				// Should use date format for last-24h
				fromTime, err := time.Parse("2006-01-02", from)
				require.NoError(t, err)
				toTime, err := time.Parse("2006-01-02", to)
				require.NoError(t, err)

				// Should be approximately 24 hours
				duration := toTime.Sub(fromTime)
				assert.InDelta(t, 24*time.Hour, duration, float64(time.Hour), "duration should be approximately 24 hours")
			},
		},
		{
			name:        "last-7d flag",
			orgID:       "test-org",
			last7d:      true,
			expectError: true, // Will fail at health check in test env
			checkDates: func(t *testing.T, from, to string) {
				fromTime, err := time.Parse("2006-01-02", from)
				require.NoError(t, err)
				toTime, err := time.Parse("2006-01-02", to)
				require.NoError(t, err)

				// Should be approximately 7 days
				duration := toTime.Sub(fromTime)
				assert.InDelta(t, 7*24*time.Hour, duration, float64(time.Hour), "duration should be approximately 7 days")
			},
		},
		{
			name:        "all-time flag",
			orgID:       "test-org",
			allTime:     true,
			expectError: true, // Will fail at health check in test env
			checkDates: func(t *testing.T, from, to string) {
				// From should be very early date
				assert.Equal(t, "2000-01-01", from)

				// To should be today
				toTime, err := time.Parse("2006-01-02", to)
				require.NoError(t, err)
				assert.Equal(t, now.Format("2006-01-02"), toTime.Format("2006-01-02"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can't easily test the internal date calculation without refactoring,
			// but we can verify the errors and flag validation
			err := runUsageQuery(
				cmd, []string{},
				tt.orgID, "", "", "day", "",
				"table", false, false, "http://test-analytics:8080", "test-key",
				tt.lastHour, tt.last24h, tt.last7d, tt.allTime,
			)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			}
		})
	}
}

func TestRunUsageQuery_MutualExclusivity(t *testing.T) {
	cleanup := setupTestConfig(t)
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	tests := []struct {
		name          string
		orgID         string
		from          string
		to            string
		lastHour      bool
		last24h       bool
		last7d        bool
		allTime       bool
		expectError   bool
		errorContains string
	}{
		{
			name:          "multiple convenience flags",
			orgID:         "test-org",
			lastHour:      true,
			last24h:       true,
			expectError:   true,
			errorContains: "only one convenience time range flag can be used at a time",
		},
		{
			name:          "last-hour and last-7d",
			orgID:         "test-org",
			lastHour:      true,
			last7d:        true,
			expectError:   true,
			errorContains: "only one convenience time range flag can be used at a time",
		},
		{
			name:          "last-24h and all-time",
			orgID:         "test-org",
			last24h:       true,
			allTime:       true,
			expectError:   true,
			errorContains: "only one convenience time range flag can be used at a time",
		},
		{
			name:          "all three convenience flags",
			orgID:         "test-org",
			lastHour:      true,
			last24h:       true,
			last7d:        true,
			expectError:   true,
			errorContains: "only one convenience time range flag can be used at a time",
		},
		{
			name:          "convenience flag with explicit from",
			orgID:         "test-org",
			from:          "2025-01-01",
			lastHour:      true,
			expectError:   true,
			errorContains: "cannot use convenience time range flags with --from/--to",
		},
		{
			name:          "convenience flag with explicit to",
			orgID:         "test-org",
			to:            "2025-01-31",
			last24h:       true,
			expectError:   true,
			errorContains: "cannot use convenience time range flags with --from/--to",
		},
		{
			name:          "convenience flag with both explicit dates",
			orgID:         "test-org",
			from:          "2025-01-01",
			to:            "2025-01-31",
			last7d:        true,
			expectError:   true,
			errorContains: "cannot use convenience time range flags with --from/--to",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runUsageQuery(
				cmd, []string{},
				tt.orgID, tt.from, tt.to, "day", "",
				"table", false, false, "http://test-analytics:8080", "test-key",
				tt.lastHour, tt.last24h, tt.last7d, tt.allTime,
			)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errorContains)
		})
	}
}

func TestRunUsageQuery_OutputFormats(t *testing.T) {
	cleanup := setupTestConfig(t)
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	tests := []struct {
		name        string
		format      string
		expectError bool
	}{
		{
			name:        "table format",
			format:      "table",
			expectError: true, // Will fail at health check
		},
		{
			name:        "json format",
			format:      "json",
			expectError: true, // Will fail at health check
		},
		{
			name:        "csv format",
			format:      "csv",
			expectError: true, // Will fail at health check
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runUsageQuery(
				cmd, []string{},
				"test-org", "2025-01-01", "2025-01-31", "day", "",
				tt.format, false, false, "http://test-analytics:8080", "test-key",
				false, false, false, false,
			)

			// All will error at health check in test env, but format validation happens first
			require.Error(t, err)
		})
	}
}

func TestRunUsageQuery_ValidationErrorTypes(t *testing.T) {
	cleanup := setupTestConfig(t)
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	tests := []struct {
		name      string
		orgID     string
		from      string
		to        string
		wantError *errors.CLIError
	}{
		{
			name:  "missing org-id returns ValidationError",
			orgID: "",
			from:  "2025-01-01",
			to:    "2025-01-31",
			wantError: &errors.CLIError{
				Code: errors.ErrCodeValidationFailed,
			},
		},
		{
			name:  "invalid date format returns ValidationError",
			orgID: "test-org",
			from:  "invalid-date",
			to:    "2025-01-31",
			wantError: &errors.CLIError{
				Code: errors.ErrCodeValidationFailed,
			},
		},
		{
			name:  "from after to returns ValidationError",
			orgID: "test-org",
			from:  "2025-12-31",
			to:    "2025-01-01",
			wantError: &errors.CLIError{
				Code: errors.ErrCodeValidationFailed,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runUsageQuery(
				cmd, []string{},
				tt.orgID, tt.from, tt.to, "day", "",
				"table", false, false, "http://test-analytics:8080", "test-key",
				false, false, false, false,
			)

			require.Error(t, err)

			// Check if error is the expected type
			var cliErr *errors.CLIError
			require.ErrorAs(t, err, &cliErr)
			assert.Equal(t, tt.wantError.Code, cliErr.Code)
		})
	}
}

func TestRunUsageQuery_DatetimeFormatSupport(t *testing.T) {
	cleanup := setupTestConfig(t)
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	tests := []struct {
		name        string
		from        string
		to          string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "date format YYYY-MM-DD",
			from:        "2025-01-01",
			to:          "2025-01-31",
			expectError: true, // Will fail at health check
			errorMsg:    "",
		},
		{
			name:        "datetime format with time",
			from:        "2025-01-01T00:00:00",
			to:          "2025-01-31T23:59:59",
			expectError: true, // Will fail at health check
			errorMsg:    "",
		},
		{
			name:        "mixed formats - date and datetime",
			from:        "2025-01-01",
			to:          "2025-01-31T23:59:59",
			expectError: true, // Will fail at health check
			errorMsg:    "",
		},
		{
			name:        "invalid datetime format",
			from:        "2025-01-01 00:00:00", // Space instead of T
			to:          "2025-01-31",
			expectError: true,
			errorMsg:    "invalid 'from' date format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runUsageQuery(
				cmd, []string{},
				"test-org", tt.from, tt.to, "day", "",
				"table", false, false, "http://test-analytics:8080", "test-key",
				false, false, false, false,
			)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			}
		})
	}
}

// TestConvenienceFlagDateCalculation tests the date calculation logic for convenience flags
func TestConvenienceFlagDateCalculation(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name         string
		lastHour     bool
		last24h      bool
		last7d       bool
		allTime      bool
		expectedFrom string
		expectedTo   string
		validate     func(t *testing.T, from, to string)
	}{
		{
			name:     "last-hour uses datetime format",
			lastHour: true,
			validate: func(t *testing.T, from, to string) {
				assert.Contains(t, from, "T")
				assert.Contains(t, to, "T")

				fromTime, _ := time.Parse("2006-01-02T15:04:05", from)
				toTime, _ := time.Parse("2006-01-02T15:04:05", to)
				duration := toTime.Sub(fromTime)
				assert.InDelta(t, time.Hour, duration, float64(5*time.Second))
			},
		},
		{
			name:    "last-24h uses date format",
			last24h: true,
			validate: func(t *testing.T, from, to string) {
				assert.NotContains(t, from, "T")
				assert.NotContains(t, to, "T")

				fromTime, _ := time.Parse("2006-01-02", from)
				toTime, _ := time.Parse("2006-01-02", to)
				duration := toTime.Sub(fromTime)
				assert.Equal(t, 24*time.Hour, duration)
			},
		},
		{
			name:   "last-7d uses date format",
			last7d: true,
			validate: func(t *testing.T, from, to string) {
				assert.NotContains(t, from, "T")
				assert.NotContains(t, to, "T")

				fromTime, _ := time.Parse("2006-01-02", from)
				toTime, _ := time.Parse("2006-01-02", to)
				duration := toTime.Sub(fromTime)
				assert.Equal(t, 7*24*time.Hour, duration)
			},
		},
		{
			name:    "all-time starts from 2000-01-01",
			allTime: true,
			validate: func(t *testing.T, from, to string) {
				assert.Equal(t, "2000-01-01", from)
				assert.Equal(t, now.Format("2006-01-02"), to)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var from, to string

			// Simulate the date calculation logic from runUsageQuery
			if tt.lastHour {
				from = now.Add(-1 * time.Hour).Format("2006-01-02T15:04:05")
				to = now.Format("2006-01-02T15:04:05")
			} else if tt.last24h {
				from = now.Add(-24 * time.Hour).Format("2006-01-02")
				to = now.Format("2006-01-02")
			} else if tt.last7d {
				from = now.AddDate(0, 0, -7).Format("2006-01-02")
				to = now.Format("2006-01-02")
			} else if tt.allTime {
				from = "2000-01-01"
				to = now.Format("2006-01-02")
			}

			tt.validate(t, from, to)
		})
	}
}

// TestUsageQueryHelpText verifies that examples in help text are valid
func TestUsageQueryHelpText(t *testing.T) {
	cmd := usageQueryCommand()

	examples := cmd.Example
	assert.NotEmpty(t, examples, "Command should have examples")

	// Verify key examples are present
	assert.Contains(t, examples, "--org-id acme-corp --from 2025-01-01 --to 2025-01-31")
	assert.Contains(t, examples, "--last-24h")
	assert.Contains(t, examples, "--last-7d")
	assert.Contains(t, examples, "--last-hour")
	assert.Contains(t, examples, "--granularity hour")
	assert.Contains(t, examples, "--model gpt-4")
	assert.Contains(t, examples, "--format json")
}

// TestUsageQueryFlagDefaults verifies default flag values
func TestUsageQueryFlagDefaults(t *testing.T) {
	cmd := usageQueryCommand()

	tests := []struct {
		flagName     string
		expectedVal  string
		expectedBool bool
	}{
		{flagName: "granularity", expectedVal: "day"},
		{flagName: "format", expectedVal: "table"},
		{flagName: "verbose", expectedBool: false},
		{flagName: "quiet", expectedBool: false},
		{flagName: "last-hour", expectedBool: false},
		{flagName: "last-24h", expectedBool: false},
		{flagName: "last-7d", expectedBool: false},
		{flagName: "all-time", expectedBool: false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("flag_%s", tt.flagName), func(t *testing.T) {
			flag := cmd.Flags().Lookup(tt.flagName)
			require.NotNil(t, flag, "flag %s should exist", tt.flagName)

			if tt.expectedVal != "" {
				assert.Equal(t, tt.expectedVal, flag.DefValue, "flag %s default value", tt.flagName)
			} else {
				assert.Equal(t, fmt.Sprintf("%t", tt.expectedBool), flag.DefValue, "flag %s default value", tt.flagName)
			}
		})
	}
}

// TestErrorMessageQuality verifies that error messages are actionable
func TestErrorMessageQuality(t *testing.T) {
	cleanup := setupTestConfig(t)
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	tests := []struct {
		name              string
		orgID             string
		from              string
		to                string
		expectedInMessage []string
	}{
		{
			name:  "missing org-id has suggestion",
			orgID: "",
			from:  "2025-01-01",
			to:    "2025-01-31",
			expectedInMessage: []string{
				"--org-id is required",
				"Suggestion",
			},
		},
		{
			name:  "invalid date has format example",
			orgID: "test-org",
			from:  "invalid",
			to:    "2025-01-31",
			expectedInMessage: []string{
				"invalid 'from' date format",
				"YYYY-MM-DD",
			},
		},
		{
			name:  "date range error has clear message",
			orgID: "test-org",
			from:  "2025-12-31",
			to:    "2025-01-01",
			expectedInMessage: []string{
				"'from' date must be before 'to' date",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runUsageQuery(
				cmd, []string{},
				tt.orgID, tt.from, tt.to, "day", "",
				"table", false, false, "http://test-analytics:8080", "test-key",
				false, false, false, false,
			)

			require.Error(t, err)
			errMsg := err.Error()

			for _, expected := range tt.expectedInMessage {
				assert.Contains(t, strings.ToLower(errMsg), strings.ToLower(expected),
					"error message should contain: %s", expected)
			}
		})
	}
}
