// Package kubernetes provides Kubernetes client operations for model deployment.
package kubernetes

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// AIModelGVR is the GroupVersionResource for AIModel CRD
var AIModelGVR = schema.GroupVersionResource{
	Group:    "aimodel.ai-aas.io",
	Version:  "v1alpha1",
	Resource: "aimodels",
}

// AIModelConfig configures an AIModel deployment
type AIModelConfig struct {
	Name            string
	Namespace       string
	ModelName       string
	ModelID         string // HuggingFace model ID
	S3Bucket        string
	S3Key           string
	Runtime         string
	RuntimeName     string
	Enabled         bool
	MinReplicas     int
	MaxReplicas     int
	GPUCount        int
	MemoryGB        int
	TrustRemoteCode bool
	Environment     string
	Labels          map[string]string
	Annotations     map[string]string
	// Recipe support
	RecipeName      string // Name of the ModelRecipe to use
	RecipeNamespace string // Namespace of the ModelRecipe (defaults to ai-model-system)
	// Override tracking - set to true when the user explicitly sets the value
	OverrideGPUCount    bool
	OverrideMemory      bool
	OverrideMinReplicas bool
	OverrideMaxReplicas bool
}

// AIModelPhase represents the current phase of the AIModel deployment
type AIModelPhase string

const (
	AIModelPhasePending      AIModelPhase = "Pending"
	AIModelPhaseDownloading  AIModelPhase = "Downloading"
	AIModelPhaseDeploying    AIModelPhase = "Deploying"
	AIModelPhaseReady        AIModelPhase = "Ready"
	AIModelPhaseFailed       AIModelPhase = "Failed"
	AIModelPhaseDisabled     AIModelPhase = "Disabled"
	AIModelPhaseRetryPending AIModelPhase = "RetryPending"
)

// AIModelStatus represents the status of an AIModel
type AIModelStatus struct {
	Name                 string
	Namespace            string
	Phase                AIModelPhase
	InferenceServiceName string
	InferenceEndpoint    string
	ReadyReplicas        int32
	Message              string
	Conditions           []Condition
}

// CreateAIModel creates a new AIModel custom resource
func (c *Client) CreateAIModel(ctx context.Context, cfg AIModelConfig) error {
	dynamicClient, err := dynamic.NewForConfig(c.config)
	if err != nil {
		return fmt.Errorf("create dynamic client: %w", err)
	}

	namespace := cfg.Namespace
	if namespace == "" {
		namespace = c.namespace
	}

	// Build AIModel manifest
	aimodel := buildAIModelManifest(cfg)

	// Create the resource
	_, err = dynamicClient.Resource(AIModelGVR).Namespace(namespace).Create(
		ctx,
		aimodel,
		metav1.CreateOptions{},
	)
	if err != nil {
		return fmt.Errorf("create aimodel: %w", err)
	}

	return nil
}

// GetAIModel gets an AIModel by name
func (c *Client) GetAIModel(ctx context.Context, name, namespace string) (*AIModelStatus, error) {
	dynamicClient, err := dynamic.NewForConfig(c.config)
	if err != nil {
		return nil, fmt.Errorf("create dynamic client: %w", err)
	}

	if namespace == "" {
		namespace = c.namespace
	}

	result, err := dynamicClient.Resource(AIModelGVR).Namespace(namespace).Get(
		ctx,
		name,
		metav1.GetOptions{},
	)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("aimodel %s not found", name)
		}
		return nil, fmt.Errorf("get aimodel: %w", err)
	}

	status, err := parseAIModelStatus(result)
	if err != nil {
		return nil, err
	}

	return status, nil
}

// ResolveInferenceServiceName resolves the actual InferenceService name for a model deployment.
// It first checks the AIModel CR status (source of truth for GitOps deployments),
// then falls back to the CLI naming convention (model-environment) for backwards compatibility.
//
// Returns: inferenceServiceName, namespace, error
func (c *Client) ResolveInferenceServiceName(ctx context.Context, modelName, environment string) (string, string, error) {
	// Default namespace is the environment name
	namespace := environment

	// Try to get the AIModel status (works for GitOps-deployed models)
	// The AIModel name in GitOps is typically just the model name, not model-environment
	aimodel, err := c.GetAIModel(ctx, modelName, environment)
	if err == nil && aimodel.InferenceServiceName != "" {
		// Found AIModel with InferenceService name - use it
		if aimodel.Namespace != "" {
			namespace = aimodel.Namespace
		}
		return aimodel.InferenceServiceName, namespace, nil
	}

	// Try the CLI naming convention (model-environment) for CLI-created deployments
	cliStyleName := fmt.Sprintf("%s-%s", modelName, environment)
	aimodel, err = c.GetAIModel(ctx, cliStyleName, environment)
	if err == nil && aimodel.InferenceServiceName != "" {
		if aimodel.Namespace != "" {
			namespace = aimodel.Namespace
		}
		return aimodel.InferenceServiceName, namespace, nil
	}

	// Fall back to constructed name (for deployments not yet synced or missing)
	return cliStyleName, namespace, nil
}

// DeleteAIModel deletes an AIModel
func (c *Client) DeleteAIModel(ctx context.Context, name, namespace string) error {
	dynamicClient, err := dynamic.NewForConfig(c.config)
	if err != nil {
		return fmt.Errorf("create dynamic client: %w", err)
	}

	if namespace == "" {
		namespace = c.namespace
	}

	err = dynamicClient.Resource(AIModelGVR).Namespace(namespace).Delete(
		ctx,
		name,
		metav1.DeleteOptions{},
	)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("delete aimodel: %w", err)
	}

	return nil
}

// ListAIModels lists AIModels in a namespace
func (c *Client) ListAIModels(ctx context.Context, namespace string, labelSelector string) ([]AIModelStatus, error) {
	dynamicClient, err := dynamic.NewForConfig(c.config)
	if err != nil {
		return nil, fmt.Errorf("create dynamic client: %w", err)
	}

	if namespace == "" {
		namespace = c.namespace
	}

	opts := metav1.ListOptions{}
	if labelSelector != "" {
		opts.LabelSelector = labelSelector
	}

	result, err := dynamicClient.Resource(AIModelGVR).Namespace(namespace).List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("list aimodels: %w", err)
	}

	var models []AIModelStatus
	for _, item := range result.Items {
		status, err := parseAIModelStatus(&item)
		if err != nil {
			continue // Skip items that can't be parsed
		}
		models = append(models, *status)
	}

	return models, nil
}

func buildAIModelManifest(cfg AIModelConfig) *unstructured.Unstructured {
	labels := map[string]interface{}{
		"app.kubernetes.io/name":       cfg.ModelName,
		"app.kubernetes.io/managed-by": "ai-aas-cli",
		"ai-aas.io/environment":        cfg.Environment,
	}
	for k, v := range cfg.Labels {
		labels[k] = v
	}

	annotations := map[string]interface{}{}
	for k, v := range cfg.Annotations {
		annotations[k] = v
	}

	// Build resource requirements
	resources := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dGi", cfg.MemoryGB)),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dGi", cfg.MemoryGB/2)),
		},
	}
	if cfg.GPUCount > 0 {
		gpuQuantity := resource.MustParse(fmt.Sprintf("%d", cfg.GPUCount))
		resources.Limits["nvidia.com/gpu"] = gpuQuantity
		resources.Requests["nvidia.com/gpu"] = gpuQuantity
	}

	// Convert ResourceRequirements to map for unstructured
	resourcesMap := map[string]interface{}{
		"limits": map[string]interface{}{
			"memory": fmt.Sprintf("%dGi", cfg.MemoryGB),
		},
		"requests": map[string]interface{}{
			"memory": fmt.Sprintf("%dGi", cfg.MemoryGB/2),
		},
	}
	if cfg.GPUCount > 0 {
		resourcesMap["limits"].(map[string]interface{})["nvidia.com/gpu"] = cfg.GPUCount
		resourcesMap["requests"].(map[string]interface{})["nvidia.com/gpu"] = cfg.GPUCount
	}

	// Build spec based on recipe or inline configuration
	spec := map[string]interface{}{}

	// If using a recipe, build recipeRef and overrides
	if cfg.RecipeName != "" {
		// Add recipeRef
		recipeRef := map[string]interface{}{
			"name": cfg.RecipeName,
		}
		if cfg.RecipeNamespace != "" {
			recipeRef["namespace"] = cfg.RecipeNamespace
		} else {
			recipeRef["namespace"] = "ai-model-system" // default namespace
		}
		spec["recipeRef"] = recipeRef

		// Build overrides for explicitly set values
		overrides := map[string]interface{}{}

		// Resource overrides
		if cfg.OverrideGPUCount || cfg.OverrideMemory {
			resourceOverrides := map[string]interface{}{}

			if cfg.OverrideGPUCount || cfg.OverrideMemory {
				gpuOverrides := map[string]interface{}{}
				if cfg.OverrideGPUCount {
					gpuOverrides["count"] = int32(cfg.GPUCount)
				}
				resourceOverrides["gpu"] = gpuOverrides
			}

			if cfg.OverrideMemory {
				memoryOverrides := map[string]interface{}{
					"requests": fmt.Sprintf("%dGi", cfg.MemoryGB),
				}
				resourceOverrides["memory"] = memoryOverrides
			}

			overrides["resources"] = resourceOverrides
		}

		// Replica overrides
		if cfg.OverrideMinReplicas || cfg.OverrideMaxReplicas {
			replicaOverrides := map[string]interface{}{}
			if cfg.OverrideMinReplicas {
				replicaOverrides["min"] = int32(cfg.MinReplicas)
			}
			if cfg.OverrideMaxReplicas {
				replicaOverrides["max"] = int32(cfg.MaxReplicas)
			}
			overrides["replicas"] = replicaOverrides
		}

		if len(overrides) > 0 {
			spec["overrides"] = overrides
		}
	} else {
		// Inline configuration (backward compatible)
		spec = map[string]interface{}{
			"modelName": cfg.ModelName,
			"modelID":   cfg.ModelID,
			"s3Bucket":  cfg.S3Bucket,
			"s3Key":     cfg.S3Key,
			"enabled":   cfg.Enabled,
			"resources": resourcesMap,
		}

		// Add optional fields
		if cfg.Runtime != "" {
			spec["runtime"] = cfg.Runtime
		}
		if cfg.RuntimeName != "" {
			spec["runtimeName"] = cfg.RuntimeName
		}
		if cfg.MinReplicas > 0 {
			spec["minReplicas"] = int32(cfg.MinReplicas)
		}
		if cfg.MaxReplicas > 0 {
			spec["maxReplicas"] = int32(cfg.MaxReplicas)
		}
		if cfg.TrustRemoteCode {
			spec["trustRemoteCode"] = true
		}
	}

	aimodel := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "aimodel.ai-aas.io/v1alpha1",
			"kind":       "AIModel",
			"metadata": map[string]interface{}{
				"name":        cfg.Name,
				"namespace":   cfg.Namespace,
				"labels":      labels,
				"annotations": annotations,
			},
			"spec": spec,
		},
	}

	return aimodel
}

func parseAIModelStatus(obj *unstructured.Unstructured) (*AIModelStatus, error) {
	status := &AIModelStatus{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}

	// Get phase
	if phase, ok, _ := unstructured.NestedString(obj.Object, "status", "phase"); ok {
		status.Phase = AIModelPhase(phase)
	}

	// Get InferenceService name
	if isvcName, ok, _ := unstructured.NestedString(obj.Object, "status", "inferenceServiceName"); ok {
		status.InferenceServiceName = isvcName
	}

	// Get endpoint
	if endpoint, ok, _ := unstructured.NestedString(obj.Object, "status", "inferenceEndpoint"); ok {
		status.InferenceEndpoint = endpoint
	}

	// Get ready replicas
	if readyReplicas, ok, _ := unstructured.NestedInt64(obj.Object, "status", "readyReplicas"); ok {
		status.ReadyReplicas = int32(readyReplicas)
	}

	// Get message
	if message, ok, _ := unstructured.NestedString(obj.Object, "status", "message"); ok {
		status.Message = message
	}

	// Get conditions
	conditions, ok, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if ok {
		for _, c := range conditions {
			if cond, ok := c.(map[string]interface{}); ok {
				condition := Condition{}
				if t, ok := cond["type"].(string); ok {
					condition.Type = t
				}
				if s, ok := cond["status"].(string); ok {
					condition.Status = s
				}
				if r, ok := cond["reason"].(string); ok {
					condition.Reason = r
				}
				if m, ok := cond["message"].(string); ok {
					condition.Message = m
				}
				status.Conditions = append(status.Conditions, condition)
			}
		}
	}

	return status, nil
}

// WaitForAIModelReady waits for an AIModel to become ready
func (c *Client) WaitForAIModelReady(ctx context.Context, name, namespace string, opts WaitOptions) error {
	if opts.Timeout == 0 {
		opts.Timeout = 10 * time.Minute
	}
	if opts.PollInterval == 0 {
		opts.PollInterval = 5 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	ticker := time.NewTicker(opts.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for aimodel %s to be ready", name)
		case <-ticker.C:
			status, err := c.GetAIModel(ctx, name, namespace)
			if err != nil {
				// Keep waiting if not found yet
				continue
			}

			// Check if ready
			if status.Phase == AIModelPhaseReady {
				return nil
			}

			// Check for failure
			if status.Phase == AIModelPhaseFailed {
				return fmt.Errorf("aimodel deployment failed: %s", status.Message)
			}

			// For disabled models
			if status.Phase == AIModelPhaseDisabled {
				return fmt.Errorf("aimodel is disabled")
			}
		}
	}
}
