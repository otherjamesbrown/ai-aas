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
// Uses Admin API: POST /v1/models/deployments
func (mdf *ModelDeploymentFixture) Create(modelName, environment string) (*ModelDeployment, error) {
	reqBody := CreateDeploymentRequest{
		ModelName:   modelName,
		Environment: environment,
		Replicas:    1,
	}

	resp, err := mdf.client.POST("/v1/models/deployments", reqBody)
	if err != nil {
		return nil, fmt.Errorf("create deployment request failed: %w", err)
	}

	if resp.StatusCode != 201 && resp.StatusCode != 200 {
		return nil, fmt.Errorf("create deployment failed: status %d, body: %s", resp.StatusCode, resp.String())
	}

	var deployment ModelDeployment
	if err := resp.DecodeJSON(&deployment); err != nil {
		return nil, fmt.Errorf("parse deployment response: %w", err)
	}

	// Register cleanup
	mdf.fm.Register("model_deployment", deployment.ModelName+"-"+deployment.Environment, map[string]string{
		"model_name":  deployment.ModelName,
		"environment": deployment.Environment,
	}, func() error {
		return mdf.Delete(deployment.ModelName, deployment.Environment)
	})

	return &deployment, nil
}

// CreateWithOptions creates a model deployment with custom options
func (mdf *ModelDeploymentFixture) CreateWithOptions(req CreateDeploymentRequest) (*ModelDeployment, error) {
	resp, err := mdf.client.POST("/v1/models/deployments", req)
	if err != nil {
		return nil, fmt.Errorf("create deployment request failed: %w", err)
	}

	if resp.StatusCode != 201 && resp.StatusCode != 200 {
		return nil, fmt.Errorf("create deployment failed: status %d, body: %s", resp.StatusCode, resp.String())
	}

	var deployment ModelDeployment
	if err := resp.DecodeJSON(&deployment); err != nil {
		return nil, fmt.Errorf("parse deployment response: %w", err)
	}

	// Register cleanup
	mdf.fm.Register("model_deployment", deployment.ModelName+"-"+deployment.Environment, map[string]string{
		"model_name":  deployment.ModelName,
		"environment": deployment.Environment,
	}, func() error {
		return mdf.Delete(deployment.ModelName, deployment.Environment)
	})

	return &deployment, nil
}

// Delete deletes a model deployment
// Uses Admin API: DELETE /v1/models/deployments/{model_name}/{environment}
func (mdf *ModelDeploymentFixture) Delete(modelName, environment string) error {
	path := fmt.Sprintf("/v1/models/deployments/%s/%s", modelName, environment)
	resp, err := mdf.client.DELETE(path)
	if err != nil {
		return fmt.Errorf("delete deployment request failed: %w", err)
	}

	// 204 No Content or 404 Not Found are both acceptable
	if resp.StatusCode != 204 && resp.StatusCode != 200 && resp.StatusCode != 404 {
		return fmt.Errorf("delete deployment failed: status %d, body: %s", resp.StatusCode, resp.String())
	}

	return nil
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
