// Package service provides business logic for pod health operations.
package service

import (
	"context"
	"fmt"

	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/handlers/pods"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/kubernetes"
	"go.uber.org/zap"
)

// PodsService provides pod health operations
type PodsService struct {
	k8sClient *kubernetes.Client
	logger    *zap.Logger
}

// NewPodsService creates a new pods service
func NewPodsService(k8sClient *kubernetes.Client, logger *zap.Logger) *PodsService {
	return &PodsService{
		k8sClient: k8sClient,
		logger:    logger,
	}
}

// GetPodHealth retrieves pod health information
func (s *PodsService) GetPodHealth(ctx context.Context, opts pods.ListOptions) (*pods.PodHealthResponse, error) {
	// Convert handler options to kubernetes client options
	k8sOpts := kubernetes.ListModelPodHealthOptions{
		Namespace:      opts.Namespace,
		ModelName:      opts.ModelName,
		Environment:    opts.Environment,
		IncludeHealthy: opts.IncludeHealthy,
	}

	// Get pod health from Kubernetes
	podInfos, err := s.k8sClient.ListModelPodHealth(ctx, k8sOpts)
	if err != nil {
		s.logger.Error("failed to list pod health",
			zap.Error(err),
			zap.String("namespace", opts.Namespace),
			zap.String("model_name", opts.ModelName),
		)
		return nil, fmt.Errorf("failed to retrieve pod health: %w", err)
	}

	// Convert to response format
	response := &pods.PodHealthResponse{
		Pods: make([]pods.PodHealth, 0, len(podInfos)),
		Summary: pods.HealthSummary{
			TotalPods: len(podInfos),
		},
	}

	for _, info := range podInfos {
		podHealth := convertToPodHealth(info)
		response.Pods = append(response.Pods, podHealth)

		// Update summary
		if podHealth.Ready && podHealth.Phase == "Running" {
			response.Summary.HealthyPods++
		} else {
			response.Summary.UnhealthyPods++
		}

		response.Summary.TotalRestarts += int(podHealth.RestartCount)
		if podHealth.RestartCount > 0 {
			response.Summary.PodsWithRestarts++
		}
	}

	return response, nil
}

// convertToPodHealth converts kubernetes.PodHealthInfo to pods.PodHealth
func convertToPodHealth(info kubernetes.PodHealthInfo) pods.PodHealth {
	ph := pods.PodHealth{
		Name:             info.Name,
		Namespace:        info.Namespace,
		ModelName:        info.ModelName,
		InferenceService: info.InferenceService,
		Phase:            info.Phase,
		Ready:            info.Ready,
		Node:             info.Node,
		RestartCount:     info.RestartCount,
		LastRestartTime:  info.LastRestartTime,
		AgeSeconds:       info.AgeSeconds,
		CreatedAt:        info.CreatedAt,
		Containers:       make([]pods.ContainerHealth, 0, len(info.Containers)),
		Conditions:       make([]pods.PodCondition, 0, len(info.Conditions)),
	}

	// Convert termination info
	if info.LastTermination != nil {
		ph.LastTermination = &pods.TerminationInfo{
			Reason:     info.LastTermination.Reason,
			ExitCode:   info.LastTermination.ExitCode,
			Message:    info.LastTermination.Message,
			StartedAt:  info.LastTermination.StartedAt,
			FinishedAt: info.LastTermination.FinishedAt,
		}
	}

	// Convert containers
	for _, c := range info.Containers {
		ph.Containers = append(ph.Containers, pods.ContainerHealth{
			Name:         c.Name,
			State:        c.State,
			Ready:        c.Ready,
			RestartCount: c.RestartCount,
			Resources: pods.ContainerResources{
				Requests: pods.ResourceList{
					CPU:    c.Resources.RequestsCPU,
					Memory: c.Resources.RequestsMemory,
					GPU:    c.Resources.RequestsGPU,
				},
				Limits: pods.ResourceList{
					CPU:    c.Resources.LimitsCPU,
					Memory: c.Resources.LimitsMemory,
					GPU:    c.Resources.LimitsGPU,
				},
			},
		})
	}

	// Convert conditions
	for _, cond := range info.Conditions {
		ph.Conditions = append(ph.Conditions, pods.PodCondition{
			Type:    cond.Type,
			Status:  cond.Status,
			Reason:  cond.Reason,
			Message: cond.Message,
		})
	}

	return ph
}
