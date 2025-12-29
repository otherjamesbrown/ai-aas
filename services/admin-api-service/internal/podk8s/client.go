// Package podk8s provides a Kubernetes client for pod health operations.
package podk8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// PodClient wraps the Kubernetes client for pod operations
type PodClient struct {
	clientset *kubernetes.Clientset
}

// NewPodClient creates a new Kubernetes client for pod health queries
// Tries in-cluster config first, then falls back to kubeconfig
func NewPodClient() (*PodClient, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fall back to kubeconfig
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("failed to get home directory: %w", err)
			}
			kubeconfig = filepath.Join(home, ".kube", "config")
		}

		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return &PodClient{clientset: clientset}, nil
}

// PodHealthInfo contains health information about a pod
type PodHealthInfo struct {
	Name             string
	Namespace        string
	ModelName        string
	InferenceService string
	Phase            string
	Ready            bool
	Node             string
	RestartCount     int32
	LastRestartTime  *time.Time
	LastTermination  *TerminationInfo
	Containers       []ContainerInfo
	Conditions       []ConditionInfo
	AgeSeconds       int64
	CreatedAt        time.Time
}

// TerminationInfo contains information about the last termination
type TerminationInfo struct {
	Reason     string
	ExitCode   int32
	Message    string
	StartedAt  *time.Time
	FinishedAt *time.Time
}

// ContainerInfo contains container health information
type ContainerInfo struct {
	Name         string
	State        string
	Ready        bool
	RestartCount int32
	Resources    ResourceInfo
}

// ResourceInfo contains resource requests and limits
type ResourceInfo struct {
	RequestsCPU    string
	RequestsMemory string
	RequestsGPU    string
	LimitsCPU      string
	LimitsMemory   string
	LimitsGPU      string
}

// ConditionInfo contains pod condition information
type ConditionInfo struct {
	Type    string
	Status  string
	Reason  string
	Message string
}

// ListModelPodHealthOptions contains options for listing pod health
type ListModelPodHealthOptions struct {
	Namespace      string
	ModelName      string
	Environment    string
	IncludeHealthy bool
}

// ListModelPodHealth retrieves health information for model pods
func (c *PodClient) ListModelPodHealth(ctx context.Context, opts ListModelPodHealthOptions) ([]PodHealthInfo, error) {
	namespace := opts.Namespace
	if namespace == "" {
		namespace = "system"
	}

	// List all pods in the namespace
	podList, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	var result []PodHealthInfo

	for _, pod := range podList.Items {
		// Filter model pods by owner references (InferenceService)
		if !isModelPod(&pod) {
			continue
		}

		// Extract model name and InferenceService name from labels/owner refs
		modelName, inferenceServiceName := extractModelInfo(&pod)

		// Apply filters
		if opts.ModelName != "" && modelName != opts.ModelName {
			continue
		}

		if opts.Environment != "" {
			if envLabel, ok := pod.Labels["environment"]; !ok || envLabel != opts.Environment {
				continue
			}
		}

		// Calculate if pod is ready
		ready := isPodReady(&pod)

		// Filter by health if requested
		if !opts.IncludeHealthy && ready && pod.Status.Phase == corev1.PodRunning {
			continue
		}

		// Extract health information
		info := extractPodHealthInfo(&pod, modelName, inferenceServiceName)
		result = append(result, info)
	}

	return result, nil
}

// isModelPod checks if a pod belongs to a model deployment (InferenceService)
func isModelPod(pod *corev1.Pod) bool {
	// Check for serving.kserve.io or serving.kubeflow.org labels
	if _, ok := pod.Labels["serving.kserve.io/inferenceservice"]; ok {
		return true
	}
	if _, ok := pod.Labels["serving.kubeflow.org/inferenceservice"]; ok {
		return true
	}

	// Check owner references for InferenceService
	for _, owner := range pod.OwnerReferences {
		if owner.Kind == "InferenceService" {
			return true
		}
		// Could also be owned by Deployment which is owned by InferenceService
		// For now, rely on labels which KServe sets
	}

	return false
}

// extractModelInfo extracts model name and InferenceService name from pod
func extractModelInfo(pod *corev1.Pod) (modelName, inferenceServiceName string) {
	// Try KServe label
	if isvcName, ok := pod.Labels["serving.kserve.io/inferenceservice"]; ok {
		inferenceServiceName = isvcName
		// Model name might be embedded in the InferenceService name
		// Typically: <model-name>-<environment> or just <model-name>
		modelName = extractModelNameFromISVC(isvcName)
		return
	}

	// Try Kubeflow label
	if isvcName, ok := pod.Labels["serving.kubeflow.org/inferenceservice"]; ok {
		inferenceServiceName = isvcName
		modelName = extractModelNameFromISVC(isvcName)
		return
	}

	// Try component label for model name
	if comp, ok := pod.Labels["component"]; ok {
		modelName = comp
	}

	// Try app label
	if app, ok := pod.Labels["app"]; ok {
		if modelName == "" {
			modelName = app
		}
		if inferenceServiceName == "" {
			inferenceServiceName = app
		}
	}

	return
}

// extractModelNameFromISVC extracts the base model name from InferenceService name
// InferenceService names typically follow: <model-name>-<environment>
func extractModelNameFromISVC(isvcName string) string {
	// Try to extract model name by removing common suffixes
	// This is heuristic - in practice, might need more sophisticated logic
	// For now, return the full ISVC name as the model name
	return isvcName
}

// isPodReady checks if a pod is ready
func isPodReady(pod *corev1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}

// extractPodHealthInfo extracts detailed health info from a pod
func extractPodHealthInfo(pod *corev1.Pod, modelName, inferenceServiceName string) PodHealthInfo {
	info := PodHealthInfo{
		Name:             pod.Name,
		Namespace:        pod.Namespace,
		ModelName:        modelName,
		InferenceService: inferenceServiceName,
		Phase:            string(pod.Status.Phase),
		Ready:            isPodReady(pod),
		Node:             pod.Spec.NodeName,
		CreatedAt:        pod.CreationTimestamp.Time,
		AgeSeconds:       int64(time.Since(pod.CreationTimestamp.Time).Seconds()),
	}

	// Extract container information
	var totalRestarts int32
	var lastRestartTime *time.Time

	for _, container := range pod.Spec.Containers {
		var containerStatus *corev1.ContainerStatus
		for j := range pod.Status.ContainerStatuses {
			if pod.Status.ContainerStatuses[j].Name == container.Name {
				containerStatus = &pod.Status.ContainerStatuses[j]
				break
			}
		}

		containerInfo := ContainerInfo{
			Name: container.Name,
			Resources: ResourceInfo{
				RequestsCPU:    getResourceQuantity(container.Resources.Requests, corev1.ResourceCPU),
				RequestsMemory: getResourceQuantity(container.Resources.Requests, corev1.ResourceMemory),
				RequestsGPU:    getResourceQuantity(container.Resources.Requests, "nvidia.com/gpu"),
				LimitsCPU:      getResourceQuantity(container.Resources.Limits, corev1.ResourceCPU),
				LimitsMemory:   getResourceQuantity(container.Resources.Limits, corev1.ResourceMemory),
				LimitsGPU:      getResourceQuantity(container.Resources.Limits, "nvidia.com/gpu"),
			},
		}

		if containerStatus != nil {
			containerInfo.Ready = containerStatus.Ready
			containerInfo.RestartCount = containerStatus.RestartCount
			totalRestarts += containerStatus.RestartCount

			// Determine state
			if containerStatus.State.Running != nil {
				containerInfo.State = "running"
			} else if containerStatus.State.Waiting != nil {
				containerInfo.State = "waiting"
			} else if containerStatus.State.Terminated != nil {
				containerInfo.State = "terminated"
			}

			// Extract last termination info
			if containerStatus.LastTerminationState.Terminated != nil {
				term := containerStatus.LastTerminationState.Terminated
				termInfo := &TerminationInfo{
					Reason:   term.Reason,
					ExitCode: term.ExitCode,
					Message:  term.Message,
				}
				if !term.StartedAt.IsZero() {
					termInfo.StartedAt = &term.StartedAt.Time
				}
				if !term.FinishedAt.IsZero() {
					termInfo.FinishedAt = &term.FinishedAt.Time
					// Track most recent restart time
					if lastRestartTime == nil || term.FinishedAt.Time.After(*lastRestartTime) {
						lastRestartTime = &term.FinishedAt.Time
					}
				}
				// Track most recent termination across all containers
				if info.LastTermination == nil {
					info.LastTermination = termInfo
				} else if termInfo.FinishedAt != nil {
					if info.LastTermination.FinishedAt == nil || termInfo.FinishedAt.After(*info.LastTermination.FinishedAt) {
						info.LastTermination = termInfo
					}
				}
			}
		}

		info.Containers = append(info.Containers, containerInfo)
	}

	info.RestartCount = totalRestarts
	info.LastRestartTime = lastRestartTime

	// Extract conditions
	for _, cond := range pod.Status.Conditions {
		info.Conditions = append(info.Conditions, ConditionInfo{
			Type:    string(cond.Type),
			Status:  string(cond.Status),
			Reason:  cond.Reason,
			Message: cond.Message,
		})
	}

	return info
}

// getResourceQuantity gets a resource quantity as a string
func getResourceQuantity(resources corev1.ResourceList, resourceName corev1.ResourceName) string {
	if qty, ok := resources[resourceName]; ok {
		return qty.String()
	}
	return ""
}

// labelSelectorFromOptions creates a label selector from options
func labelSelectorFromOptions(opts ListModelPodHealthOptions) string {
	selector := labels.NewSelector()

	// This is a helper for future use if we want to use label selectors
	// Currently unused but kept for reference

	return selector.String()
}
