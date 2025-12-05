// Package kubernetes provides Kubernetes client operations for model deployment.
package kubernetes

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// InferenceServiceGVR is the GroupVersionResource for KServe InferenceService
var InferenceServiceGVR = schema.GroupVersionResource{
	Group:    "serving.kserve.io",
	Version:  "v1beta1",
	Resource: "inferenceservices",
}

// InferenceServiceConfig configures an InferenceService deployment
type InferenceServiceConfig struct {
	Name           string
	Namespace      string
	ModelName      string
	StorageURI     string            // S3 path to model files (optional for HF models)
	HFModelID      string            // HuggingFace model ID (e.g., "mistralai/Mistral-7B-Instruct-v0.3")
	Runtime        string            // ClusterServingRuntime name (e.g., "vllm-runtime")
	RuntimeVersion string
	GPUCount       int
	MemoryGB       int
	MinReplicas    int
	MaxReplicas    int
	Environment    string
	Labels         map[string]string
	Annotations    map[string]string
	EnvVars        map[string]string // Additional environment variables
}

// InferenceServiceStatus represents the status of an InferenceService
type InferenceServiceStatus struct {
	Name           string
	Namespace      string
	Ready          bool
	URL            string
	Conditions     []Condition
	Replicas       int32
	ReadyReplicas  int32
	LatestRevision string
}

// Condition represents a Kubernetes condition
type Condition struct {
	Type    string
	Status  string
	Reason  string
	Message string
}

// CreateInferenceService creates a new InferenceService
func (c *Client) CreateInferenceService(ctx context.Context, cfg InferenceServiceConfig) error {
	dynamicClient, err := dynamic.NewForConfig(c.config)
	if err != nil {
		return fmt.Errorf("create dynamic client: %w", err)
	}

	namespace := cfg.Namespace
	if namespace == "" {
		namespace = c.namespace
	}

	// Build InferenceService manifest
	isvc := buildInferenceServiceManifest(cfg)

	// Create the resource
	_, err = dynamicClient.Resource(InferenceServiceGVR).Namespace(namespace).Create(
		ctx,
		isvc,
		metav1.CreateOptions{},
	)
	if err != nil {
		return fmt.Errorf("create inferenceservice: %w", err)
	}

	return nil
}

// GetInferenceService gets an InferenceService by name
func (c *Client) GetInferenceService(ctx context.Context, name, namespace string) (*InferenceServiceStatus, error) {
	dynamicClient, err := dynamic.NewForConfig(c.config)
	if err != nil {
		return nil, fmt.Errorf("create dynamic client: %w", err)
	}

	if namespace == "" {
		namespace = c.namespace
	}

	result, err := dynamicClient.Resource(InferenceServiceGVR).Namespace(namespace).Get(
		ctx,
		name,
		metav1.GetOptions{},
	)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("inferenceservice %s not found", name)
		}
		return nil, fmt.Errorf("get inferenceservice: %w", err)
	}

	status, err := parseInferenceServiceStatus(result)
	if err != nil {
		return nil, err
	}

	// Get pod counts for replica status
	pods, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("serving.kserve.io/inferenceservice=%s", name),
	})
	if err == nil {
		status.Replicas = int32(len(pods.Items))
		for _, pod := range pods.Items {
			if isPodReady(&pod) {
				status.ReadyReplicas++
			}
		}
	}

	return status, nil
}

// DeleteInferenceService deletes an InferenceService
func (c *Client) DeleteInferenceService(ctx context.Context, name, namespace string) error {
	dynamicClient, err := dynamic.NewForConfig(c.config)
	if err != nil {
		return fmt.Errorf("create dynamic client: %w", err)
	}

	if namespace == "" {
		namespace = c.namespace
	}

	err = dynamicClient.Resource(InferenceServiceGVR).Namespace(namespace).Delete(
		ctx,
		name,
		metav1.DeleteOptions{},
	)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("delete inferenceservice: %w", err)
	}

	return nil
}

// ListInferenceServices lists InferenceServices in a namespace
func (c *Client) ListInferenceServices(ctx context.Context, namespace string, labelSelector string) ([]InferenceServiceStatus, error) {
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

	result, err := dynamicClient.Resource(InferenceServiceGVR).Namespace(namespace).List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("list inferenceservices: %w", err)
	}

	var services []InferenceServiceStatus
	for _, item := range result.Items {
		status, err := parseInferenceServiceStatus(&item)
		if err != nil {
			continue // Skip items that can't be parsed
		}
		services = append(services, *status)
	}

	return services, nil
}

func buildInferenceServiceManifest(cfg InferenceServiceConfig) *unstructured.Unstructured {
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
	resources := map[string]interface{}{
		"limits": map[string]interface{}{
			"memory": fmt.Sprintf("%dGi", cfg.MemoryGB),
		},
		"requests": map[string]interface{}{
			"memory": fmt.Sprintf("%dGi", cfg.MemoryGB/2),
		},
	}
	if cfg.GPUCount > 0 {
		resources["limits"].(map[string]interface{})["nvidia.com/gpu"] = cfg.GPUCount
		resources["requests"].(map[string]interface{})["nvidia.com/gpu"] = cfg.GPUCount
	}

	// Build model spec
	modelSpec := map[string]interface{}{
		"modelFormat": map[string]interface{}{
			"name": "vllm",
		},
		"resources": resources,
	}

	// For HuggingFace models, we pass model ID via env var (vLLM downloads directly)
	// For S3 models, we use storageUri (storage initializer downloads)
	if cfg.StorageURI != "" && !strings.HasPrefix(cfg.StorageURI, "hf://") {
		modelSpec["storageUri"] = cfg.StorageURI
	}

	// Add explicit runtime if specified
	if cfg.Runtime != "" {
		modelSpec["runtime"] = cfg.Runtime
	}

	// Build container env vars for HuggingFace model
	var containerEnvVars []interface{}
	if cfg.HFModelID != "" {
		containerEnvVars = append(containerEnvVars, map[string]interface{}{
			"name":  "VLLM_MODEL_NAME",
			"value": cfg.HFModelID,
		})
	}
	// Add any additional env vars
	for k, v := range cfg.EnvVars {
		containerEnvVars = append(containerEnvVars, map[string]interface{}{
			"name":  k,
			"value": v,
		})
	}
	if len(containerEnvVars) > 0 {
		modelSpec["env"] = containerEnvVars
	}

	isvc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "serving.kserve.io/v1beta1",
			"kind":       "InferenceService",
			"metadata": map[string]interface{}{
				"name":        cfg.Name,
				"namespace":   cfg.Namespace,
				"labels":      labels,
				"annotations": annotations,
			},
			"spec": map[string]interface{}{
				"predictor": map[string]interface{}{
					"model":       modelSpec,
					"minReplicas": cfg.MinReplicas,
					"maxReplicas": cfg.MaxReplicas,
				},
			},
		},
	}

	return isvc
}

func parseInferenceServiceStatus(obj *unstructured.Unstructured) (*InferenceServiceStatus, error) {
	status := &InferenceServiceStatus{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}

	// Get URL
	if url, ok, _ := unstructured.NestedString(obj.Object, "status", "url"); ok {
		status.URL = url
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

				// Check if Ready
				if condition.Type == "Ready" && condition.Status == "True" {
					status.Ready = true
				}
			}
		}
	}

	return status, nil
}

// GetPodStatus gets the status of pods for an InferenceService
func (c *Client) GetPodStatus(ctx context.Context, name, namespace string) ([]PodStatus, error) {
	if namespace == "" {
		namespace = c.namespace
	}

	// List pods with matching labels
	pods, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("serving.kserve.io/inferenceservice=%s", name),
	})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}

	var statuses []PodStatus
	for _, pod := range pods.Items {
		status := PodStatus{
			Name:      pod.Name,
			Phase:     string(pod.Status.Phase),
			Ready:     isPodReady(&pod),
			Restarts:  getPodRestarts(&pod),
			Age:       time.Since(pod.CreationTimestamp.Time),
		}
		statuses = append(statuses, status)
	}

	return statuses, nil
}

// PodStatus represents the status of a pod
type PodStatus struct {
	Name     string
	Phase    string
	Ready    bool
	Restarts int32
	Age      time.Duration
}

func isPodReady(pod *corev1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func getPodRestarts(pod *corev1.Pod) int32 {
	var restarts int32
	for _, cs := range pod.Status.ContainerStatuses {
		restarts += cs.RestartCount
	}
	return restarts
}

// ScaleInferenceService scales an InferenceService to the specified replica count
func (c *Client) ScaleInferenceService(ctx context.Context, name, namespace string, replicas int) error {
	dynamicClient, err := dynamic.NewForConfig(c.config)
	if err != nil {
		return fmt.Errorf("create dynamic client: %w", err)
	}

	if namespace == "" {
		namespace = c.namespace
	}

	// Get current InferenceService
	result, err := dynamicClient.Resource(InferenceServiceGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get inferenceservice: %w", err)
	}

	// Update minReplicas and maxReplicas
	if err := unstructured.SetNestedField(result.Object, int64(replicas), "spec", "predictor", "minReplicas"); err != nil {
		return fmt.Errorf("set minReplicas: %w", err)
	}
	if err := unstructured.SetNestedField(result.Object, int64(replicas), "spec", "predictor", "maxReplicas"); err != nil {
		return fmt.Errorf("set maxReplicas: %w", err)
	}

	// Update the resource
	_, err = dynamicClient.Resource(InferenceServiceGVR).Namespace(namespace).Update(ctx, result, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("update inferenceservice: %w", err)
	}

	return nil
}

// RestartInferenceService performs a rolling restart by updating an annotation
func (c *Client) RestartInferenceService(ctx context.Context, name, namespace string) error {
	dynamicClient, err := dynamic.NewForConfig(c.config)
	if err != nil {
		return fmt.Errorf("create dynamic client: %w", err)
	}

	if namespace == "" {
		namespace = c.namespace
	}

	// Get current InferenceService
	result, err := dynamicClient.Resource(InferenceServiceGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get inferenceservice: %w", err)
	}

	// Update restart annotation to trigger rolling restart
	annotations, _, _ := unstructured.NestedStringMap(result.Object, "metadata", "annotations")
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations["ai-aas.io/restarted-at"] = time.Now().Format(time.RFC3339)

	annotationsInterface := make(map[string]interface{})
	for k, v := range annotations {
		annotationsInterface[k] = v
	}
	if err := unstructured.SetNestedMap(result.Object, annotationsInterface, "metadata", "annotations"); err != nil {
		return fmt.Errorf("set annotations: %w", err)
	}

	// Also update pod template annotation to force pod recreation
	podAnnotations, _, _ := unstructured.NestedStringMap(result.Object, "spec", "predictor", "model", "annotations")
	if podAnnotations == nil {
		podAnnotations = make(map[string]string)
	}
	podAnnotations["ai-aas.io/restarted-at"] = time.Now().Format(time.RFC3339)

	podAnnotationsInterface := make(map[string]interface{})
	for k, v := range podAnnotations {
		podAnnotationsInterface[k] = v
	}
	// Set on predictor level for KServe
	if err := unstructured.SetNestedField(result.Object, podAnnotationsInterface, "spec", "predictor", "annotations"); err != nil {
		// Try without setting if path doesn't exist
		_ = err
	}

	// Update the resource
	_, err = dynamicClient.Resource(InferenceServiceGVR).Namespace(namespace).Update(ctx, result, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("update inferenceservice: %w", err)
	}

	return nil
}

