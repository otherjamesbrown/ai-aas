//go:build full && e2e_tier || !e2e_tier

package suites

// DEPRECATED: Integration tests in this file have been migrated to UC structure.
// See tests/usecases/analytics_flow_test.go for:
//   - TestUC_ANL_001_UsageRecording (usage recording validation)
//   - TestUC_ANL_003_AnalyticsExport (replaces all export tests)
//   - TestUC_ANL_004_CrossServiceCorrelation (trace ID correlation)
//
// These E2E tests remain for backwards compatibility but should not be extended.
// All new integration tests should be added to tests/usecases/ following UC specs.

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/ai-aas/tests/e2e/fixtures"
	"github.com/ai-aas/tests/e2e/harness"
)

// ExportJobResponse represents the response from export job endpoints
type ExportJobResponse struct {
	JobID       string            `json:"jobId"`
	OrgID       string            `json:"orgId"`
	Status      string            `json:"status"`
	Granularity string            `json:"granularity"`
	TimeRange   TimeRangeResponse `json:"timeRange"`
	CreatedAt   string            `json:"createdAt"`
	CompletedAt *string           `json:"completedAt,omitempty"`
	OutputURI   *string           `json:"outputUri,omitempty"`
	Checksum    *string           `json:"checksum,omitempty"`
	RowCount    *int64            `json:"rowCount,omitempty"`
	InitiatedBy string            `json:"initiatedBy"`
	Error       *string           `json:"error,omitempty"`
}

type TimeRangeResponse struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type ListExportJobsResponse struct {
	Items []ExportJobResponse `json:"items"`
}

// TestAnalyticsExportWorkflow tests the complete export workflow:
// 1. Create organization and API key
// 2. Generate usage data (make inference requests)
// 3. Create export job
// 4. Poll for completion
// 5. Download and verify export data
func TestAnalyticsExportWorkflow(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	if ctx.Config.APIURLs.AnalyticsService == "" {
		t.Skip("Analytics service not configured")
	}

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	saFixture := fixtures.NewServiceAccountFixture(ctx.Client, ctx.Fixtures)
	apiKeyFixture := fixtures.NewAPIKeyFixture(ctx.Client, ctx.Fixtures)

	// Step 1: Create organization
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}
	t.Logf("Created organization: %s", org.ID)

	// Step 2: Create service account and API key
	sa, err := saFixture.Create(ctx, org.ID, "")
	if err != nil {
		t.Fatalf("Failed to create service account: %v", err)
	}

	apiKey, err := apiKeyFixture.Create(ctx, org.ID, sa.ID, "", []string{"inference:read", "inference:write"})
	if err != nil {
		t.Fatalf("Failed to create API key: %v", err)
	}
	t.Logf("Created API key: %s", apiKey.ID)

	// Step 3: Generate usage data by making inference requests
	routerClient := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
	routerClient.SetHeader("Authorization", "Bearer "+apiKey.Key)

	if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
		routerClient.SetHeader("Host", "api.dev.otherjamesbrown.com")
	}

	// Get available models
	modelsResp, err := routerClient.GET("/v1/models")
	if err != nil {
		t.Fatalf("Failed to get models: %v", err)
	}

	var models ModelsResponse
	if err := json.Unmarshal(modelsResp.Body, &models); err != nil {
		t.Fatalf("Failed to parse models: %v", err)
	}

	if len(models.Data) == 0 {
		t.Skip("No models available - skipping test")
	}

	modelID := models.Data[0].ID
	t.Logf("Using model: %s", modelID)

	// Make a few inference requests to generate usage data
	testPrompt := getTestPrompt(modelID)
	reqBody := buildChatRequest(modelID, testPrompt, 20)

	t.Log("Generating usage data with inference requests...")
	for i := 0; i < 3; i++ {
		inferResp, err := routerClient.POST("/v1/chat/completions", reqBody)
		if err != nil {
			t.Logf("Warning: Inference request %d failed: %v", i+1, err)
			continue
		}
		if inferResp.StatusCode != 200 {
			t.Logf("Warning: Inference request %d returned status %d", i+1, inferResp.StatusCode)
			continue
		}
		t.Logf("Inference request %d completed successfully", i+1)
	}

	// Wait for usage data to be processed
	t.Log("Waiting for usage data to be processed...")
	time.Sleep(5 * time.Second)

	// Step 4: Create export job
	analyticsClient := harness.NewClient(ctx.Config.APIURLs.AnalyticsService, ctx.Config.Timeouts.RequestTimeout)

	// Use admin key for analytics
	adminKey := ctx.Config.Credentials.AdminAPIKey
	if adminKey != "" {
		analyticsClient.SetHeader("Authorization", "Bearer "+adminKey)
		analyticsClient.SetHeader("X-API-Key", adminKey)
	}

	if isIPAddress(ctx.Config.APIURLs.AnalyticsService) {
		analyticsClient.SetHeader("Host", "analytics.dev.otherjamesbrown.com")
	}

	now := time.Now().UTC()
	exportReq := map[string]interface{}{
		"timeRange": map[string]string{
			"start": now.Add(-1 * time.Hour).Format(time.RFC3339),
			"end":   now.Format(time.RFC3339),
		},
		"granularity": "hourly",
	}

	createPath := "/analytics/v1/orgs/" + org.ID + "/exports"
	createResp, err := analyticsClient.POST(createPath, exportReq)
	if err != nil {
		t.Fatalf("Failed to create export job: %v", err)
	}

	if createResp.StatusCode != 202 {
		t.Fatalf("Expected status 202 Accepted, got %d: %s", createResp.StatusCode, string(createResp.Body))
	}

	var exportJob ExportJobResponse
	if err := json.Unmarshal(createResp.Body, &exportJob); err != nil {
		t.Fatalf("Failed to parse export job response: %v", err)
	}

	t.Logf("Export job created: %s (status: %s)", exportJob.JobID, exportJob.Status)

	// Step 5: Poll for job completion
	jobID := exportJob.JobID
	getJobPath := "/analytics/v1/orgs/" + org.ID + "/exports/" + jobID

	t.Log("Polling for job completion...")
	maxAttempts := 30
	pollInterval := 2 * time.Second

	var finalJob ExportJobResponse
	jobCompleted := false

	for i := 0; i < maxAttempts; i++ {
		time.Sleep(pollInterval)

		getResp, err := analyticsClient.GET(getJobPath)
		if err != nil {
			t.Logf("Warning: Failed to get job status (attempt %d/%d): %v", i+1, maxAttempts, err)
			continue
		}

		if getResp.StatusCode != 200 {
			t.Logf("Warning: Get job returned status %d (attempt %d/%d)", getResp.StatusCode, i+1, maxAttempts)
			continue
		}

		if err := json.Unmarshal(getResp.Body, &finalJob); err != nil {
			t.Logf("Warning: Failed to parse job response (attempt %d/%d): %v", i+1, maxAttempts, err)
			continue
		}

		t.Logf("Job status (attempt %d/%d): %s", i+1, maxAttempts, finalJob.Status)

		// Check if job is in terminal state
		if finalJob.Status == "succeeded" {
			jobCompleted = true
			t.Log("Export job completed successfully")
			break
		} else if finalJob.Status == "failed" {
			errorMsg := "unknown error"
			if finalJob.Error != nil {
				errorMsg = *finalJob.Error
			}
			t.Fatalf("Export job failed: %s", errorMsg)
		} else if finalJob.Status == "expired" {
			t.Fatal("Export job expired")
		}
	}

	if !jobCompleted {
		t.Fatalf("Export job did not complete within %d seconds (final status: %s)", maxAttempts*2, finalJob.Status)
	}

	// Verify job has output URI
	if finalJob.OutputURI == nil {
		t.Fatal("Export job completed but has no output URI")
	}

	t.Logf("Export output URI: %s", *finalJob.OutputURI)

	if finalJob.RowCount != nil {
		t.Logf("Export row count: %d", *finalJob.RowCount)
	}

	if finalJob.Checksum != nil {
		t.Logf("Export checksum: %s", *finalJob.Checksum)
	}

	// Step 6: Get download URL (should redirect to signed URL)
	downloadPath := "/analytics/v1/orgs/" + org.ID + "/exports/" + jobID + "/download"
	downloadResp, err := analyticsClient.GET(downloadPath)
	if err != nil {
		t.Fatalf("Failed to get download URL: %v", err)
	}

	// Expect 302 Found redirect
	if downloadResp.StatusCode != 302 && downloadResp.StatusCode != 200 {
		t.Fatalf("Expected status 302 or 200 for download, got %d: %s", downloadResp.StatusCode, string(downloadResp.Body))
	}

	t.Log("Successfully retrieved download URL")

	// Note: We don't actually download the file from S3 in this test
	// The fact that we got a redirect/URL is sufficient to verify the workflow
}

// TestAnalyticsExportListJobs tests listing export jobs with filters
func TestAnalyticsExportListJobs(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	if ctx.Config.APIURLs.AnalyticsService == "" {
		t.Skip("Analytics service not configured")
	}

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)

	// Create organization
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}

	analyticsClient := harness.NewClient(ctx.Config.APIURLs.AnalyticsService, ctx.Config.Timeouts.RequestTimeout)

	adminKey := ctx.Config.Credentials.AdminAPIKey
	if adminKey != "" {
		analyticsClient.SetHeader("Authorization", "Bearer "+adminKey)
		analyticsClient.SetHeader("X-API-Key", adminKey)
	}

	if isIPAddress(ctx.Config.APIURLs.AnalyticsService) {
		analyticsClient.SetHeader("Host", "analytics.dev.otherjamesbrown.com")
	}

	// Create a couple of export jobs
	now := time.Now().UTC()
	exportReq := map[string]interface{}{
		"timeRange": map[string]string{
			"start": now.Add(-1 * time.Hour).Format(time.RFC3339),
			"end":   now.Format(time.RFC3339),
		},
		"granularity": "hourly",
	}

	createPath := "/analytics/v1/orgs/" + org.ID + "/exports"

	// Create first job
	createResp1, err := analyticsClient.POST(createPath, exportReq)
	if err != nil {
		t.Fatalf("Failed to create first export job: %v", err)
	}
	if createResp1.StatusCode != 202 {
		t.Fatalf("Expected status 202 for first job, got %d", createResp1.StatusCode)
	}

	var job1 ExportJobResponse
	json.Unmarshal(createResp1.Body, &job1)
	t.Logf("Created export job 1: %s", job1.JobID)

	// Create second job
	createResp2, err := analyticsClient.POST(createPath, exportReq)
	if err != nil {
		t.Fatalf("Failed to create second export job: %v", err)
	}
	if createResp2.StatusCode != 202 {
		t.Fatalf("Expected status 202 for second job, got %d", createResp2.StatusCode)
	}

	var job2 ExportJobResponse
	json.Unmarshal(createResp2.Body, &job2)
	t.Logf("Created export job 2: %s", job2.JobID)

	// List all export jobs
	listPath := "/analytics/v1/orgs/" + org.ID + "/exports"
	listResp, err := analyticsClient.GET(listPath)
	if err != nil {
		t.Fatalf("Failed to list export jobs: %v", err)
	}

	if listResp.StatusCode != 200 {
		t.Fatalf("Expected status 200 for list, got %d: %s", listResp.StatusCode, string(listResp.Body))
	}

	var listResult ListExportJobsResponse
	if err := json.Unmarshal(listResp.Body, &listResult); err != nil {
		t.Fatalf("Failed to parse list response: %v", err)
	}

	if len(listResult.Items) < 2 {
		t.Fatalf("Expected at least 2 jobs, got %d", len(listResult.Items))
	}

	t.Logf("Listed %d export jobs", len(listResult.Items))

	// Verify our jobs are in the list
	foundJob1 := false
	foundJob2 := false
	for _, job := range listResult.Items {
		if job.JobID == job1.JobID {
			foundJob1 = true
		}
		if job.JobID == job2.JobID {
			foundJob2 = true
		}
		t.Logf("  - Job %s: status=%s, created=%s", job.JobID, job.Status, job.CreatedAt)
	}

	if !foundJob1 || !foundJob2 {
		t.Error("Not all created jobs found in list")
	}

	// Test filtering by status
	listPendingPath := listPath + "?status=pending"
	listPendingResp, err := analyticsClient.GET(listPendingPath)
	if err == nil && listPendingResp.StatusCode == 200 {
		var pendingResult ListExportJobsResponse
		if err := json.Unmarshal(listPendingResp.Body, &pendingResult); err == nil {
			t.Logf("Filtered list (status=pending): %d jobs", len(pendingResult.Items))
			for _, job := range pendingResult.Items {
				if job.Status != "pending" && job.Status != "running" {
					t.Errorf("Job %s in pending filter has status %s", job.JobID, job.Status)
				}
			}
		}
	}
}

// TestAnalyticsExportValidation tests validation of export request parameters
func TestAnalyticsExportValidation(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	if ctx.Config.APIURLs.AnalyticsService == "" {
		t.Skip("Analytics service not configured")
	}

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)

	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}

	analyticsClient := harness.NewClient(ctx.Config.APIURLs.AnalyticsService, ctx.Config.Timeouts.RequestTimeout)

	adminKey := ctx.Config.Credentials.AdminAPIKey
	if adminKey != "" {
		analyticsClient.SetHeader("Authorization", "Bearer "+adminKey)
		analyticsClient.SetHeader("X-API-Key", adminKey)
	}

	if isIPAddress(ctx.Config.APIURLs.AnalyticsService) {
		analyticsClient.SetHeader("Host", "analytics.dev.otherjamesbrown.com")
	}

	createPath := "/analytics/v1/orgs/" + org.ID + "/exports"
	now := time.Now().UTC()

	testCases := []struct {
		name           string
		request        map[string]interface{}
		expectedStatus int
		expectedError  string
	}{
		{
			name: "missing time range",
			request: map[string]interface{}{
				"granularity": "hourly",
			},
			expectedStatus: 400,
			expectedError:  "timeRange",
		},
		{
			name: "end before start",
			request: map[string]interface{}{
				"timeRange": map[string]string{
					"start": now.Format(time.RFC3339),
					"end":   now.Add(-1 * time.Hour).Format(time.RFC3339),
				},
			},
			expectedStatus: 400,
			expectedError:  "must be after",
		},
		{
			name: "time range too large (> 31 days)",
			request: map[string]interface{}{
				"timeRange": map[string]string{
					"start": now.Add(-32 * 24 * time.Hour).Format(time.RFC3339),
					"end":   now.Format(time.RFC3339),
				},
			},
			expectedStatus: 400,
			expectedError:  "31 days",
		},
		{
			name: "invalid granularity",
			request: map[string]interface{}{
				"timeRange": map[string]string{
					"start": now.Add(-1 * time.Hour).Format(time.RFC3339),
					"end":   now.Format(time.RFC3339),
				},
				"granularity": "invalid",
			},
			expectedStatus: 400,
			expectedError:  "granularity",
		},
		{
			name: "valid request with default granularity",
			request: map[string]interface{}{
				"timeRange": map[string]string{
					"start": now.Add(-1 * time.Hour).Format(time.RFC3339),
					"end":   now.Format(time.RFC3339),
				},
			},
			expectedStatus: 202,
			expectedError:  "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := analyticsClient.POST(createPath, tc.request)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}

			if resp.StatusCode != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d: %s", tc.expectedStatus, resp.StatusCode, string(resp.Body))
			}

			if tc.expectedError != "" {
				bodyStr := string(resp.Body)
				if !strings.Contains(strings.ToLower(bodyStr), strings.ToLower(tc.expectedError)) {
					t.Errorf("Expected error message to contain '%s', got: %s", tc.expectedError, bodyStr)
				}
			}
		})
	}
}
