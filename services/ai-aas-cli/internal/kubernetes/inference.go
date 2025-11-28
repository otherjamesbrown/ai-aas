// Package kubernetes provides Kubernetes client operations for model deployment.
package kubernetes

import (
	"context"
	"fmt"
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
	StorageURI     string // S3 path to model files
	RuntimeVersion string
	GPUCount       int
	MemoryGB       int
	MinReplicas    int
	MaxReplicas    int
	Environment    string
	Labels         map[string]string
	Annotations    map[string]string
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

	return parseInferenceServiceStatus(result)
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
					"model": map[string]interface{}{
						"modelFormat": map[string]interface{}{
							"name": "vllm",
						},
						"storageUri": cfg.StorageURI,
						"resources":  resources,
					},
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

