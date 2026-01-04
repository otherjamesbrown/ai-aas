package usecases_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// FixtureManager manages test fixtures and cleanup for UC tests
type FixtureManager struct {
	t        *testing.T
	client   *TestClient
	fixtures []TestFixture
}

// TestFixture represents a test resource that needs cleanup
type TestFixture struct {
	Type      string
	ID        string
	CreatedAt time.Time
	Metadata  map[string]string
	CleanupFn func() error
}

// NewFixtureManager creates a new fixture manager for a test
func NewFixtureManager(t *testing.T, client *TestClient) *FixtureManager {
	t.Helper()
	fm := &FixtureManager{
		t:        t,
		client:   client,
		fixtures: []TestFixture{},
	}

	// Register cleanup to run at test end
	t.Cleanup(func() {
		if err := fm.Cleanup(); err != nil {
			t.Logf("Warning: Cleanup failed: %v", err)
		}
	})

	return fm
}

// Register registers a fixture for cleanup
func (fm *FixtureManager) Register(fixtureType, id string, metadata map[string]string, cleanupFn func() error) {
	fm.fixtures = append(fm.fixtures, TestFixture{
		Type:      fixtureType,
		ID:        id,
		CreatedAt: time.Now(),
		Metadata:  metadata,
		CleanupFn: cleanupFn,
	})
}

// Cleanup performs cleanup of all registered fixtures in reverse order
func (fm *FixtureManager) Cleanup() error {
	// Cleanup in reverse order (last created, first deleted)
	for i := len(fm.fixtures) - 1; i >= 0; i-- {
		fixture := fm.fixtures[i]
		if fixture.CleanupFn != nil {
			if err := fixture.CleanupFn(); err != nil {
				fm.t.Logf("Warning: Failed to cleanup %s %s: %v", fixture.Type, fixture.ID, err)
			}
		}
	}
	return nil
}

// OrganizationFixture provides organization CRUD for tests
type OrganizationFixture struct {
	fm     *FixtureManager
	client *TestClient
}

// NewOrganizationFixture creates a new organization fixture manager
func NewOrganizationFixture(fm *FixtureManager, client *TestClient) *OrganizationFixture {
	return &OrganizationFixture{
		fm:     fm,
		client: client,
	}
}

// Organization represents a test organization
type Organization struct {
	ID        string    `json:"orgId"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

// Create creates a test organization and registers it for cleanup
func (of *OrganizationFixture) Create(name string) (*Organization, error) {
	if name == "" {
		name = fmt.Sprintf("test-org-%s", generateUniqueID())
	}

	// Always add unique suffix to slug to prevent collisions, even when name is provided
	slug := fmt.Sprintf("%s-%s", name, generateUniqueID())
	reqBody := map[string]interface{}{
		"name": name,
		"slug": slug,
	}

	resp, err := of.client.POST("/v1/orgs", reqBody)
	if err != nil {
		return nil, fmt.Errorf("create organization: %w", err)
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return nil, fmt.Errorf("create organization failed: status %d, body: %s", resp.StatusCode, string(resp.Body))
	}

	var org Organization
	if err := json.Unmarshal(resp.Body, &org); err != nil {
		return nil, fmt.Errorf("unmarshal organization: %w", err)
	}

	// Register for cleanup
	of.fm.Register("organization", org.ID, map[string]string{
		"name": org.Name,
	}, func() error {
		return of.Delete(org.ID)
	})

	return &org, nil
}

// Delete deletes an organization
func (of *OrganizationFixture) Delete(id string) error {
	resp, err := of.client.DELETE(fmt.Sprintf("/v1/orgs/%s", id))
	if err != nil {
		return fmt.Errorf("delete organization: %w", err)
	}

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return fmt.Errorf("delete organization failed: status %d, body: %s", resp.StatusCode, string(resp.Body))
	}

	return nil
}

// ServiceAccountFixture provides service account CRUD for tests
type ServiceAccountFixture struct {
	fm     *FixtureManager
	client *TestClient
}

// NewServiceAccountFixture creates a new service account fixture manager
func NewServiceAccountFixture(fm *FixtureManager, client *TestClient) *ServiceAccountFixture {
	return &ServiceAccountFixture{
		fm:     fm,
		client: client,
	}
}

// ServiceAccount represents a test service account
type ServiceAccount struct {
	ID             string    `json:"serviceAccountId"`
	Name           string    `json:"name"`
	OrganizationID string    `json:"orgId"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"createdAt"`
}

// Create creates a test service account and registers it for cleanup
func (saf *ServiceAccountFixture) Create(orgID, name string) (*ServiceAccount, error) {
	if name == "" {
		name = fmt.Sprintf("test-sa-%s", generateUniqueID())
	}

	reqBody := map[string]interface{}{
		"name": name,
	}

	resp, err := saf.client.POST(fmt.Sprintf("/v1/orgs/%s/service-accounts", orgID), reqBody)
	if err != nil {
		return nil, fmt.Errorf("create service account: %w", err)
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return nil, fmt.Errorf("create service account failed: status %d, body: %s", resp.StatusCode, string(resp.Body))
	}

	var sa ServiceAccount
	if err := json.Unmarshal(resp.Body, &sa); err != nil {
		return nil, fmt.Errorf("unmarshal service account: %w", err)
	}

	// Register for cleanup
	saf.fm.Register("service_account", sa.ID, map[string]string{
		"name":  sa.Name,
		"orgId": orgID,
	}, func() error {
		return saf.Delete(orgID, sa.ID)
	})

	return &sa, nil
}

// Delete deletes a service account
func (saf *ServiceAccountFixture) Delete(orgID, serviceAccountID string) error {
	resp, err := saf.client.DELETE(fmt.Sprintf("/v1/orgs/%s/service-accounts/%s", orgID, serviceAccountID))
	if err != nil {
		return fmt.Errorf("delete service account: %w", err)
	}

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return fmt.Errorf("delete service account failed: status %d, body: %s", resp.StatusCode, string(resp.Body))
	}

	return nil
}

// APIKeyFixture provides API key CRUD for tests
type APIKeyFixture struct {
	fm     *FixtureManager
	client *TestClient
}

// NewAPIKeyFixture creates a new API key fixture manager
func NewAPIKeyFixture(fm *FixtureManager, client *TestClient) *APIKeyFixture {
	return &APIKeyFixture{
		fm:     fm,
		client: client,
	}
}

// APIKey represents a test API key
type APIKey struct {
	ID          string `json:"keyId"`
	Key         string `json:"token"`
	Fingerprint string `json:"fingerprint"`
	Status      string `json:"status"`
	Name        string `json:"name"`
}

// Create creates a test API key and registers it for cleanup
// Requires orgID and serviceAccountID - use ServiceAccountFixture.Create first
func (akf *APIKeyFixture) Create(orgID, serviceAccountID, name string, scopes []string) (*APIKey, error) {
	if name == "" {
		name = fmt.Sprintf("test-key-%s", generateUniqueID())
	}
	if scopes == nil {
		scopes = []string{"inference:read", "inference:write"}
	}

	reqBody := map[string]interface{}{
		"name":   name,
		"scopes": scopes,
	}

	resp, err := akf.client.POST(fmt.Sprintf("/v1/orgs/%s/service-accounts/%s/api-keys", orgID, serviceAccountID), reqBody)
	if err != nil {
		return nil, fmt.Errorf("create API key: %w", err)
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return nil, fmt.Errorf("create API key failed: status %d, body: %s", resp.StatusCode, string(resp.Body))
	}

	var apiKey APIKey
	if err := json.Unmarshal(resp.Body, &apiKey); err != nil {
		return nil, fmt.Errorf("unmarshal API key: %w", err)
	}

	// Register for cleanup
	akf.fm.Register("api_key", apiKey.ID, map[string]string{
		"name":  apiKey.Name,
		"orgId": orgID,
	}, func() error {
		return akf.Delete(orgID, apiKey.ID)
	})

	return &apiKey, nil
}

// CreateWithServiceAccount is a convenience method that creates a service account
// and then creates an API key for it
func (akf *APIKeyFixture) CreateWithServiceAccount(orgID, name string, scopes []string) (*APIKey, error) {
	// Create service account first
	saFixture := NewServiceAccountFixture(akf.fm, akf.client)
	sa, err := saFixture.Create(orgID, "")
	if err != nil {
		return nil, fmt.Errorf("create service account for API key: %w", err)
	}

	// Then create API key
	return akf.Create(orgID, sa.ID, name, scopes)
}

// Delete deletes an API key
func (akf *APIKeyFixture) Delete(orgID, keyID string) error {
	resp, err := akf.client.DELETE(fmt.Sprintf("/v1/orgs/%s/api-keys/%s", orgID, keyID))
	if err != nil {
		return fmt.Errorf("delete API key: %w", err)
	}

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return fmt.Errorf("delete API key failed: status %d, body: %s", resp.StatusCode, string(resp.Body))
	}

	return nil
}

// ModelDeploymentFixture provides model deployment operations for tests
type ModelDeploymentFixture struct {
	fm     *FixtureManager
	client *TestClient
}

// NewModelDeploymentFixture creates a new model deployment fixture manager
func NewModelDeploymentFixture(fm *FixtureManager, client *TestClient) *ModelDeploymentFixture {
	return &ModelDeploymentFixture{
		fm:     fm,
		client: client,
	}
}

// ModelDeployment represents a deployed model (matches Admin API response)
type ModelDeployment struct {
	ID                   string `json:"id"`
	ModelID              string `json:"model_id"`
	ModelName            string `json:"model_name,omitempty"`
	ExternalName         string `json:"external_name,omitempty"`
	CacheID              string `json:"cache_id,omitempty"`
	Environment          string `json:"environment"`
	Namespace            string `json:"namespace"`
	InferenceServiceName string `json:"inferenceservice_name,omitempty"`
	Endpoint             string `json:"endpoint,omitempty"`
	Enabled              bool   `json:"enabled"`
	Status               string `json:"status"`
	ReplicasDesired      int    `json:"replicas_desired"`
	ReplicasReady        int    `json:"replicas_ready"`
	GPUCount             int    `json:"gpu_count"`
	MemoryGB             int    `json:"memory_gb,omitempty"`
}

// CreateDeploymentRequest represents a request to create a deployment
type CreateDeploymentRequest struct {
	ModelName    string `json:"model_name"`
	ModelID      string `json:"model_id,omitempty"`
	ExternalName string `json:"external_name,omitempty"`
	CacheID      string `json:"cache_id,omitempty"`
	Environment  string `json:"environment"`
	Namespace    string `json:"namespace,omitempty"`
	GPUCount     int    `json:"gpu_count,omitempty"`
	MemoryGB     int    `json:"memory_gb,omitempty"`
	Replicas     int    `json:"replicas,omitempty"`
	ModelType    string `json:"model_type,omitempty"`
}

// Create creates a model deployment and registers it for cleanup
// Deprecated: Direct model deployment creation is blocked by Kyverno policy.
// Use GitOpsModelFixture.Create() instead.
func (mdf *ModelDeploymentFixture) Create(modelName, environment string) (*ModelDeployment, error) {
	return nil, fmt.Errorf("direct model deployment creation is blocked by Kyverno policy; use GitOpsModelFixture instead")
}

// CreateWithOptions creates a model deployment with custom options
// Deprecated: Direct model deployment creation is blocked by Kyverno policy.
// Use GitOpsModelFixture.Create() instead.
func (mdf *ModelDeploymentFixture) CreateWithOptions(req CreateDeploymentRequest) (*ModelDeployment, error) {
	return nil, fmt.Errorf("direct model deployment creation is blocked by Kyverno policy; use GitOpsModelFixture instead")
}

// Delete deletes a model deployment
// Deprecated: Direct model deployment deletion is blocked by Kyverno policy.
// GitOps model cleanup is handled automatically by GitOpsModelFixture.Delete().
func (mdf *ModelDeploymentFixture) Delete(modelName, environment string) error {
	return fmt.Errorf("direct model deployment deletion is blocked by Kyverno policy; use GitOpsModelFixture instead")
}

// Get retrieves a model deployment
// Uses Admin API: GET /v1/models/deployments/{model_name}/{environment}
func (mdf *ModelDeploymentFixture) Get(modelName, environment string) (*ModelDeployment, error) {
	path := fmt.Sprintf("/v1/models/deployments/%s/%s", modelName, environment)
	resp, err := mdf.client.GET(path)
	if err != nil {
		return nil, fmt.Errorf("get deployment request failed: %w", err)
	}

	if resp.StatusCode == 404 {
		return nil, nil // Not found
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("get deployment failed: status %d, body: %s", resp.StatusCode, resp.String())
	}

	var deployment ModelDeployment
	if err := resp.DecodeJSON(&deployment); err != nil {
		return nil, fmt.Errorf("parse deployment response: %w", err)
	}

	return &deployment, nil
}

// List retrieves all deployments with optional filters
// Uses Admin API: GET /v1/models/deployments
func (mdf *ModelDeploymentFixture) List(environment, modelName string) ([]ModelDeployment, error) {
	path := "/v1/models/deployments"
	params := []string{}
	if environment != "" {
		params = append(params, "environment="+environment)
	}
	if modelName != "" {
		params = append(params, "model_name="+modelName)
	}
	if len(params) > 0 {
		path += "?"
		for i, p := range params {
			if i > 0 {
				path += "&"
			}
			path += p
		}
	}

	resp, err := mdf.client.GET(path)
	if err != nil {
		return nil, fmt.Errorf("list deployments request failed: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("list deployments failed: status %d, body: %s", resp.StatusCode, resp.String())
	}

	var deployments []ModelDeployment
	if err := resp.DecodeJSON(&deployments); err != nil {
		return nil, fmt.Errorf("parse deployments response: %w", err)
	}

	return deployments, nil
}

// PreExistingModelReference represents a model already deployed in the cluster
// Used for read-only tests that don't need to create/destroy deployments
//
// Example usage:
//
//	func TestInference_WithPreExistingModel(t *testing.T) {
//	    // Get reference to well-known model
//	    modelRef, err := GetWellKnownModel("tinyllama")
//	    require.NoError(t, err)
//
//	    // Optionally verify it exists (recommended in test setup)
//	    client := setupTestClient(t)
//	    err = modelRef.VerifyModelExists(client)
//	    require.NoError(t, err)
//
//	    // Use the model for inference tests
//	    resp := client.POST("/v1/chat/completions", map[string]interface{}{
//	        "model": modelRef.ModelName,
//	        "messages": []map[string]string{{"role": "user", "content": "Hello"}},
//	    })
//	    require.Equal(t, 200, resp.StatusCode)
//	}
type PreExistingModelReference struct {
	ModelName   string // The model name as registered in Admin API
	Environment string // The environment where it's deployed (e.g., "development")
	Namespace   string // The Kubernetes namespace (e.g., "development", "system")
}

// WellKnownTestModels defines pre-deployed models that tests can rely on
// These models MUST be deployed and maintained in the development cluster
var WellKnownTestModels = map[string]PreExistingModelReference{
	"tinyllama": {
		ModelName:   "tinyllama-1.1b-chat",
		Environment: "development",
		Namespace:   "development",
	},
	"llama-3.1-8b": {
		ModelName:   "llama-3.1-8b-instruct",
		Environment: "development",
		Namespace:   "system",
	},
}

// GetWellKnownModel retrieves a pre-existing model reference by key
// Returns an error if the model key is not found
func GetWellKnownModel(key string) (*PreExistingModelReference, error) {
	model, exists := WellKnownTestModels[key]
	if !exists {
		return nil, fmt.Errorf("well-known model %q not found; available: tinyllama, llama-3.1-8b", key)
	}
	return &model, nil
}

// VerifyModelExists checks if the pre-existing model is actually deployed
// Uses the ModelDeploymentFixture to query the Admin API
func (ref *PreExistingModelReference) VerifyModelExists(client *TestClient) error {
	fm := &FixtureManager{
		t:        nil, // No cleanup needed for verification
		client:   client,
		fixtures: []TestFixture{},
	}
	mdf := NewModelDeploymentFixture(fm, client)

	deployment, err := mdf.Get(ref.ModelName, ref.Environment)
	if err != nil {
		return fmt.Errorf("failed to verify model %s in %s: %w", ref.ModelName, ref.Environment, err)
	}

	if deployment == nil {
		return fmt.Errorf("model %s not found in environment %s", ref.ModelName, ref.Environment)
	}

	if deployment.Status != "Running" && deployment.Status != "Ready" {
		return fmt.Errorf("model %s exists but status is %s (expected Running or Ready)", ref.ModelName, deployment.Status)
	}

	return nil
}

// GitOpsModelFixture provides GitOps-based model deployment operations for tests
type GitOpsModelFixture struct {
	fm             *FixtureManager
	configRepoPath string
	kubeconfig     string
	environment    string
	modelsCreated  []string
}

// ModelConfig defines the configuration for creating a GitOps model
type ModelConfig struct {
	// Model identification
	ModelName    string
	ModelID      string
	ExternalName string

	// Runtime configuration
	Runtime      string   // e.g., "vllm"
	ModelType    string   // e.g., "text"
	RuntimeArgs  []string // e.g., ["--max-model-len=2048"]

	// Resource allocation
	MinReplicas int
	MaxReplicas int
	CPURequest  string
	MemoryRequest string
	GPUCount    int
	CPULimit    string
	MemoryLimit string

	// Advanced options
	TrustRemoteCode bool
	Tolerations     []map[string]interface{}
	Labels          map[string]string
}

// DeployedModel represents a successfully deployed GitOps model
type DeployedModel struct {
	ModelName    string
	AIModelName  string // Kubernetes resource name
	Namespace    string
	Status       string
	ConfigPath   string // Path to the YAML file in ai-aas-config
}

// NewGitOpsModelFixture creates a new GitOps model fixture manager
func NewGitOpsModelFixture(fm *FixtureManager, cfg GitOpsTestConfig) (*GitOpsModelFixture, error) {
	fixture := &GitOpsModelFixture{
		fm:             fm,
		configRepoPath: cfg.ConfigRepoPath,
		kubeconfig:     cfg.Kubeconfig,
		environment:    cfg.Environment,
		modelsCreated:  []string{},
	}

	// Validate ai-aas-config repo path exists
	if _, err := os.Stat(cfg.ConfigRepoPath); err != nil {
		return nil, fmt.Errorf("ai-aas-config repo not found at %s: %w", cfg.ConfigRepoPath, err)
	}

	// Validate kubeconfig exists
	if _, err := os.Stat(cfg.Kubeconfig); err != nil {
		return nil, fmt.Errorf("kubeconfig not found at %s: %w", cfg.Kubeconfig, err)
	}

	return fixture, nil
}

// Create creates a GitOps model deployment
// Returns the deployed model information or an error
func (f *GitOpsModelFixture) Create(cfg ModelConfig) (*DeployedModel, error) {
	// Generate unique model name if not provided
	if cfg.ModelName == "" {
		cfg.ModelName = fmt.Sprintf("test-model-%s", generateUniqueID())
	}

	// Set defaults
	if cfg.Runtime == "" {
		cfg.Runtime = "vllm"
	}
	if cfg.ModelType == "" {
		cfg.ModelType = "text"
	}
	if cfg.MinReplicas == 0 {
		cfg.MinReplicas = 1
	}
	if cfg.MaxReplicas == 0 {
		cfg.MaxReplicas = 1
	}

	// Generate AIModel YAML manifest
	yamlContent := f.generateAIModelYAML(cfg)

	// Determine file path in ai-aas-config
	modelsDir := filepath.Join(f.configRepoPath, "environments", f.environment, "models")
	modelFileName := fmt.Sprintf("%s.yaml", cfg.ModelName)
	modelFilePath := filepath.Join(modelsDir, modelFileName)

	// Ensure models directory exists
	if err := os.MkdirAll(modelsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create models directory: %w", err)
	}

	// Write model YAML file
	if err := os.WriteFile(modelFilePath, []byte(yamlContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to write model file: %w", err)
	}

	// Commit to ai-aas-config
	commitMessage := fmt.Sprintf("test: add GitOps model %s for UC testing", cfg.ModelName)
	if err := f.gitCommitAndPush(commitMessage); err != nil {
		// Clean up file on failure
		os.Remove(modelFilePath)
		return nil, fmt.Errorf("failed to commit model config: %w", err)
	}

	// Track model for cleanup
	f.modelsCreated = append(f.modelsCreated, cfg.ModelName)

	// Register cleanup with FixtureManager
	f.fm.Register("gitops_model", cfg.ModelName, map[string]string{
		"environment": f.environment,
		"config_path": modelFilePath,
	}, func() error {
		return f.Delete(cfg.ModelName)
	})

	deployed := &DeployedModel{
		ModelName:   cfg.ModelName,
		AIModelName: cfg.ModelName,
		Namespace:   f.environment,
		Status:      "Pending",
		ConfigPath:  modelFilePath,
	}

	return deployed, nil
}

// Delete removes a GitOps model deployment
func (f *GitOpsModelFixture) Delete(modelName string) error {
	// Determine file path
	modelFilePath := filepath.Join(f.configRepoPath, "environments", f.environment, "models", fmt.Sprintf("%s.yaml", modelName))

	// Check if file exists
	if _, err := os.Stat(modelFilePath); os.IsNotExist(err) {
		// File already deleted, return nil (idempotent)
		return nil
	}

	// Remove file
	if err := os.Remove(modelFilePath); err != nil {
		return fmt.Errorf("failed to remove model file: %w", err)
	}

	// Commit and push removal
	commitMessage := fmt.Sprintf("test: remove GitOps model %s (UC test cleanup)", modelName)
	if err := f.gitCommitAndPush(commitMessage); err != nil {
		return fmt.Errorf("failed to commit model removal: %w", err)
	}

	return nil
}

// WaitForReady waits for an AIModel to reach Ready phase
func (f *GitOpsModelFixture) WaitForReady(ctx context.Context, modelName string) error {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Get current status for debugging
			status, _ := f.kubectlExec("get", "aimodel", modelName, "-n", f.environment,
				"-o", "jsonpath={.status.phase} - {.status.message}")
			return fmt.Errorf("timeout waiting for AIModel %s to be Ready, current status: %s", modelName, status)
		case <-ticker.C:
			output, err := f.kubectlExec("get", "aimodel", modelName, "-n", f.environment,
				"-o", "jsonpath={.status.phase}")
			if err == nil {
				phase := strings.TrimSpace(output)
				if phase == "Ready" {
					return nil
				}
				if phase == "Failed" {
					msg, _ := f.kubectlExec("get", "aimodel", modelName, "-n", f.environment,
						"-o", "jsonpath={.status.message}")
					return fmt.Errorf("AIModel %s failed: %s", modelName, msg)
				}
			}
		}
	}
}

// generateAIModelYAML generates the AIModel YAML manifest
func (f *GitOpsModelFixture) generateAIModelYAML(cfg ModelConfig) string {
	var buf strings.Builder

	buf.WriteString(fmt.Sprintf(`# AIModel CR for GitOps Test - %s
#
# This model is created by UC tests and will be automatically cleaned up.
#
apiVersion: aimodel.ai-aas.io/v1alpha1
kind: AIModel
metadata:
  name: %s
  namespace: %s
  labels:
    app: vllm-inference
    runtime: %s
    environment: %s
    test: gitops-uc
`, f.environment, cfg.ModelName, f.environment, cfg.Runtime, f.environment))

	// Add custom labels if provided
	for k, v := range cfg.Labels {
		buf.WriteString(fmt.Sprintf("    %s: %s\n", k, v))
	}

	buf.WriteString("spec:\n")
	buf.WriteString(fmt.Sprintf("  modelName: %s\n", cfg.ModelName))
	buf.WriteString(fmt.Sprintf("  modelID: %s\n", cfg.ModelID))

	if cfg.ExternalName != "" {
		buf.WriteString(fmt.Sprintf("  externalName: %s\n", cfg.ExternalName))
	}

	buf.WriteString("  enabled: true\n")
	buf.WriteString(fmt.Sprintf("  runtime: %s\n", cfg.Runtime))
	buf.WriteString(fmt.Sprintf("  modelType: %s\n", cfg.ModelType))

	if cfg.TrustRemoteCode {
		buf.WriteString("  trustRemoteCode: true\n")
	}

	buf.WriteString(fmt.Sprintf("  minReplicas: %d\n", cfg.MinReplicas))
	buf.WriteString(fmt.Sprintf("  maxReplicas: %d\n", cfg.MaxReplicas))

	// Runtime args
	if len(cfg.RuntimeArgs) > 0 {
		buf.WriteString("  runtimeArgs:\n")
		for _, arg := range cfg.RuntimeArgs {
			buf.WriteString(fmt.Sprintf("    - %q\n", arg))
		}
	}

	// Resources
	buf.WriteString("  resources:\n")
	buf.WriteString("    requests:\n")
	if cfg.CPURequest != "" {
		buf.WriteString(fmt.Sprintf("      cpu: %q\n", cfg.CPURequest))
	} else {
		buf.WriteString("      cpu: \"2\"\n")
	}
	if cfg.MemoryRequest != "" {
		buf.WriteString(fmt.Sprintf("      memory: %q\n", cfg.MemoryRequest))
	} else {
		buf.WriteString("      memory: \"8Gi\"\n")
	}
	if cfg.GPUCount > 0 {
		buf.WriteString(fmt.Sprintf("      nvidia.com/gpu: \"%d\"\n", cfg.GPUCount))
	}

	buf.WriteString("    limits:\n")
	if cfg.CPULimit != "" {
		buf.WriteString(fmt.Sprintf("      cpu: %q\n", cfg.CPULimit))
	} else {
		buf.WriteString("      cpu: \"4\"\n")
	}
	if cfg.MemoryLimit != "" {
		buf.WriteString(fmt.Sprintf("      memory: %q\n", cfg.MemoryLimit))
	} else {
		buf.WriteString("      memory: \"16Gi\"\n")
	}
	if cfg.GPUCount > 0 {
		buf.WriteString(fmt.Sprintf("      nvidia.com/gpu: \"%d\"\n", cfg.GPUCount))
	}

	// Tolerations
	if len(cfg.Tolerations) > 0 {
		buf.WriteString("  tolerations:\n")
		for _, toleration := range cfg.Tolerations {
			buf.WriteString("    - ")
			first := true
			for k, v := range toleration {
				if !first {
					buf.WriteString("\n      ")
				}
				buf.WriteString(fmt.Sprintf("%s: %v\n", k, v))
				first = false
			}
		}
	} else {
		// Default GPU toleration
		buf.WriteString("  tolerations:\n")
		buf.WriteString("    - key: nvidia.com/gpu\n")
		buf.WriteString("      operator: Exists\n")
		buf.WriteString("      effect: NoSchedule\n")
	}

	return buf.String()
}

// gitExec runs a git command in the config repo
func (f *GitOpsModelFixture) gitExec(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = f.configRepoPath
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// gitCommitAndPush commits and pushes changes to the config repo
func (f *GitOpsModelFixture) gitCommitAndPush(message string) error {
	// Pull latest first to avoid conflicts
	if _, err := f.gitExec("pull", "--rebase", "origin", "develop"); err != nil {
		// Log warning but continue - may be ok if no remote changes
		fmt.Printf("Warning: git pull failed (may be ok): %v\n", err)
	}

	// Add all changes
	if _, err := f.gitExec("add", "-A"); err != nil {
		return fmt.Errorf("git add failed: %w", err)
	}

	// Check if there are changes to commit
	status, _ := f.gitExec("status", "--porcelain")
	if strings.TrimSpace(status) == "" {
		// No changes to commit
		return nil
	}

	// Commit
	if _, err := f.gitExec("commit", "-m", message); err != nil {
		return fmt.Errorf("git commit failed: %w", err)
	}

	// Push
	if output, err := f.gitExec("push", "origin", "develop"); err != nil {
		return fmt.Errorf("git push failed: %s: %w", output, err)
	}

	return nil
}

// kubectlExec runs a kubectl command
func (f *GitOpsModelFixture) kubectlExec(args ...string) (string, error) {
	fullArgs := append([]string{"--kubeconfig=" + f.kubeconfig}, args...)
	cmd := exec.Command("kubectl", fullArgs...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// BenchmarkRunFixture provides benchmark run lifecycle operations for tests
type BenchmarkRunFixture struct {
	fm        *FixtureManager
	t         *testing.T
	orgCtx    *TestOrgContext
	runsCreated []string
}

// BenchmarkTarget represents a benchmark target
type BenchmarkTarget struct {
	Name     string
	Model    string
	Scenario string
}

// BenchmarkRun represents a benchmark run
type BenchmarkRun struct {
	ID       string
	TargetID string
	Status   string
	Results  map[string]interface{}
}

// NewBenchmarkRunFixture creates a new benchmark run fixture manager
func NewBenchmarkRunFixture(fm *FixtureManager, t *testing.T, orgCtx *TestOrgContext) *BenchmarkRunFixture {
	return &BenchmarkRunFixture{
		fm:          fm,
		t:           t,
		orgCtx:      orgCtx,
		runsCreated: []string{},
	}
}

// CreateTarget creates a benchmark target and registers it for cleanup
func (brf *BenchmarkRunFixture) CreateTarget(targetName, model, scenario string) (*BenchmarkTarget, error) {
	if targetName == "" {
		targetName = fmt.Sprintf("test-target-%s", generateUniqueID())
	}
	if model == "" {
		model = "llama-7b"
	}
	if scenario == "" {
		scenario = "standard"
	}

	// Create target using CLI
	result := brf.orgCtx.RunCLI("benchmark", "target", "add", targetName, "--model", model, "--scenario", scenario)
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("failed to create benchmark target: %s", result.Output)
	}

	target := &BenchmarkTarget{
		Name:     targetName,
		Model:    model,
		Scenario: scenario,
	}

	// Register for cleanup
	brf.fm.Register("benchmark_target", targetName, map[string]string{
		"model":    model,
		"scenario": scenario,
	}, func() error {
		return brf.DeleteTarget(targetName)
	})

	return target, nil
}

// DeleteTarget deletes a benchmark target
func (brf *BenchmarkRunFixture) DeleteTarget(targetName string) error {
	result := brf.orgCtx.RunCLI("benchmark", "target", "remove", targetName, "--force")
	if result.ExitCode != 0 && !strings.Contains(result.Output, "not found") {
		return fmt.Errorf("failed to delete benchmark target: %s", result.Output)
	}
	return nil
}

// TriggerRun triggers a benchmark run and returns the run information
func (brf *BenchmarkRunFixture) TriggerRun(targetName string) (*BenchmarkRun, error) {
	result := brf.orgCtx.RunCLI("benchmark", "run", "trigger", targetName)
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("failed to trigger benchmark run: %s", result.Output)
	}

	// Extract run ID from output (assuming format: "Run ID: <id>" or similar)
	// For now, we'll try to parse the JSON output if --json is supported,
	// or extract from text output
	var runID string

	// Try JSON output first
	jsonResult := brf.orgCtx.RunCLI("benchmark", "run", "trigger", targetName, "--json")
	if jsonResult.ExitCode == 0 && isValidJSON(jsonResult.Output) {
		var runData map[string]interface{}
		if err := json.Unmarshal([]byte(jsonResult.Output), &runData); err == nil {
			if id, ok := runData["id"].(string); ok {
				runID = id
			} else if id, ok := runData["runId"].(string); ok {
				runID = id
			}
		}
	}

	// Fallback: try to extract from text output
	if runID == "" {
		// Look for patterns like "Run ID: xyz" or "ID: xyz"
		lines := strings.Split(result.Output, "\n")
		for _, line := range lines {
			if strings.Contains(strings.ToLower(line), "run") && strings.Contains(strings.ToLower(line), "id") {
				// Simple extraction - assumes format "... ID: <value>"
				parts := strings.Split(line, ":")
				if len(parts) >= 2 {
					runID = strings.TrimSpace(parts[len(parts)-1])
					break
				}
			}
		}
	}

	if runID == "" {
		return nil, fmt.Errorf("failed to extract run ID from output: %s", result.Output)
	}

	run := &BenchmarkRun{
		ID:       runID,
		TargetID: targetName,
		Status:   "pending",
	}

	// Track run for cleanup (optional - runs may auto-cleanup)
	brf.runsCreated = append(brf.runsCreated, runID)

	return run, nil
}

// WaitForCompletion waits for a benchmark run to complete or fail
// Returns the final run status and any error
// timeout: maximum time to wait (e.g., 5*time.Minute)
func (brf *BenchmarkRunFixture) WaitForCompletion(ctx context.Context, runID string, timeout time.Duration) (*BenchmarkRun, error) {
	// NOTE: This implementation uses polling with `benchmark run show` since
	// `benchmark run wait` command doesn't exist yet (see UC-BM-005)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for benchmark run %s to complete", runID)
		case <-ticker.C:
			// Check run status
			result := brf.orgCtx.RunCLI("benchmark", "run", "show", runID, "--json")
			if result.ExitCode != 0 {
				// Run might not exist yet or API error - continue polling
				continue
			}

			var runData map[string]interface{}
			if err := json.Unmarshal([]byte(result.Output), &runData); err != nil {
				// Parse error - continue polling
				continue
			}

			status, ok := runData["status"].(string)
			if !ok {
				// No status field - continue polling
				continue
			}

			// Check for terminal states
			status = strings.ToLower(status)
			switch status {
			case "completed", "success", "succeeded":
				return &BenchmarkRun{
					ID:      runID,
					Status:  status,
					Results: runData,
				}, nil
			case "failed", "error":
				return &BenchmarkRun{
					ID:      runID,
					Status:  status,
					Results: runData,
				}, fmt.Errorf("benchmark run failed: %s", status)
			case "cancelled", "canceled":
				return &BenchmarkRun{
					ID:      runID,
					Status:  status,
					Results: runData,
				}, fmt.Errorf("benchmark run was cancelled")
			// Non-terminal states: pending, running, queued, etc.
			default:
				// Continue polling
				continue
			}
		}
	}
}

// GetRunStatus retrieves the current status of a benchmark run
func (brf *BenchmarkRunFixture) GetRunStatus(runID string) (*BenchmarkRun, error) {
	result := brf.orgCtx.RunCLI("benchmark", "run", "show", runID, "--json")
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("failed to get run status: %s", result.Output)
	}

	var runData map[string]interface{}
	if err := json.Unmarshal([]byte(result.Output), &runData); err != nil {
		return nil, fmt.Errorf("failed to parse run data: %w", err)
	}

	status, _ := runData["status"].(string)

	return &BenchmarkRun{
		ID:      runID,
		Status:  status,
		Results: runData,
	}, nil
}
