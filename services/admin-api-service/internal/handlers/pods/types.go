// Package pods provides types for pod health API.
package pods

import (
	"errors"
	"time"
)

// ErrK8sUnavailable indicates the Kubernetes client is not available
var ErrK8sUnavailable = errors.New("kubernetes client unavailable")

// PodHealthResponse represents the response from GET /v1/pods/health
type PodHealthResponse struct {
	Pods    []PodHealth   `json:"pods"`
	Summary HealthSummary `json:"summary"`
}

// PodHealth represents the health information of a single pod
type PodHealth struct {
	Name             string            `json:"name"`
	Namespace        string            `json:"namespace"`
	ModelName        string            `json:"model_name"`
	InferenceService string            `json:"inferenceservice"`
	Phase            string            `json:"phase"`
	Ready            bool              `json:"ready"`
	Node             string            `json:"node"`
	RestartCount     int32             `json:"restart_count"`
	LastRestartTime  *time.Time        `json:"last_restart_time,omitempty"`
	LastTermination  *TerminationInfo  `json:"last_termination,omitempty"`
	Containers       []ContainerHealth `json:"containers"`
	Conditions       []PodCondition    `json:"conditions"`
	AgeSeconds       int64             `json:"age_seconds"`
	CreatedAt        time.Time         `json:"created_at"`
}

// TerminationInfo represents information about the last container termination
type TerminationInfo struct {
	Reason     string     `json:"reason"`
	ExitCode   int32      `json:"exit_code"`
	Message    string     `json:"message"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

// ContainerHealth represents the health information of a container
type ContainerHealth struct {
	Name         string             `json:"name"`
	State        string             `json:"state"`
	Ready        bool               `json:"ready"`
	RestartCount int32              `json:"restart_count"`
	Resources    ContainerResources `json:"resources"`
}

// ContainerResources represents resource requests and limits
type ContainerResources struct {
	Requests ResourceList `json:"requests"`
	Limits   ResourceList `json:"limits"`
}

// ResourceList represents a list of resources (CPU, memory, GPU)
type ResourceList struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
	GPU    string `json:"nvidia.com/gpu,omitempty"`
}

// PodCondition represents a pod condition
type PodCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

// HealthSummary provides aggregate statistics
type HealthSummary struct {
	TotalPods        int `json:"total_pods"`
	HealthyPods      int `json:"healthy_pods"`
	UnhealthyPods    int `json:"unhealthy_pods"`
	TotalRestarts    int `json:"total_restarts"`
	PodsWithRestarts int `json:"pods_with_restarts"`
}

// ListOptions represents query parameters for listing pod health
type ListOptions struct {
	Namespace      string
	ModelName      string
	Environment    string
	IncludeHealthy bool
}
