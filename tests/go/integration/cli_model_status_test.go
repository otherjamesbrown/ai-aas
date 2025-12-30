//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	testNamespace = "development"
	testTimeout   = 5 * time.Minute
)

var aiModelGVR = schema.GroupVersionResource{
	Group:    "aimodel.ai-aas.io",
	Version:  "v1alpha1",
	Resource: "aimodels",
}

// TestCLIModelStatusFailureModes validates the CLI correctly reports each failure mode
// with appropriate phase and messaging.
//
// References:
//   - UC-OPS-001: Model Status Observability
//   - Bead: aas-5f2xh
func TestCLIModelStatusFailureModes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Verify we can connect to cluster
	dynamicClient, err := getDynamicClient()
	if err != nil {
		t.Skipf("Cannot connect to kubernetes cluster: %v", err)
	}

	// Verify CLI is available
	cliPath := getCLIPath()
	if _, err := os.Stat(cliPath); err != nil {
		t.Fatalf("CLI binary not found at %s. Run ./scripts/build-clis.sh first", cliPath)
	}

	tests := []struct {
		name            string
		configFile      string
		expectedPhase   string
		expectedPattern string
		timeout         time.Duration
		skipReason      string // If set, skip this test with reason
	}{
		{
			name:            "UC-OPS-001/AC-03: insufficient CPU shows clear message",
			configFile:      "test-insufficient-cpu.yaml",
			expectedPhase:   "Scheduling",
			expectedPattern: `(?i)(insufficient|unavailable).*(cpu|cpus)`,
			timeout:         2 * time.Minute,
		},
		{
			name:            "UC-OPS-001/AC-03: insufficient memory shows clear message",
			configFile:      "test-insufficient-memory.yaml",
			expectedPhase:   "Scheduling",
			expectedPattern: `(?i)(insufficient|unavailable).*(memory|mem)`,
			timeout:         2 * time.Minute,
		},
		{
			name:            "UC-OPS-001/AC-04: missing toleration blocks scheduling",
			configFile:      "test-missing-toleration.yaml",
			expectedPhase:   "Scheduling",
			expectedPattern: `(?i)(taint|tolerat)`,
			timeout:         2 * time.Minute,
		},
		{
			name:            "UC-OPS-001/AC-03: no GPU available shows resource shortage",
			configFile:      "test-no-gpu-available.yaml",
			expectedPhase:   "Scheduling",
			expectedPattern: `(?i)(insufficient|unavailable).*(gpu|nvidia)`,
			timeout:         2 * time.Minute,
		},
		{
			name:            "UC-OPS-001/AC-05: autoscaler scaling up shows progress",
			configFile:      "test-needs-scaleup.yaml",
			expectedPhase:   "Scheduling",
			expectedPattern: `(?i)(scal|triggerscaleup|notriggerscaleup)`,
			timeout:         3 * time.Minute,
		},
		{
			name:            "UC-OPS-001/AC-05: autoscaler at max shows limit",
			configFile:      "test-at-max-scale.yaml",
			expectedPhase:   "Scheduling",
			expectedPattern: `(?i)(max|limit|node.?group)`,
			timeout:         2 * time.Minute,
		},
		{
			name:            "UC-OPS-001/AC-07: invalid S3 path shows download failure",
			configFile:      "test-invalid-s3-path.yaml",
			expectedPhase:   "Failed",
			expectedPattern: `(?i)(not.?found|404|s3|download)`,
			timeout:         3 * time.Minute,
		},
		{
			name:            "UC-OPS-001/AC-07: invalid HuggingFace model shows failure",
			configFile:      "test-invalid-hf-model.yaml",
			expectedPhase:   "Failed",
			expectedPattern: `(?i)(not.?found|404|huggingface)`,
			timeout:         3 * time.Minute,
		},
		{
			name:            "UC-OPS-001/AC-08: OOM killed shows memory error",
			configFile:      "test-oom-kill.yaml",
			expectedPhase:   "Failed",
			expectedPattern: `(?i)(oom|out.?of.?memory|killed)`,
			timeout:         4 * time.Minute,
		},
		{
			name:            "UC-OPS-001/AC-08: crash loop shows container failure",
			configFile:      "test-crash-loop.yaml",
			expectedPhase:   "Failed",
			expectedPattern: `(?i)(crash|crashloop|restart|exit)`,
			timeout:         4 * time.Minute,
		},
		{
			name:            "UC-OPS-001/AC-06: invalid recipe shows validation error",
			configFile:      "test-invalid-recipe.yaml",
			expectedPhase:   "Failed",
			expectedPattern: `(?i)(recipe|not.?found|invalid)`,
			timeout:         2 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipReason != "" {
				t.Skip(tt.skipReason)
			}

			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			// Get full path to config
			configPath := getConfigPath(tt.configFile)
			if _, err := os.Stat(configPath); err != nil {
				t.Fatalf("Config file not found: %s", configPath)
			}

			// Extract model name from config
			modelName := getModelNameFromConfig(t, configPath)

			// Ensure clean state - delete if exists
			_ = deleteAIModel(ctx, dynamicClient, modelName, testNamespace)
			time.Sleep(2 * time.Second)

			// Apply the test config (but keep enabled=false initially)
			t.Logf("Applying config: %s", configPath)
			applyConfig(t, configPath)
			defer func() {
				// Cleanup: delete the AIModel
				t.Logf("Cleaning up model: %s", modelName)
				_ = deleteAIModel(ctx, dynamicClient, modelName, testNamespace)
			}()

			// Enable the model to trigger deployment
			t.Logf("Enabling model: %s", modelName)
			enableModel(t, dynamicClient, modelName, testNamespace)

			// Wait for expected phase with timeout
			t.Logf("Waiting for phase: %s (timeout: %v)", tt.expectedPhase, tt.timeout)
			status, err := waitForPhase(ctx, dynamicClient, modelName, testNamespace, tt.expectedPhase)
			require.NoError(t, err, "Failed to reach expected phase %s", tt.expectedPhase)

			// Verify CLI status command returns correct information
			t.Logf("Testing CLI status output")
			output := runCLIStatus(t, modelName, testNamespace)

			// Verify phase is present in output
			assert.Contains(t, output, tt.expectedPhase,
				"CLI output should contain phase '%s'", tt.expectedPhase)

			// Verify expected pattern in output
			matched, err := regexp.MatchString(tt.expectedPattern, output)
			require.NoError(t, err, "Invalid regex pattern: %s", tt.expectedPattern)
			assert.True(t, matched,
				"CLI output should match pattern '%s'. Output:\n%s",
				tt.expectedPattern, output)

			// Log full status for debugging
			t.Logf("Final status:\n%s", output)
			t.Logf("Status details: Phase=%s, Message=%s",
				status.Phase, status.Message)
		})
	}
}

// TestCLIModelStatusNotFound verifies error handling when model doesn't exist
func TestCLIModelStatusNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cliPath := getCLIPath()
	if _, err := os.Stat(cliPath); err != nil {
		t.Fatalf("CLI binary not found at %s", cliPath)
	}

	cmd := exec.Command(cliPath, "model", "status", "nonexistent-model-12345",
		"--namespace", testNamespace)
	output, err := cmd.CombinedOutput()

	// Should fail
	assert.Error(t, err, "Command should fail for nonexistent model")

	// Output should indicate model not found
	outputStr := string(output)
	assert.Regexp(t, `(?i)(not.?found|does.?not.?exist)`, outputStr,
		"Error message should indicate model not found")
}

// TestCLIModelStatusJSONFormat verifies JSON output format
func TestCLIModelStatusJSONFormat(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test requires at least one deployed model
	// Skip if no models are available
	dynamicClient, err := getDynamicClient()
	if err != nil {
		t.Skipf("Cannot connect to kubernetes cluster: %v", err)
	}

	// List models
	list, err := dynamicClient.Resource(aiModelGVR).Namespace(testNamespace).List(
		context.Background(),
		metav1.ListOptions{Limit: 1},
	)
	if err != nil || len(list.Items) == 0 {
		t.Skip("No models available for testing JSON output")
	}

	modelName := list.Items[0].GetName()

	// Run status with --format json
	cliPath := getCLIPath()
	cmd := exec.Command(cliPath, "model", "status", modelName,
		"--namespace", testNamespace,
		"--format", "json")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "CLI status --format json should succeed")

	// Verify it's valid JSON
	outputStr := string(output)
	assert.True(t, strings.HasPrefix(strings.TrimSpace(outputStr), "{"),
		"JSON output should start with {")
	assert.Contains(t, outputStr, `"phase"`,
		"JSON output should contain phase field")
}

// Helper functions

// AIModelStatus represents parsed status from AIModel
type AIModelStatus struct {
	Phase              string
	Message            string
	SchedulingBlocked  bool
	SchedulingReason   string
	SchedulingMessage  string
	Progress           map[string]interface{}
	PhaseStartTime     *time.Time
}

func getDynamicClient() (dynamic.Interface, error) {
	// Use KUBECONFIG env var or default
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, _ := os.UserHomeDir()
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	return dynamic.NewForConfig(config)
}

func getCLIPath() string {
	// Check if CLI_PATH env var is set
	if cliPath := os.Getenv("CLI_PATH"); cliPath != "" {
		return cliPath
	}

	// Default to installed location
	home, _ := os.UserHomeDir()
	installedPath := filepath.Join(home, ".local", "bin", "ai-aas-cli")
	if _, err := os.Stat(installedPath); err == nil {
		return installedPath
	}

	// Fall back to source location
	repoRoot := getRepoRoot()
	return filepath.Join(repoRoot, "services", "ai-aas-cli", "ai-aas-cli")
}

func getRepoRoot() string {
	// Assume we're in tests/go/integration/
	cwd, _ := os.Getwd()
	// Navigate up to repo root
	root := filepath.Join(cwd, "..", "..", "..")
	absRoot, _ := filepath.Abs(root)
	return absRoot
}

func getConfigPath(filename string) string {
	repoRoot := getRepoRoot()
	return filepath.Join(repoRoot, "tests", "failure-modes", "configs", filename)
}

func getModelNameFromConfig(t *testing.T, configPath string) string {
	// Extract model name from YAML file
	// For these test configs, the model name is in metadata.name
	content, err := os.ReadFile(configPath)
	require.NoError(t, err, "Failed to read config file")

	// Simple regex to extract metadata.name
	re := regexp.MustCompile(`(?m)^\s*name:\s*(.+)$`)
	matches := re.FindStringSubmatch(string(content))
	require.True(t, len(matches) > 1, "Could not find metadata.name in config")

	return strings.TrimSpace(matches[1])
}

func applyConfig(t *testing.T, configPath string) {
	cmd := exec.Command("kubectl", "apply", "-f", configPath)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "kubectl apply failed: %s", string(output))
}

func deleteAIModel(ctx context.Context, client dynamic.Interface, name, namespace string) error {
	return client.Resource(aiModelGVR).Namespace(namespace).Delete(
		ctx,
		name,
		metav1.DeleteOptions{},
	)
}

func enableModel(t *testing.T, client dynamic.Interface, name, namespace string) {
	// Patch the AIModel to set spec.enabled=true
	patch := []byte(`{"spec": {"enabled": true}}`)
	_, err := client.Resource(aiModelGVR).Namespace(namespace).Patch(
		context.Background(),
		name,
		"application/merge-patch+json",
		patch,
		metav1.PatchOptions{},
	)
	require.NoError(t, err, "Failed to enable model")
}

func waitForPhase(ctx context.Context, client dynamic.Interface, name, namespace, expectedPhase string) (*AIModelStatus, error) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for phase %s", expectedPhase)
		case <-ticker.C:
			obj, err := client.Resource(aiModelGVR).Namespace(namespace).Get(
				context.Background(),
				name,
				metav1.GetOptions{},
			)
			if err != nil {
				continue
			}

			status := parseAIModelStatus(obj)
			if status.Phase == expectedPhase {
				return status, nil
			}

			// Log current phase for debugging
			fmt.Printf("  Current phase: %s (waiting for %s)\n", status.Phase, expectedPhase)
		}
	}
}

func parseAIModelStatus(obj *unstructured.Unstructured) *AIModelStatus {
	status := &AIModelStatus{}

	if phase, ok, _ := unstructured.NestedString(obj.Object, "status", "phase"); ok {
		status.Phase = phase
	}

	if message, ok, _ := unstructured.NestedString(obj.Object, "status", "message"); ok {
		status.Message = message
	}

	if schedulingMap, ok, _ := unstructured.NestedMap(obj.Object, "status", "schedulingStatus"); ok {
		if scheduled, ok := schedulingMap["scheduled"].(bool); ok {
			status.SchedulingBlocked = !scheduled
		}
		if reason, ok := schedulingMap["blockedReason"].(string); ok {
			status.SchedulingReason = reason
		}
		if msg, ok := schedulingMap["blockedMessage"].(string); ok {
			status.SchedulingMessage = msg
		}
	}

	if progress, ok, _ := unstructured.NestedMap(obj.Object, "status", "progress"); ok {
		status.Progress = progress
	}

	return status
}

func runCLIStatus(t *testing.T, modelName, namespace string) string {
	cliPath := getCLIPath()
	cmd := exec.Command(cliPath, "model", "status", modelName,
		"--namespace", namespace)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "CLI status command failed: %s", string(output))
	return string(output)
}
