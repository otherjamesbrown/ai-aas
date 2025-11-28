// Package status provides platform health check commands.
package status

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/output"
)

// ServiceStatus represents the health status of a service
type ServiceStatus struct {
	Name       string `json:"name"`
	URL        string `json:"url"`
	Status     string `json:"status"`
	StatusCode int    `json:"status_code,omitempty"`
	Latency    string `json:"latency"`
	Error      string `json:"error,omitempty"`
	Details    string `json:"details,omitempty"`
}

// PlatformStatus represents the overall platform health
type PlatformStatus struct {
	Environment string          `json:"environment"`
	Timestamp   string          `json:"timestamp"`
	Healthy     bool            `json:"healthy"`
	Services    []ServiceStatus `json:"services"`
}

// NewCommand creates the status command
func NewCommand() *cobra.Command {
	var verbose bool
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check health of all platform services",
		Long: `Check the health and availability of all platform services.

This command performs health checks on:
  - Admin API Service (authentication, model management)
  - User-Org Service (user and organization management)
  - Inference API (model inference endpoints)
  - API Router (request routing)

The checks use the configured base domain to construct service URLs.

Examples:
  ai-aas-cli status              # Check all services
  ai-aas-cli status --verbose    # Show detailed information
  ai-aas-cli status --json       # Output as JSON`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(cmd.Context(), verbose, jsonOutput)
		},
	}

	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "show detailed information")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")

	return cmd
}

func runStatus(ctx context.Context, verbose, jsonOutput bool) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Extract base domain from API endpoint
	baseDomain, err := extractBaseDomain(cfg.APIEndpoint)
	if err != nil {
		return fmt.Errorf("parse endpoint: %w", err)
	}

	if !jsonOutput {
		fmt.Println("╔════════════════════════════════════════════════════════════════╗")
		fmt.Println("║                    Platform Health Check                       ║")
		fmt.Println("╚════════════════════════════════════════════════════════════════╝")
		fmt.Println()
		fmt.Printf("Environment:  %s\n", cfg.Environment)
		fmt.Printf("Base Domain:  %s\n", baseDomain)
		fmt.Println()
	}

	// Define services to check
	services := []struct {
		name     string
		url      string
		endpoint string
		authReq  bool
	}{
		// Core Platform Services
		{
			name:     "Admin API",
			url:      fmt.Sprintf("https://admin-api.%s", baseDomain),
			endpoint: "/healthz",
			authReq:  false,
		},
		{
			name:     "API Router",
			url:      fmt.Sprintf("https://api.%s", baseDomain),
			endpoint: "/v1/status/healthz",
			authReq:  false,
		},
		{
			name:     "User-Org Service",
			url:      fmt.Sprintf("https://user-org.%s", baseDomain),
			endpoint: "/healthz",
			authReq:  false,
		},
		{
			name:     "Inference API",
			url:      fmt.Sprintf("https://api.%s", baseDomain),
			endpoint: "/v1/models",
			authReq:  true,
		},
		// Frontend
		{
			name:     "Web Portal",
			url:      fmt.Sprintf("https://portal.%s", baseDomain),
			endpoint: "/",
			authReq:  false,
		},
		// Analytics & Observability
		{
			name:     "Analytics",
			url:      fmt.Sprintf("https://analytics.%s", baseDomain),
			endpoint: "/analytics/v1/status/healthz",
			authReq:  false,
		},
		{
			name:     "Grafana",
			url:      fmt.Sprintf("https://grafana.%s", baseDomain),
			endpoint: "/api/health",
			authReq:  false,
		},
		{
			name:     "ArgoCD",
			url:      fmt.Sprintf("https://argocd.%s", baseDomain),
			endpoint: "/healthz",
			authReq:  false,
		},
	}

	// Create HTTP client with TLS skip for self-signed certs
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		},
	}

	// Check services concurrently
	var wg sync.WaitGroup
	results := make([]ServiceStatus, len(services))

	for i, svc := range services {
		wg.Add(1)
		go func(idx int, s struct {
			name     string
			url      string
			endpoint string
			authReq  bool
		}) {
			defer wg.Done()
			results[idx] = checkService(ctx, client, s.name, s.url, s.endpoint, cfg.APIKey, s.authReq)
		}(i, svc)
	}

	wg.Wait()

	// Determine overall health
	allHealthy := true
	for _, r := range results {
		if r.Status != "healthy" && r.Status != "auth_required" {
			allHealthy = false
			break
		}
	}

	status := PlatformStatus{
		Environment: cfg.Environment,
		Timestamp:   time.Now().Format(time.RFC3339),
		Healthy:     allHealthy,
		Services:    results,
	}

	if jsonOutput {
		return output.PrintJSON(status)
	}

	// Table output
	headers := []string{"SERVICE", "STATUS", "LATENCY", "URL"}
	if verbose {
		headers = append(headers, "DETAILS")
	}

	var rows [][]string
	for _, r := range results {
		statusIcon := getStatusIcon(r.Status)
		row := []string{r.Name, statusIcon + " " + r.Status, r.Latency, r.URL}
		if verbose {
			detail := r.Details
			if r.Error != "" {
				detail = r.Error
			}
			row = append(row, detail)
		}
		rows = append(rows, row)
	}

	output.PrintTable(headers, rows)

	fmt.Println()
	if allHealthy {
		fmt.Println("✓ All services are healthy")
	} else {
		fmt.Println("✗ Some services are unhealthy")
		fmt.Println()
		fmt.Println("Troubleshooting:")
		fmt.Println("  - Check if services are deployed and running")
		fmt.Println("  - Verify ingress configuration")
		fmt.Println("  - Check ArgoCD for sync status")
	}

	return nil
}

func checkService(ctx context.Context, client *http.Client, name, baseURL, endpoint, apiKey string, authRequired bool) ServiceStatus {
	fullURL := baseURL + endpoint
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return ServiceStatus{
			Name:    name,
			URL:     fullURL,
			Status:  "error",
			Latency: "-",
			Error:   err.Error(),
		}
	}

	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	resp, err := client.Do(req)
	latency := time.Since(start)

	if err != nil {
		status := "unreachable"
		errMsg := err.Error()
		if strings.Contains(errMsg, "no such host") {
			status = "dns_error"
		} else if strings.Contains(errMsg, "connection refused") {
			status = "connection_refused"
		} else if strings.Contains(errMsg, "timeout") {
			status = "timeout"
		}
		return ServiceStatus{
			Name:    name,
			URL:     fullURL,
			Status:  status,
			Latency: "-",
			Error:   errMsg,
		}
	}
	defer resp.Body.Close()

	result := ServiceStatus{
		Name:       name,
		URL:        fullURL,
		StatusCode: resp.StatusCode,
		Latency:    latency.Round(time.Millisecond).String(),
	}

	// Read response body for details
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		result.Status = "healthy"
		result.Details = fmt.Sprintf("HTTP %d", resp.StatusCode)
	case resp.StatusCode == 401 || resp.StatusCode == 403:
		// Auth required but service is reachable
		if authRequired {
			result.Status = "healthy"
			result.Details = "Reachable (auth required)"
		} else {
			result.Status = "auth_required"
			result.Details = "Unexpected auth requirement"
		}
	case resp.StatusCode == 404:
		// Endpoint not found but service may be running
		result.Status = "endpoint_not_found"
		result.Details = fmt.Sprintf("HTTP %d - endpoint may not exist", resp.StatusCode)
	case resp.StatusCode >= 500:
		result.Status = "server_error"
		result.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		// Try to extract error message
		if len(body) > 0 {
			var errResp map[string]interface{}
			if json.Unmarshal(body, &errResp) == nil {
				if msg, ok := errResp["error"].(string); ok {
					result.Details = msg
				}
			}
		}
	default:
		result.Status = "unknown"
		result.Details = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	return result
}

func extractBaseDomain(endpoint string) (string, error) {
	if endpoint == "" {
		return "", fmt.Errorf("API endpoint not configured")
	}

	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}

	host := parsed.Hostname()
	// Remove "api." prefix if present
	host = strings.TrimPrefix(host, "api.")
	return host, nil
}

func getStatusIcon(status string) string {
	switch status {
	case "healthy":
		return "✓"
	case "auth_required":
		return "⚡"
	case "unreachable", "dns_error", "connection_refused", "timeout":
		return "✗"
	case "endpoint_not_found":
		return "?"
	case "server_error":
		return "!"
	default:
		return "?"
	}
}
