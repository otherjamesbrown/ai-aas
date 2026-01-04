// Package admin provides pipeline diagnostic operations.
package admin

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/client"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/gitops"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/kubernetes"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/registry"
)

// PipelineClient provides pipeline diagnostic operations
type PipelineClient struct {
	api *api.Client
}

// NewPipelineClient creates a new pipeline client
func NewPipelineClient(apiClient *api.Client) *PipelineClient {
	return &PipelineClient{api: apiClient}
}

// PipelineStatus represents the complete pipeline status for a model
type PipelineStatus struct {
	ModelName           string         `json:"model_name"`
	Environment         string         `json:"environment"`
	IsFullyOperational  bool           `json:"is_fully_operational"`
	OverallStatus       string         `json:"overall_status"`
	GitOps              PipelineStage  `json:"gitops"`
	Kubernetes          PipelineStage  `json:"kubernetes"`
	AdminAPI            PipelineStage  `json:"admin_api"`
	RoutingPolicy       PipelineStage  `json:"routing_policy"`
	InferenceEndpoint   PipelineStage  `json:"inference_endpoint"`
	SuggestedActions    []string       `json:"suggested_actions,omitempty"`
	CheckedAt           time.Time      `json:"checked_at"`
}

// PipelineStage represents a single stage in the pipeline
type PipelineStage struct {
	Name    string        `json:"name"`
	Status  string        `json:"status"` // ok, warning, error
	Checks  []StageCheck  `json:"checks"`
	Message string        `json:"message,omitempty"`
}

// StageCheck represents a single check within a stage
type StageCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // ok, warning, error, missing
	Value   string `json:"value"`
	Details string `json:"details,omitempty"`
	Error   string `json:"error,omitempty"`
}

// GetPipelineStatus retrieves the complete pipeline status for a model
func (c *PipelineClient) GetPipelineStatus(ctx context.Context, modelName, environment string) (*PipelineStatus, error) {
	status := &PipelineStatus{
		ModelName:   modelName,
		Environment: environment,
		CheckedAt:   time.Now(),
	}

	// Check each stage
	status.GitOps = c.checkGitOpsStage(ctx, modelName, environment)
	status.Kubernetes = c.checkKubernetesStage(ctx, modelName, environment)
	status.AdminAPI = c.checkAdminAPIStage(ctx, modelName, environment)
	status.RoutingPolicy = c.checkRoutingPolicyStage(ctx, modelName, environment)
	status.InferenceEndpoint = c.checkInferenceEndpointStage(ctx, modelName, environment)

	// Determine overall status
	status.IsFullyOperational = true
	allStatuses := []string{
		status.GitOps.Status,
		status.Kubernetes.Status,
		status.AdminAPI.Status,
		status.RoutingPolicy.Status,
		status.InferenceEndpoint.Status,
	}

	for _, s := range allStatuses {
		if s == "error" {
			status.IsFullyOperational = false
			status.OverallStatus = "Model has errors and is not operational"
			break
		}
	}

	if status.IsFullyOperational {
		for _, s := range allStatuses {
			if s == "warning" {
				status.OverallStatus = "Model is operational with warnings"
				break
			}
		}
	}

	if status.OverallStatus == "" {
		status.OverallStatus = "Model is fully operational"
	}

	// Generate suggested actions based on errors
	status.SuggestedActions = c.generateSuggestedActions(status)

	return status, nil
}

// checkGitOpsStage checks the GitOps stage (ai-aas-config repository)
func (c *PipelineClient) checkGitOpsStage(ctx context.Context, modelName, environment string) PipelineStage {
	stage := PipelineStage{
		Name:   "GitOps",
		Status: "ok",
		Checks: []StageCheck{},
	}

	// Try to find ai-aas-config repository
	configRepoPath := c.findConfigRepo()
	if configRepoPath == "" {
		stage.Status = "warning"
		stage.Checks = append(stage.Checks, StageCheck{
			Name:    "AIModel CR",
			Status:  "warning",
			Value:   "Cannot verify (ai-aas-config not found)",
			Details: "Could not locate ai-aas-config repository. Skipping GitOps checks.",
		})
		return stage
	}

	// Create GitOps client
	gitClient, err := gitops.NewClient(configRepoPath)
	if err != nil {
		stage.Status = "warning"
		stage.Checks = append(stage.Checks, StageCheck{
			Name:   "AIModel CR",
			Status: "warning",
			Value:  "Cannot verify",
			Error:  fmt.Sprintf("Failed to create GitOps client: %v", err),
		})
		return stage
	}

	// Check if model manifest exists
	manifestExists := gitClient.ModelManifestExists(environment, modelName)
	if manifestExists {
		manifestPath := filepath.Join("environments", environment, "models", modelName+".yaml")
		stage.Checks = append(stage.Checks, StageCheck{
			Name:    "AIModel CR",
			Status:  "ok",
			Value:   fmt.Sprintf("Found at %s", manifestPath),
			Details: fmt.Sprintf("Repository: %s", configRepoPath),
		})
	} else {
		stage.Status = "error"
		stage.Checks = append(stage.Checks, StageCheck{
			Name:   "AIModel CR",
			Status: "missing",
			Value:  "Not found in ai-aas-config",
			Error:  fmt.Sprintf("Expected at environments/%s/models/%s.yaml", environment, modelName),
		})
	}

	// Get current branch
	currentBranch, err := gitClient.CurrentBranch(ctx)
	if err == nil {
		expectedBranch := gitops.BranchForEnvironment(environment)
		if currentBranch == expectedBranch {
			stage.Checks = append(stage.Checks, StageCheck{
				Name:   "Target Revision",
				Status: "ok",
				Value:  currentBranch,
			})
		} else {
			stage.Status = "warning"
			stage.Checks = append(stage.Checks, StageCheck{
				Name:    "Target Revision",
				Status:  "warning",
				Value:   fmt.Sprintf("On %s (expected %s)", currentBranch, expectedBranch),
				Details: fmt.Sprintf("Environment %s typically uses branch %s", environment, expectedBranch),
			})
		}
	}

	// Note: Last sync time would require ArgoCD API integration
	stage.Checks = append(stage.Checks, StageCheck{
		Name:    "Last Sync",
		Status:  "ok",
		Value:   "Unknown (ArgoCD API not integrated)",
		Details: "Use ArgoCD UI to view sync status",
	})

	return stage
}

// checkKubernetesStage checks the Kubernetes stage (ai-model-operator)
func (c *PipelineClient) checkKubernetesStage(ctx context.Context, modelName, environment string) PipelineStage {
	stage := PipelineStage{
		Name:   "Kubernetes",
		Status: "ok",
		Checks: []StageCheck{},
	}

	// Create Kubernetes client
	k8sClient, err := kubernetes.NewClient(kubernetes.ClientConfig{
		Namespace: environment,
	})
	if err != nil {
		stage.Status = "error"
		stage.Checks = append(stage.Checks, StageCheck{
			Name:   "Kubernetes Access",
			Status: "error",
			Value:  "Cannot connect",
			Error:  fmt.Sprintf("Failed to create Kubernetes client: %v", err),
		})
		return stage
	}

	// Get AIModel status
	aimodel, err := k8sClient.GetAIModel(ctx, modelName, environment)
	if err != nil {
		stage.Status = "error"
		stage.Checks = append(stage.Checks, StageCheck{
			Name:   "AIModel",
			Status: "missing",
			Value:  "Not found",
			Error:  fmt.Sprintf("AIModel %s not found in namespace %s", modelName, environment),
		})
		return stage
	}

	// AIModel exists
	stage.Checks = append(stage.Checks, StageCheck{
		Name:   "AIModel",
		Status: "ok",
		Value:  string(aimodel.Phase),
	})

	// InferenceService
	if aimodel.InferenceServiceName != "" {
		isvcStatus := "ok"
		if aimodel.Phase != kubernetes.AIModelPhaseReady {
			isvcStatus = "warning"
		}
		stage.Checks = append(stage.Checks, StageCheck{
			Name:   "InferenceService",
			Status: isvcStatus,
			Value:  fmt.Sprintf("%s (%s)", aimodel.InferenceServiceName, aimodel.Phase),
		})
	} else {
		stage.Status = "warning"
		stage.Checks = append(stage.Checks, StageCheck{
			Name:   "InferenceService",
			Status: "warning",
			Value:  "Not created yet",
		})
	}

	// Pod count
	podStatus := fmt.Sprintf("%d Running", aimodel.ReadyReplicas)
	stage.Checks = append(stage.Checks, StageCheck{
		Name:   "Pods",
		Status: "ok",
		Value:  podStatus,
	})

	// GPU Allocation (would need pod details)
	stage.Checks = append(stage.Checks, StageCheck{
		Name:    "GPU Allocation",
		Status:  "ok",
		Value:   "Check pod spec",
		Details: "Use 'kubectl describe pod' for GPU details",
	})

	if aimodel.Message != "" {
		stage.Message = aimodel.Message
	}

	return stage
}

// checkAdminAPIStage checks the Admin API stage (model registry, deployments, backends)
func (c *PipelineClient) checkAdminAPIStage(ctx context.Context, modelName, environment string) PipelineStage {
	stage := PipelineStage{
		Name:   "Admin API",
		Status: "ok",
		Checks: []StageCheck{},
	}

	// Check model registry
	regClient := registry.NewClient(c.api)
	model, err := regClient.Get(ctx, modelName)
	if err != nil {
		stage.Status = "error"
		stage.Checks = append(stage.Checks, StageCheck{
			Name:   "Registry",
			Status: "missing",
			Value:  "Not registered",
			Error:  fmt.Sprintf("Model %s not found in registry", modelName),
		})
		return stage
	}

	// Model is registered
	stage.Checks = append(stage.Checks, StageCheck{
		Name:    "Registry",
		Status:  "ok",
		Value:   fmt.Sprintf("Registered (%s)", model.HFModelID),
		Details: fmt.Sprintf("ID: %s", model.ID),
	})

	// Check deployment
	deployClient := client.NewDeploymentClient(c.api)
	deployment, err := deployClient.Get(ctx, modelName, environment)
	if err != nil {
		stage.Status = "warning"
		stage.Checks = append(stage.Checks, StageCheck{
			Name:    "Deployment",
			Status:  "warning",
			Value:   "No deployment record",
			Details: "Model may be deployed via GitOps without Admin API sync",
		})
	} else {
		deployStatus := "ok"
		if deployment.Status != "ready" {
			deployStatus = "warning"
		}
		stage.Checks = append(stage.Checks, StageCheck{
			Name:   "Deployment",
			Status: deployStatus,
			Value:  fmt.Sprintf("%s (id: %s)", deployment.Status, deployment.ID),
		})

		// Backend
		if deployment.InferenceServiceName != "" {
			stage.Checks = append(stage.Checks, StageCheck{
				Name:   "Backend",
				Status: "ok",
				Value:  deployment.InferenceServiceName,
			})
		}
	}

	return stage
}

// checkRoutingPolicyStage checks the routing policy stage
func (c *PipelineClient) checkRoutingPolicyStage(ctx context.Context, modelName, environment string) PipelineStage {
	stage := PipelineStage{
		Name:   "Routing Policy",
		Status: "ok",
		Checks: []StageCheck{},
	}

	// List routing policies for this model
	// Note: This requires Admin API routing policy endpoint
	// For now, we'll make a best-effort check

	var policies []Policy
	if err := c.api.Get(ctx, "/v1/routing/policies?model="+modelName, &policies); err != nil {
		stage.Status = "warning"
		stage.Checks = append(stage.Checks, StageCheck{
			Name:    "Policy Check",
			Status:  "warning",
			Value:   "Cannot verify",
			Details: "Routing policy API not accessible or model not configured",
		})
		return stage
	}

	// Check for global policy
	globalPolicy := false
	orgPolicies := 0
	for _, p := range policies {
		if p.OrganizationID == "*" {
			globalPolicy = true
		} else {
			orgPolicies++
		}
	}

	if globalPolicy {
		stage.Checks = append(stage.Checks, StageCheck{
			Name:   "Global Policy",
			Status: "ok",
			Value:  "Exists",
		})
	} else {
		stage.Status = "error"
		stage.Checks = append(stage.Checks, StageCheck{
			Name:  "Global Policy",
			Status: "missing",
			Value: "Not configured",
			Error: "No global routing policy found. Model is not accessible.",
		})
	}

	stage.Checks = append(stage.Checks, StageCheck{
		Name:   "Org Policies",
		Status: "ok",
		Value:  fmt.Sprintf("%d custom policies", orgPolicies),
	})

	// Active backend (would need to query policy details)
	if globalPolicy {
		stage.Checks = append(stage.Checks, StageCheck{
			Name:   "Active Backend",
			Status: "ok",
			Value:  "Configured (see policy details)",
		})
	}

	return stage
}

// checkInferenceEndpointStage checks the inference endpoint health
func (c *PipelineClient) checkInferenceEndpointStage(ctx context.Context, modelName, environment string) PipelineStage {
	stage := PipelineStage{
		Name:   "Inference Endpoint",
		Status: "ok",
		Checks: []StageCheck{},
	}

	// Determine the endpoint URL based on environment
	// This is a simplified version - in reality, this would come from config
	baseURL := c.getInferenceBaseURL(environment)

	stage.Checks = append(stage.Checks, StageCheck{
		Name:   "URL",
		Status: "ok",
		Value:  baseURL + "/v1/chat/completions",
	})

	// Perform health check
	healthURL := baseURL + "/health"
	httpClient := &http.Client{
		Timeout: 5 * time.Second,
	}

	start := time.Now()
	resp, err := httpClient.Get(healthURL)
	latency := time.Since(start)

	if err != nil {
		stage.Status = "error"
		stage.Checks = append(stage.Checks, StageCheck{
			Name:   "Health",
			Status: "error",
			Value:  "Not responding",
			Error:  fmt.Sprintf("Health check failed: %v", err),
		})
		return stage
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		stage.Checks = append(stage.Checks, StageCheck{
			Name:   "Health",
			Status: "ok",
			Value:  fmt.Sprintf("Responding (%dms)", latency.Milliseconds()),
		})
	} else {
		stage.Status = "warning"
		stage.Checks = append(stage.Checks, StageCheck{
			Name:   "Health",
			Status: "warning",
			Value:  fmt.Sprintf("HTTP %d (%dms)", resp.StatusCode, latency.Milliseconds()),
		})
	}

	// Last request time (would need to be tracked by Admin API)
	stage.Checks = append(stage.Checks, StageCheck{
		Name:    "Last Request",
		Status:  "ok",
		Value:   "Unknown",
		Details: "Request tracking not implemented",
	})

	return stage
}

// generateSuggestedActions generates suggested actions based on pipeline errors
func (c *PipelineClient) generateSuggestedActions(status *PipelineStatus) []string {
	actions := []string{}

	// Check GitOps issues
	for _, check := range status.GitOps.Checks {
		if check.Status == "missing" && check.Name == "AIModel CR" {
			actions = append(actions, fmt.Sprintf("Create AIModel manifest in ai-aas-config: environments/%s/models/%s.yaml", status.Environment, status.ModelName))
		}
	}

	// Check Kubernetes issues
	for _, check := range status.Kubernetes.Checks {
		if check.Status == "missing" && check.Name == "AIModel" {
			actions = append(actions, fmt.Sprintf("Deploy model: ai-aas-cli model deploy create %s -e %s", status.ModelName, status.Environment))
		}
	}

	// Check Admin API issues
	for _, check := range status.AdminAPI.Checks {
		if check.Status == "missing" && check.Name == "Registry" {
			actions = append(actions, fmt.Sprintf("Register model: ai-aas-cli model registry add %s --name %s", "<hf-model-id>", status.ModelName))
		}
	}

	// Check routing policy issues
	for _, check := range status.RoutingPolicy.Checks {
		if check.Status == "missing" && check.Name == "Global Policy" {
			actions = append(actions, fmt.Sprintf("Create routing policy: ai-aas-cli routing policy create --model %s --backends <backend-id>", status.ModelName))
		}
	}

	// Check inference endpoint issues
	for _, check := range status.InferenceEndpoint.Checks {
		if check.Status == "error" {
			actions = append(actions, "Check api-router service status: kubectl get svc -n api-router")
			actions = append(actions, "Check api-router logs: kubectl logs -n api-router -l app=api-router")
		}
	}

	return actions
}

// findConfigRepo attempts to find the ai-aas-config repository
func (c *PipelineClient) findConfigRepo() string {
	// Common locations to check
	locations := []string{
		"~/ai-aas-config",
		"../ai-aas-config",
		"../../ai-aas-config",
	}

	for _, loc := range locations {
		// Expand ~
		if strings.HasPrefix(loc, "~/") {
			home, err := os.UserHomeDir()
			if err != nil {
				continue
			}
			loc = filepath.Join(home, loc[2:])
		}

		// Check if .git exists
		gitPath := filepath.Join(loc, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			return loc
		}
	}

	return ""
}

// getInferenceBaseURL returns the inference endpoint base URL for an environment
func (c *PipelineClient) getInferenceBaseURL(environment string) string {
	// This is a simplified version - in reality, this should come from config
	switch environment {
	case "development":
		return "https://api.dev.otherjamesbrown.com"
	case "staging":
		return "https://api.staging.otherjamesbrown.com"
	case "production":
		return "https://api.otherjamesbrown.com"
	default:
		return "https://api.dev.otherjamesbrown.com"
	}
}
