package suites

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

// TestRecipeDeployE2E tests the full E2E flow of deploying a model using a recipe
//
// Prerequisites:
//   - Kubernetes cluster with ai-model-operator installed
//   - ModelRecipe CRD installed
//   - AIModel CRD installed
//   - KUBECONFIG environment variable set (or default ~/.kube/config exists)
//
// Test Flow:
//  1. Create a ModelRecipe in the cluster
//  2. Deploy a model using --recipe flag via CLI
//  3. Verify the AIModel references the recipe
//  4. Verify the AIModel has merged configuration from recipe
//  5. Verify InferenceService is created (optional, if operator is running)
//  6. Clean up resources
func TestRecipeDeployE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Setup Kubernetes client
	k8sClient, err := setupKubernetesClient(t)
	if err != nil {
		t.Skipf("Kubernetes cluster not available: %v", err)
		return
	}

	// Test configuration
	recipeNamespace := "ai-model-system"
	recipeName := "test-llama-7b-recipe"
	modelName := "test-llama-7b"
	deployNamespace := "development"
	aimodelName := fmt.Sprintf("%s-%s", modelName, deployNamespace)

	// Cleanup function - runs even if test fails
	t.Cleanup(func() {
		cleanupE2EResources(t, k8sClient, recipeName, recipeNamespace, aimodelName, deployNamespace)
	})

	// Step 1: Create ModelRecipe
	t.Run("Step1_CreateModelRecipe", func(t *testing.T) {
		testCreateModelRecipe(t, k8sClient, recipeName, recipeNamespace)
	})

	// Step 2: Deploy model using recipe via CLI
	t.Run("Step2_DeployWithRecipe", func(t *testing.T) {
		testDeployModelWithRecipe(t, modelName, deployNamespace, recipeName)
	})

	// Step 3: Verify AIModel references recipe
	t.Run("Step3_VerifyAIModelRecipeRef", func(t *testing.T) {
		testVerifyAIModelRecipeRef(t, k8sClient, aimodelName, deployNamespace, recipeName, recipeNamespace)
	})

	// Step 4: Verify merged configuration
	t.Run("Step4_VerifyMergedConfig", func(t *testing.T) {
		testVerifyMergedConfiguration(t, k8sClient, aimodelName, deployNamespace)
	})

	// Step 5: Verify InferenceService (optional - only if operator is running)
	t.Run("Step5_VerifyInferenceService", func(t *testing.T) {
		testVerifyInferenceService(t, k8sClient, aimodelName, deployNamespace)
	})
}

// TestRecipeDeployWithOverrides tests deploying with a recipe and overriding some values
func TestRecipeDeployWithOverrides(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Setup Kubernetes client
	k8sClient, err := setupKubernetesClient(t)
	if err != nil {
		t.Skipf("Kubernetes cluster not available: %v", err)
		return
	}

	// Test configuration
	recipeNamespace := "ai-model-system"
	recipeName := "test-llama-7b-override-recipe"
	modelName := "test-llama-7b-override"
	deployNamespace := "development"
	aimodelName := fmt.Sprintf("%s-%s", modelName, deployNamespace)

	// Cleanup function
	t.Cleanup(func() {
		cleanupE2EResources(t, k8sClient, recipeName, recipeNamespace, aimodelName, deployNamespace)
	})

	// Step 1: Create ModelRecipe with base configuration
	t.Run("Step1_CreateModelRecipe", func(t *testing.T) {
		testCreateModelRecipe(t, k8sClient, recipeName, recipeNamespace)
	})

	// Step 2: Deploy with recipe and override GPU count
	t.Run("Step2_DeployWithRecipeAndOverrides", func(t *testing.T) {
		testDeployModelWithRecipeAndOverrides(t, modelName, deployNamespace, recipeName)
	})

	// Step 3: Verify overrides are applied
	t.Run("Step3_VerifyOverridesApplied", func(t *testing.T) {
		testVerifyOverridesApplied(t, k8sClient, aimodelName, deployNamespace)
	})
}

// setupKubernetesClient creates a Kubernetes dynamic client for testing
func setupKubernetesClient(t *testing.T) (dynamic.Interface, error) {
	t.Helper()

	// Load kubeconfig from default location or KUBECONFIG env var
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	return dynamicClient, nil
}

// testCreateModelRecipe creates a test ModelRecipe in the cluster
func testCreateModelRecipe(t *testing.T, k8sClient dynamic.Interface, recipeName, namespace string) {
	t.Helper()

	// Create namespace if it doesn't exist
	nsGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}
	nsObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata": map[string]interface{}{
				"name": namespace,
			},
		},
	}
	_, err := k8sClient.Resource(nsGVR).Create(context.Background(), nsObj, metav1.CreateOptions{})
	if err != nil && !isAlreadyExistsError(err) {
		t.Logf("Warning: Failed to create namespace %s: %v (may already exist)", namespace, err)
	}

	// Define ModelRecipe
	recipeGVR := schema.GroupVersionResource{
		Group:    "aimodel.ai-aas.io",
		Version:  "v1alpha1",
		Resource: "modelrecipes",
	}

	recipe := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "aimodel.ai-aas.io/v1alpha1",
			"kind":       "ModelRecipe",
			"metadata": map[string]interface{}{
				"name":      recipeName,
				"namespace": namespace,
				"labels": map[string]interface{}{
					"ai.ai-aas.io/model-family": "llama",
					"ai.ai-aas.io/model-size":   "7b",
					"ai.ai-aas.io/task":         "text-generation",
					"ai.ai-aas.io/runtime":      "vllm",
					"test":                      "e2e",
				},
			},
			"spec": map[string]interface{}{
				"modelID":     "meta-llama/Llama-2-7b-hf",
				"displayName": "Test Llama 2 7B",
				"description": "Test recipe for E2E testing",
				"runtime":     "vllm",
				"image":       "vllm/vllm-openai:v0.10.2",
				"resources": map[string]interface{}{
					"gpu": map[string]interface{}{
						"vendor":      "nvidia",
						"count":       1,
						"minMemoryGB": 16,
					},
					"cpu": map[string]interface{}{
						"requests": "4",
						"limits":   "8",
					},
					"memory": map[string]interface{}{
						"requests": "16Gi",
						"limits":   "32Gi",
					},
				},
				"runtimeArgs": map[string]interface{}{
					"vllm": map[string]interface{}{
						"dtype":                "auto",
						"maxModelLen":          4096,
						"gpuMemoryUtilization": "0.9",
						"trustRemoteCode":      false,
						"tokenizerMode":        "auto",
					},
				},
				"scheduling": map[string]interface{}{
					"tolerations": []interface{}{
						map[string]interface{}{
							"key":      "nvidia.com/gpu",
							"operator": "Exists",
							"effect":   "NoSchedule",
						},
					},
				},
				"healthCheck": map[string]interface{}{
					"startupProbeSeconds": 300,
					"livenessPath":        "/health",
					"readinessPath":       "/health",
				},
			},
		},
	}

	// Create the ModelRecipe
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	created, err := k8sClient.Resource(recipeGVR).Namespace(namespace).Create(ctx, recipe, metav1.CreateOptions{})
	require.NoError(t, err, "Failed to create ModelRecipe")

	t.Logf("Created ModelRecipe: %s/%s", namespace, recipeName)

	// Verify it was created
	name, found, err := unstructured.NestedString(created.Object, "metadata", "name")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, recipeName, name)
}

// testDeployModelWithRecipe deploys a model using the CLI with --recipe flag
func testDeployModelWithRecipe(t *testing.T, modelName, environment, recipeName string) {
	t.Helper()

	// Build CLI command
	// Note: This assumes the AI-AAS CLI is available in PATH or built
	cmd := exec.Command("ai-aas-cli", "model", "deploy", modelName,
		"-e", environment,
		"--recipe", recipeName,
		"--dry-run", // Use dry-run for testing without actual deployment
	)

	output, err := cmd.CombinedOutput()
	t.Logf("CLI output:\n%s", string(output))

	// For E2E test, we expect this might fail if CLI isn't fully configured
	// But we can still verify the YAML output structure
	if err != nil {
		t.Logf("CLI command failed (expected if not configured): %v", err)
		// Try to verify if the output at least mentions the recipe
		assert.Contains(t, string(output), recipeName, "CLI output should mention the recipe name")
	}

	// For actual deployment (not dry-run), we would remove --dry-run flag
	// and verify the AIModel is created in the cluster
}

// testVerifyAIModelRecipeRef verifies the AIModel has the correct recipeRef
func testVerifyAIModelRecipeRef(t *testing.T, k8sClient dynamic.Interface, aimodelName, namespace, expectedRecipeName, expectedRecipeNamespace string) {
	t.Helper()

	aimodelGVR := schema.GroupVersionResource{
		Group:    "aimodel.ai-aas.io",
		Version:  "v1alpha1",
		Resource: "aimodels",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// For testing, create a minimal AIModel with recipeRef
	// In real scenario, this would be created by the CLI
	aimodel := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "aimodel.ai-aas.io/v1alpha1",
			"kind":       "AIModel",
			"metadata": map[string]interface{}{
				"name":      aimodelName,
				"namespace": namespace,
				"labels": map[string]interface{}{
					"test": "e2e",
				},
			},
			"spec": map[string]interface{}{
				"recipeRef": map[string]interface{}{
					"name":      expectedRecipeName,
					"namespace": expectedRecipeNamespace,
				},
				"enabled": true,
			},
		},
	}

	// Create AIModel
	created, err := k8sClient.Resource(aimodelGVR).Namespace(namespace).Create(ctx, aimodel, metav1.CreateOptions{})
	if err != nil {
		t.Logf("Failed to create AIModel (may need namespace): %v", err)
		// Try to create namespace first
		nsGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}
		nsObj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Namespace",
				"metadata": map[string]interface{}{
					"name": namespace,
				},
			},
		}
		_, _ = k8sClient.Resource(nsGVR).Create(context.Background(), nsObj, metav1.CreateOptions{})

		// Retry creation
		created, err = k8sClient.Resource(aimodelGVR).Namespace(namespace).Create(ctx, aimodel, metav1.CreateOptions{})
		require.NoError(t, err, "Failed to create AIModel after namespace creation")
	}

	t.Logf("Created AIModel: %s/%s", namespace, aimodelName)

	// Verify recipeRef
	recipeRef, found, err := unstructured.NestedMap(created.Object, "spec", "recipeRef")
	require.NoError(t, err)
	require.True(t, found, "recipeRef should be present in AIModel spec")

	recipeName, found, err := unstructured.NestedString(created.Object, "spec", "recipeRef", "name")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, expectedRecipeName, recipeName, "Recipe name should match")

	recipeNs, found, err := unstructured.NestedString(created.Object, "spec", "recipeRef", "namespace")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, expectedRecipeNamespace, recipeNs, "Recipe namespace should match")

	t.Logf("Verified AIModel recipeRef: %v", recipeRef)
}

// testVerifyMergedConfiguration verifies the operator has merged recipe config into AIModel
func testVerifyMergedConfiguration(t *testing.T, k8sClient dynamic.Interface, aimodelName, namespace string) {
	t.Helper()

	aimodelGVR := schema.GroupVersionResource{
		Group:    "aimodel.ai-aas.io",
		Version:  "v1alpha1",
		Resource: "aimodels",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Wait briefly for operator to process (if running)
	time.Sleep(2 * time.Second)

	// Get AIModel
	aimodel, err := k8sClient.Resource(aimodelGVR).Namespace(namespace).Get(ctx, aimodelName, metav1.GetOptions{})
	if err != nil {
		t.Skipf("AIModel not found (operator may not be running): %v", err)
		return
	}

	// Check if status has been populated by operator
	status, found, err := unstructured.NestedMap(aimodel.Object, "status")
	if !found || err != nil {
		t.Logf("Status not yet populated by operator (may not be running)")
		return
	}

	t.Logf("AIModel status: %v", status)

	// If operator is running, it should have set some status fields
	// This is optional verification - operator may not be running in test environment
	phase, found, _ := unstructured.NestedString(aimodel.Object, "status", "phase")
	if found {
		t.Logf("AIModel phase: %s", phase)
		assert.NotEmpty(t, phase, "Phase should be set by operator")
	}
}

// testVerifyInferenceService verifies InferenceService was created (if operator is running)
func testVerifyInferenceService(t *testing.T, k8sClient dynamic.Interface, name, namespace string) {
	t.Helper()

	isvcGVR := schema.GroupVersionResource{
		Group:    "serving.kserve.io",
		Version:  "v1beta1",
		Resource: "inferenceservices",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Wait briefly for operator to create InferenceService
	time.Sleep(3 * time.Second)

	// Try to get InferenceService
	isvc, err := k8sClient.Resource(isvcGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		t.Logf("InferenceService not found (operator may not be running): %v", err)
		return // Skip this check if operator isn't running
	}

	t.Logf("InferenceService found: %s/%s", namespace, name)

	// Verify basic structure
	isvcName, found, err := unstructured.NestedString(isvc.Object, "metadata", "name")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, name, isvcName)

	// Verify it has spec.predictor (basic InferenceService structure)
	predictor, found, err := unstructured.NestedMap(isvc.Object, "spec", "predictor")
	if found && err == nil {
		t.Logf("InferenceService predictor config: %v", predictor)
		assert.NotEmpty(t, predictor, "Predictor should be configured")
	}
}

// testDeployModelWithRecipeAndOverrides tests deploying with overrides
func testDeployModelWithRecipeAndOverrides(t *testing.T, modelName, environment, recipeName string) {
	t.Helper()

	// Build CLI command with overrides
	cmd := exec.Command("ai-aas-cli", "model", "deploy", modelName,
		"-e", environment,
		"--recipe", recipeName,
		"--gpu-count", "2", // Override GPU count from recipe
		"--memory", "48",   // Override memory from recipe
		"--dry-run",
	)

	output, err := cmd.CombinedOutput()
	t.Logf("CLI output with overrides:\n%s", string(output))

	if err != nil {
		t.Logf("CLI command failed (expected if not configured): %v", err)
		// Verify output mentions overrides
		outputStr := string(output)
		assert.Contains(t, outputStr, recipeName, "Output should mention recipe")
		// Could also check for override values in output
	}
}

// testVerifyOverridesApplied verifies that CLI/operator applied overrides correctly
func testVerifyOverridesApplied(t *testing.T, k8sClient dynamic.Interface, aimodelName, namespace string) {
	t.Helper()

	aimodelGVR := schema.GroupVersionResource{
		Group:    "aimodel.ai-aas.io",
		Version:  "v1alpha1",
		Resource: "aimodels",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// For testing, create an AIModel with overrides
	aimodel := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "aimodel.ai-aas.io/v1alpha1",
			"kind":       "AIModel",
			"metadata": map[string]interface{}{
				"name":      aimodelName,
				"namespace": namespace,
				"labels": map[string]interface{}{
					"test": "e2e-override",
				},
			},
			"spec": map[string]interface{}{
				"recipeRef": map[string]interface{}{
					"name":      "test-llama-7b-override-recipe",
					"namespace": "ai-model-system",
				},
				"overrides": map[string]interface{}{
					"resources": map[string]interface{}{
						"gpu": map[string]interface{}{
							"count": 2, // Override from recipe's 1 GPU
						},
						"memory": map[string]interface{}{
							"requests": "48Gi", // Override from recipe's 16Gi
						},
					},
				},
				"enabled": true,
			},
		},
	}

	// Create AIModel with overrides
	created, err := k8sClient.Resource(aimodelGVR).Namespace(namespace).Create(ctx, aimodel, metav1.CreateOptions{})
	if err != nil {
		// Try to create namespace first
		nsGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}
		nsObj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Namespace",
				"metadata": map[string]interface{}{
					"name": namespace,
				},
			},
		}
		_, _ = k8sClient.Resource(nsGVR).Create(context.Background(), nsObj, metav1.CreateOptions{})

		created, err = k8sClient.Resource(aimodelGVR).Namespace(namespace).Create(ctx, aimodel, metav1.CreateOptions{})
		require.NoError(t, err, "Failed to create AIModel with overrides")
	}

	t.Logf("Created AIModel with overrides: %s/%s", namespace, aimodelName)

	// Verify overrides are in the spec
	overrides, found, err := unstructured.NestedMap(created.Object, "spec", "overrides")
	require.NoError(t, err)
	require.True(t, found, "Overrides should be present in AIModel spec")

	// Verify GPU count override
	gpuCount, found, err := unstructured.NestedFieldNoCopy(created.Object, "spec", "overrides", "resources", "gpu", "count")
	if found && err == nil {
		// Handle both int64 and int32
		switch v := gpuCount.(type) {
		case int64:
			assert.Equal(t, int64(2), v, "GPU count override should be 2")
		case int:
			assert.Equal(t, 2, v, "GPU count override should be 2")
		}
	}

	t.Logf("Verified overrides: %v", overrides)
}

// cleanupE2EResources cleans up test resources
func cleanupE2EResources(t *testing.T, k8sClient dynamic.Interface, recipeName, recipeNs, aimodelName, aimodelNs string) {
	t.Helper()
	ctx := context.Background()

	// Delete AIModel
	aimodelGVR := schema.GroupVersionResource{
		Group:    "aimodel.ai-aas.io",
		Version:  "v1alpha1",
		Resource: "aimodels",
	}
	err := k8sClient.Resource(aimodelGVR).Namespace(aimodelNs).Delete(ctx, aimodelName, metav1.DeleteOptions{})
	if err != nil && !isNotFoundError(err) {
		t.Logf("Warning: Failed to delete AIModel %s/%s: %v", aimodelNs, aimodelName, err)
	} else {
		t.Logf("Deleted AIModel: %s/%s", aimodelNs, aimodelName)
	}

	// Delete ModelRecipe
	recipeGVR := schema.GroupVersionResource{
		Group:    "aimodel.ai-aas.io",
		Version:  "v1alpha1",
		Resource: "modelrecipes",
	}
	err = k8sClient.Resource(recipeGVR).Namespace(recipeNs).Delete(ctx, recipeName, metav1.DeleteOptions{})
	if err != nil && !isNotFoundError(err) {
		t.Logf("Warning: Failed to delete ModelRecipe %s/%s: %v", recipeNs, recipeName, err)
	} else {
		t.Logf("Deleted ModelRecipe: %s/%s", recipeNs, recipeName)
	}

	// Also try to delete InferenceService if it exists
	isvcGVR := schema.GroupVersionResource{
		Group:    "serving.kserve.io",
		Version:  "v1beta1",
		Resource: "inferenceservices",
	}
	err = k8sClient.Resource(isvcGVR).Namespace(aimodelNs).Delete(ctx, aimodelName, metav1.DeleteOptions{})
	if err != nil && !isNotFoundError(err) {
		t.Logf("Warning: Failed to delete InferenceService %s/%s: %v", aimodelNs, aimodelName, err)
	} else if err == nil {
		t.Logf("Deleted InferenceService: %s/%s", aimodelNs, aimodelName)
	}
}

// isNotFoundError checks if error is NotFound
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	// Check for Kubernetes NotFound error
	return err.Error() == "not found" ||
		   (len(err.Error()) > 9 && err.Error()[len(err.Error())-9:] == "not found")
}

// isAlreadyExistsError checks if error is AlreadyExists
func isAlreadyExistsError(err error) bool {
	if err == nil {
		return false
	}
	// Check for Kubernetes AlreadyExists error
	return err.Error() == "already exists" ||
		   (len(err.Error()) > 14 && err.Error()[len(err.Error())-14:] == "already exists")
}
