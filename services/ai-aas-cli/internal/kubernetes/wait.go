// Package kubernetes provides Kubernetes client operations for model deployment.
package kubernetes

import (
	"context"
	"fmt"
	"time"
)

// WaitOptions configures wait behavior
type WaitOptions struct {
	Timeout      time.Duration
	PollInterval time.Duration
	OnProgress   func(status *InferenceServiceStatus)
}

// DefaultWaitOptions returns default wait options
func DefaultWaitOptions() WaitOptions {
	return WaitOptions{
		Timeout:      10 * time.Minute,
		PollInterval: 5 * time.Second,
	}
}

// WaitForReady waits for an InferenceService to become ready
func (c *Client) WaitForReady(ctx context.Context, name, namespace string, opts WaitOptions) error {
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
			return fmt.Errorf("timeout waiting for inferenceservice %s to be ready", name)
		case <-ticker.C:
			status, err := c.GetInferenceService(ctx, name, namespace)
			if err != nil {
				// Keep waiting if not found yet
				continue
			}

			if opts.OnProgress != nil {
				opts.OnProgress(status)
			}

			if status.Ready {
				return nil
			}

			// Check for failure conditions
			for _, cond := range status.Conditions {
				if cond.Type == "Ready" && cond.Status == "False" {
					if cond.Reason == "RevisionFailed" || cond.Reason == "ContainerCreating" {
						// Still progressing
						continue
					}
					if cond.Reason == "FailedCreate" || cond.Reason == "ServiceUnavailable" {
						return fmt.Errorf("deployment failed: %s - %s", cond.Reason, cond.Message)
					}
				}
			}
		}
	}
}

// WaitForDelete waits for an InferenceService to be deleted
func (c *Client) WaitForDelete(ctx context.Context, name, namespace string, timeout time.Duration) error {
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for inferenceservice %s to be deleted", name)
		case <-ticker.C:
			_, err := c.GetInferenceService(ctx, name, namespace)
			if err != nil {
				// Resource not found - deleted successfully
				return nil
			}
		}
	}
}

// WaitForPodReady waits for at least one pod to be ready
func (c *Client) WaitForPodReady(ctx context.Context, name, namespace string, opts WaitOptions) error {
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
			return fmt.Errorf("timeout waiting for pods to be ready")
		case <-ticker.C:
			pods, err := c.GetPodStatus(ctx, name, namespace)
			if err != nil {
				continue
			}

			for _, pod := range pods {
				if pod.Ready {
					return nil
				}
			}
		}
	}
}

// CheckHealth checks if an InferenceService is healthy
func (c *Client) CheckHealth(ctx context.Context, name, namespace string) (*HealthStatus, error) {
	status, err := c.GetInferenceService(ctx, name, namespace)
	if err != nil {
		return &HealthStatus{
			Healthy: false,
			Error:   err.Error(),
		}, nil
	}

	health := &HealthStatus{
		Healthy:       status.Ready,
		URL:           status.URL,
		Replicas:      status.Replicas,
		ReadyReplicas: status.ReadyReplicas,
	}

	// Check pod status
	pods, err := c.GetPodStatus(ctx, name, namespace)
	if err == nil {
		health.PodCount = len(pods)
		for _, pod := range pods {
			if pod.Ready {
				health.ReadyPods++
			}
			health.TotalRestarts += int(pod.Restarts)
		}
	}

	// Determine health message
	if health.Healthy {
		health.Message = "All systems operational"
	} else {
		for _, cond := range status.Conditions {
			if cond.Type == "Ready" && cond.Status == "False" {
				health.Message = cond.Message
				health.Error = cond.Reason
				break
			}
		}
	}

	return health, nil
}

// HealthStatus represents the health of an InferenceService
type HealthStatus struct {
	Healthy       bool
	URL           string
	Message       string
	Error         string
	Replicas      int32
	ReadyReplicas int32
	PodCount      int
	ReadyPods     int
	TotalRestarts int
}
