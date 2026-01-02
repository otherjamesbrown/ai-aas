package gitops

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// ArgoCDClient provides operations for ArgoCD.
type ArgoCDClient struct {
	server    string
	authToken string
}

// ArgoCDAppStatus represents the status of an ArgoCD application.
type ArgoCDAppStatus struct {
	Name       string
	SyncStatus string // Synced, OutOfSync, Unknown
	Health     string // Healthy, Degraded, Progressing, Missing, Unknown
	Message    string
}

// NewArgoCDClient creates a new ArgoCD client.
// If server is empty, it will use the argocd CLI's default server.
func NewArgoCDClient(server, authToken string) *ArgoCDClient {
	return &ArgoCDClient{
		server:    server,
		authToken: authToken,
	}
}

// SyncApp triggers a sync for the specified ArgoCD application.
func (c *ArgoCDClient) SyncApp(ctx context.Context, appName string) error {
	args := []string{"app", "sync", appName}
	if c.server != "" {
		args = append(args, "--server", c.server)
	}
	if c.authToken != "" {
		args = append(args, "--auth-token", c.authToken)
	}

	cmd := exec.CommandContext(ctx, "argocd", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sync app %s: %s: %w", appName, string(output), err)
	}
	return nil
}

// GetAppStatus gets the status of an ArgoCD application.
func (c *ArgoCDClient) GetAppStatus(ctx context.Context, appName string) (*ArgoCDAppStatus, error) {
	args := []string{"app", "get", appName, "-o", "json"}
	if c.server != "" {
		args = append(args, "--server", c.server)
	}
	if c.authToken != "" {
		args = append(args, "--auth-token", c.authToken)
	}

	cmd := exec.CommandContext(ctx, "argocd", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("get app %s: %w", appName, err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	status := &ArgoCDAppStatus{
		Name: appName,
	}

	// Extract status
	if statusMap, ok := result["status"].(map[string]interface{}); ok {
		if sync, ok := statusMap["sync"].(map[string]interface{}); ok {
			if s, ok := sync["status"].(string); ok {
				status.SyncStatus = s
			}
		}
		if health, ok := statusMap["health"].(map[string]interface{}); ok {
			if s, ok := health["status"].(string); ok {
				status.Health = s
			}
			if m, ok := health["message"].(string); ok {
				status.Message = m
			}
		}
	}

	return status, nil
}

// WaitForSync waits for an ArgoCD application to be synced.
func (c *ArgoCDClient) WaitForSync(ctx context.Context, appName string) error {
	args := []string{"app", "wait", appName, "--sync"}
	if c.server != "" {
		args = append(args, "--server", c.server)
	}
	if c.authToken != "" {
		args = append(args, "--auth-token", c.authToken)
	}

	cmd := exec.CommandContext(ctx, "argocd", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("wait for sync %s: %s: %w", appName, string(output), err)
	}
	return nil
}

// AppNameForEnvironment returns the ArgoCD app name for the ai-aas-config environment.
// This assumes an ArgoCD Application exists that manages the ai-aas-config repo.
func AppNameForEnvironment(environment string) string {
	// The ArgoCD Application that manages model deployments for each environment
	switch environment {
	case "development":
		return "ai-aas-models-development"
	case "staging":
		return "ai-aas-models-staging"
	case "production":
		return "ai-aas-models-production"
	default:
		return fmt.Sprintf("ai-aas-models-%s", strings.ToLower(environment))
	}
}
