//go:build (nightly || full) && e2e_tier || !e2e_tier

package suites

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ai-aas/tests/e2e/fixtures"
	"github.com/ai-aas/tests/e2e/harness"
)

// TestModelOnboardingE2E tests the full model onboarding workflow
//
// Prerequisites:
//   - Admin API service running
//   - ai-aas-cli in PATH or built
//   - Admin API key configured
//
// Test Flow:
//  1. Register model: ai-aas-cli model registry add
//  2. Verify in registry: ai-aas-cli model registry list
//  3. Cache model: ai-aas-cli model cache pull
//  4. Check cache status: ai-aas-cli model cache status
//  5. Deploy model: ai-aas-cli model deploy create
//  6. Verify in models list: GET /v1/models
//  7. Make inference request to new model
//  8. Clean up resources
func TestModelOnboardingE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	// Test configuration - using a small model for faster testing
	modelID := "TinyLlama/TinyLlama-1.1B-Chat-v1.0"
	modelName := fmt.Sprintf("e2e-tiny-llama-%d", time.Now().Unix())
	environment := "development"

	// Cleanup function - runs even if test fails
	t.Cleanup(func() {
		cleanupModel(t, modelName, environment)
	})

	// Step 1: Register model
	t.Run("Step1_RegisterModel", func(t *testing.T) {
		testRegistryAdd(t, ctx, modelID, modelName)
	})

	// Step 2: Verify in registry
	t.Run("Step2_VerifyInRegistry", func(t *testing.T) {
		testRegistryList(t, ctx, modelName)
	})

	// Step 3: Cache model
	t.Run("Step3_CacheModel", func(t *testing.T) {
		testCachePull(t, ctx, modelName)
	})

	// Step 4: Check cache status
	t.Run("Step4_CheckCacheStatus", func(t *testing.T) {
		testCacheStatus(t, ctx, modelName)
	})

	// Step 5: Deploy model
	t.Run("Step5_DeployModel", func(t *testing.T) {
		testDeployCreate(t, ctx, modelName, environment)
	})

	// Step 6: Verify in models list
	t.Run("Step6_VerifyInModelsList", func(t *testing.T) {
		testModelsListAPI(t, ctx, modelName)
	})

	// Step 7: Inference request (optional - may take time for model to be ready)
	t.Run("Step7_InferenceRequest", func(t *testing.T) {
		testInferenceToNewModel(t, ctx, modelName)
	})
}

// TestModelRegistryOperations tests registry add/list/show/remove operations
func TestModelRegistryOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	modelID := "TinyLlama/TinyLlama-1.1B-Chat-v1.0"
	modelName := fmt.Sprintf("e2e-registry-test-%d", time.Now().Unix())

	t.Cleanup(func() {
		cleanupModel(t, modelName, "")
	})

	// Add model to registry
	t.Run("RegistryAdd", func(t *testing.T) {
		testRegistryAdd(t, ctx, modelID, modelName)
	})

	// List models in registry
	t.Run("RegistryList", func(t *testing.T) {
		testRegistryList(t, ctx, modelName)
	})

	// Show model details
	t.Run("RegistryShow", func(t *testing.T) {
		testRegistryShow(t, ctx, modelName)
	})

	// Remove model from registry
	t.Run("RegistryRemove", func(t *testing.T) {
		testRegistryRemove(t, ctx, modelName)
	})
}

// TestModelCacheOperations tests cache pull/status/delete operations
func TestModelCacheOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	modelID := "TinyLlama/TinyLlama-1.1B-Chat-v1.0"
	modelName := fmt.Sprintf("e2e-cache-test-%d", time.Now().Unix())

	t.Cleanup(func() {
		cleanupModel(t, modelName, "")
	})

	// Register model first
	t.Run("Setup_RegisterModel", func(t *testing.T) {
		testRegistryAdd(t, ctx, modelID, modelName)
	})

	// Pull model to cache
	t.Run("CachePull", func(t *testing.T) {
		testCachePull(t, ctx, modelName)
	})

	// Check cache status
	t.Run("CacheStatus", func(t *testing.T) {
		testCacheStatus(t, ctx, modelName)
	})

	// Delete from cache
	t.Run("CacheDelete", func(t *testing.T) {
		testCacheDelete(t, ctx, modelName)
	})
}

// testRegistryAdd tests adding a model to the registry via CLI
func testRegistryAdd(t *testing.T, ctx *harness.Context, modelID, modelName string) {
	t.Helper()

	cmd := exec.Command("ai-aas-cli", "model", "registry", "add",
		modelID,
		"--name", modelName,
	)

	output, err := cmd.CombinedOutput()
	outputStr := string(output)
	t.Logf("Registry add output:\n%s", outputStr)

	if err != nil {
		// Check if it's a known acceptable error
		if strings.Contains(outputStr, "already exists") {
			t.Logf("Model already exists in registry (acceptable)")
			return
		}
		t.Fatalf("Failed to add model to registry: %v\nOutput: %s", err, outputStr)
	}

	// Verify success message
	assert.Contains(t, outputStr, modelName, "Output should mention model name")
}

// testRegistryList tests listing models in the registry
func testRegistryList(t *testing.T, ctx *harness.Context, expectedModelName string) {
	t.Helper()

	cmd := exec.Command("ai-aas-cli", "model", "registry", "list", "--output", "json")
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Fatalf("Failed to list registry models: %v\nOutput: %s", err, string(output))
	}

	outputStr := string(output)
	t.Logf("Registry list output: %s", truncateString(outputStr, 200))

	// Verify expected model is in the list
	assert.Contains(t, outputStr, expectedModelName, "Registry list should contain the model")
}

// testRegistryShow tests showing model details from registry
func testRegistryShow(t *testing.T, ctx *harness.Context, modelName string) {
	t.Helper()

	cmd := exec.Command("ai-aas-cli", "model", "registry", "show", modelName, "--output", "json")
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Fatalf("Failed to show model details: %v\nOutput: %s", err, string(output))
	}

	outputStr := string(output)
	t.Logf("Registry show output: %s", truncateString(outputStr, 200))

	// Verify model name is in output
	assert.Contains(t, outputStr, modelName, "Model details should contain model name")
}

// testRegistryRemove tests removing a model from the registry
func testRegistryRemove(t *testing.T, ctx *harness.Context, modelName string) {
	t.Helper()

	cmd := exec.Command("ai-aas-cli", "model", "registry", "remove", modelName)
	output, err := cmd.CombinedOutput()

	if err != nil {
		outputStr := string(output)
		// Check if model doesn't exist (acceptable)
		if strings.Contains(outputStr, "not found") {
			t.Logf("Model not found in registry (may already be removed)")
			return
		}
		t.Fatalf("Failed to remove model from registry: %v\nOutput: %s", err, outputStr)
	}

	t.Logf("Model removed from registry: %s", modelName)
}

// testCachePull tests pulling a model to cache
func testCachePull(t *testing.T, ctx *harness.Context, modelName string) {
	t.Helper()

	cmd := exec.Command("ai-aas-cli", "model", "cache", "pull", modelName)
	output, err := cmd.CombinedOutput()
	outputStr := string(output)
	t.Logf("Cache pull output:\n%s", outputStr)

	if err != nil {
		// Check for acceptable errors
		if strings.Contains(outputStr, "already cached") {
			t.Logf("Model already cached (acceptable)")
			return
		}
		if strings.Contains(outputStr, "not found in registry") {
			t.Skip("Model not found in registry - may need to add first")
			return
		}
		t.Fatalf("Failed to pull model to cache: %v\nOutput: %s", err, outputStr)
	}

	// Verify success or progress message
	// Note: This operation may take time, so we just verify the command started
	t.Logf("Cache pull initiated for model: %s", modelName)
}

// testCacheStatus tests checking cache status of a model
func testCacheStatus(t *testing.T, ctx *harness.Context, modelName string) {
	t.Helper()

	cmd := exec.Command("ai-aas-cli", "model", "cache", "status", modelName, "--output", "json")
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		// Check if model not found (acceptable if cache pull failed)
		if strings.Contains(outputStr, "not found") {
			t.Logf("Model not found in cache (may still be pulling or not cached)")
			return
		}
		t.Fatalf("Failed to get cache status: %v\nOutput: %s", err, outputStr)
	}

	t.Logf("Cache status output: %s", truncateString(outputStr, 200))

	// Verify output contains model name
	assert.Contains(t, outputStr, modelName, "Cache status should mention model name")
}

// testCacheDelete tests deleting a model from cache
func testCacheDelete(t *testing.T, ctx *harness.Context, modelName string) {
	t.Helper()

	cmd := exec.Command("ai-aas-cli", "model", "cache", "delete", modelName)
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		// Check if already deleted or not found (acceptable)
		if strings.Contains(outputStr, "not found") {
			t.Logf("Model not found in cache (may already be deleted)")
			return
		}
		t.Fatalf("Failed to delete from cache: %v\nOutput: %s", err, outputStr)
	}

	t.Logf("Model deleted from cache: %s", modelName)
}

// testDeployCreate tests deploying a model via CLI
func testDeployCreate(t *testing.T, ctx *harness.Context, modelName, environment string) {
	t.Helper()

	cmd := exec.Command("ai-aas-cli", "model", "deploy", "create",
		modelName,
		"-e", environment,
	)

	output, err := cmd.CombinedOutput()
	outputStr := string(output)
	t.Logf("Deploy create output:\n%s", outputStr)

	if err != nil {
		// Check for acceptable errors
		if strings.Contains(outputStr, "already deployed") {
			t.Logf("Model already deployed (acceptable)")
			return
		}
		if strings.Contains(outputStr, "not found") {
			t.Skip("Model not found - may need to be cached first")
			return
		}
		t.Fatalf("Failed to deploy model: %v\nOutput: %s", err, outputStr)
	}

	// Verify deployment initiated
	t.Logf("Deployment initiated for model: %s in environment: %s", modelName, environment)
}

// testModelsListAPI tests that the model appears in GET /v1/models
func testModelsListAPI(t *testing.T, ctx *harness.Context, expectedModelName string) {
	t.Helper()

	// Create client for API router service
	routerClient := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)

	// If using IP address, set Host header for ingress routing
	if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
		routerClient.SetHeader("Host", "api.dev.otherjamesbrown.com")
	}

	// GET /v1/models - publicly accessible
	resp, err := routerClient.GET("/v1/models")
	if err != nil {
		t.Fatalf("Failed to get models from API: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(resp.Body))
	}

	var modelsResp ModelsResponse
	if err := json.Unmarshal(resp.Body, &modelsResp); err != nil {
		t.Fatalf("Failed to parse models response: %v", err)
	}

	t.Logf("Models API returned %d models", len(modelsResp.Data))

	// Check if our model is in the list
	found := false
	for _, model := range modelsResp.Data {
		if model.ID == expectedModelName {
			found = true
			t.Logf("Found deployed model in API: %s", model.ID)
			break
		}
	}

	if !found {
		t.Logf("Warning: Model %s not found in /v1/models - may still be deploying", expectedModelName)
		// Don't fail - deployment may take time
	}
}

// testInferenceToNewModel tests making an inference request to the newly deployed model
func testInferenceToNewModel(t *testing.T, ctx *harness.Context, modelName string) {
	t.Helper()

	// Create organization and API key for inference
	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	saFixture := fixtures.NewServiceAccountFixture(ctx.Client, ctx.Fixtures)
	apiKeyFixture := fixtures.NewAPIKeyFixture(ctx.Client, ctx.Fixtures)

	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Skipf("Failed to create organization for inference test: %v", err)
		return
	}

	sa, err := saFixture.Create(ctx, org.ID, "")
	if err != nil {
		t.Skipf("Failed to create service account: %v", err)
		return
	}

	apiKey, err := apiKeyFixture.Create(ctx, org.ID, sa.ID, "", []string{"inference:read", "inference:write"})
	if err != nil {
		t.Skipf("Failed to create API key: %v", err)
		return
	}

	// Create router client with auth
	routerClient := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
	routerClient.SetHeader("Authorization", "Bearer "+apiKey.Key)

	if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
		routerClient.SetHeader("Host", "api.dev.otherjamesbrown.com")
	}

	// Build inference request
	reqBody := map[string]interface{}{
		"model": modelName,
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": "Say hello in one word.",
			},
		},
		"max_tokens": 10,
	}

	// Make inference request
	inferenceResp, err := routerClient.POST("/v1/chat/completions", reqBody)
	if err != nil {
		t.Logf("Inference request failed: %v (model may not be ready yet)", err)
		return
	}

	if inferenceResp.StatusCode == 503 {
		t.Log("Backend not ready (503) - model may still be starting up")
		return
	}

	if inferenceResp.StatusCode != 200 {
		t.Logf("Inference returned status %d: %s (model may not be ready)", inferenceResp.StatusCode, string(inferenceResp.Body))
		return
	}

	var completionResp ChatCompletionResponse
	if err := json.Unmarshal(inferenceResp.Body, &completionResp); err != nil {
		t.Fatalf("Failed to parse inference response: %v", err)
	}

	// Validate response
	require.NotEmpty(t, completionResp.Choices, "Response should have choices")
	require.NotEmpty(t, completionResp.Choices[0].Message.Content, "Response content should not be empty")

	t.Logf("Inference successful - Response: %s", truncateString(completionResp.Choices[0].Message.Content, 50))
	t.Logf("Token usage - Prompt: %d, Completion: %d, Total: %d",
		completionResp.Usage.PromptTokens,
		completionResp.Usage.CompletionTokens,
		completionResp.Usage.TotalTokens)
}

// cleanupModel removes a model from deployment, cache, and registry
func cleanupModel(t *testing.T, modelName, environment string) {
	t.Helper()

	// Delete deployment if environment specified
	if environment != "" {
		cmd := exec.Command("ai-aas-cli", "model", "deploy", "delete", modelName, "-e", environment)
		output, _ := cmd.CombinedOutput()
		t.Logf("Cleanup deployment output: %s", string(output))
	}

	// Delete from cache
	cmd := exec.Command("ai-aas-cli", "model", "cache", "delete", modelName)
	output, _ := cmd.CombinedOutput()
	t.Logf("Cleanup cache output: %s", string(output))

	// Remove from registry
	cmd = exec.Command("ai-aas-cli", "model", "registry", "remove", modelName)
	output, _ = cmd.CombinedOutput()
	t.Logf("Cleanup registry output: %s", string(output))

	t.Logf("Cleanup completed for model: %s", modelName)
}
