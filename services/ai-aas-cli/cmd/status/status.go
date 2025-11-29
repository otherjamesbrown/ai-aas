// Package status provides platform health check commands.
package status

import (
	"bytes"
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
			name:     "Model Registry",
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
	// Configure transport for better concurrent performance
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig:     &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
			MaxIdleConns:        20,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     30 * time.Second,
			DisableKeepAlives:   false,
			ForceAttemptHTTP2:   true,
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

	// Test actual inference backend connectivity (only if API key is configured)
	if cfg.APIKey != "" {
		inferenceResult := checkInferenceBackend(ctx, client, baseDomain, cfg.APIKey)
		results = append(results, inferenceResult)
	}

	// Determine overall health
	allHealthy := true
	hasSlow := false
	for _, r := range results {
		if r.Status == "slow" {
			hasSlow = true
		} else if r.Status != "healthy" && r.Status != "auth_required" {
			allHealthy = false
		}
	}

	status := PlatformStatus{
		Environment: cfg.Environment,
		Timestamp:   time.Now().Format(time.RFC3339),
		Healthy:     allHealthy && !hasSlow,
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
	if allHealthy && !hasSlow {
		fmt.Println(colorGreen + "✓ All services are healthy" + colorReset)
	} else if allHealthy && hasSlow {
		fmt.Println(colorYellow + "⚠ All services reachable but some are slow (>1s response)" + colorReset)
		fmt.Println()
		fmt.Println("Possible causes:")
		fmt.Println("  - High load on backend services")
		fmt.Println("  - Network latency or DNS resolution delays")
		fmt.Println("  - Cold start / model loading in progress")
	} else {
		fmt.Println(colorRed + "✗ Some services are unhealthy" + colorReset)
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
		// Check latency - if > 1 second, mark as slow
		if latency > time.Second {
			result.Status = "slow"
			result.Details = fmt.Sprintf("HTTP %d (response > 1s)", resp.StatusCode)
		} else {
			result.Status = "healthy"
			result.Details = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
	case resp.StatusCode == 401 || resp.StatusCode == 403:
		// Auth required but service is reachable
		if authRequired {
			if latency > time.Second {
				result.Status = "slow"
				result.Details = "Reachable but slow (auth required)"
			} else {
				result.Status = "healthy"
				result.Details = "Reachable (auth required)"
			}
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

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorCyan   = "\033[36m"
)

func getStatusIcon(status string) string {
	switch status {
	case "healthy":
		return colorGreen + "✓" + colorReset
	case "slow":
		return colorYellow + "⚠" + colorReset
	case "auth_required":
		return colorCyan + "⚡" + colorReset
	case "unreachable", "dns_error", "connection_refused", "timeout":
		return colorRed + "✗" + colorReset
	case "endpoint_not_found":
		return colorYellow + "?" + colorReset
	case "server_error", "backend_error":
		return colorRed + "!" + colorReset
	case "no_models":
		return colorYellow + "○" + colorReset
	default:
		return colorYellow + "?" + colorReset
	}
}

// checkInferenceBackend tests actual inference by making a minimal chat completion request
func checkInferenceBackend(ctx context.Context, client *http.Client, baseDomain, apiKey string) ServiceStatus {
	baseURL := fmt.Sprintf("https://api.%s", baseDomain)
	modelsURL := baseURL + "/v1/models"
	completionsURL := baseURL + "/v1/chat/completions"

	result := ServiceStatus{
		Name: "Inference Backend",
		URL:  completionsURL,
	}

	start := time.Now()

	// First, get available models
	modelsReq, err := http.NewRequestWithContext(ctx, "GET", modelsURL, nil)
	if err != nil {
		result.Status = "error"
		result.Latency = "-"
		result.Error = err.Error()
		return result
	}
	// Set both headers for compatibility - API router accepts either X-API-Key or Authorization Bearer
	modelsReq.Header.Set("X-API-Key", apiKey)
	modelsReq.Header.Set("Authorization", "Bearer "+apiKey)

	modelsResp, err := client.Do(modelsReq)
	if err != nil {
		result.Status = "unreachable"
		result.Latency = "-"
		result.Error = err.Error()
		return result
	}
	defer modelsResp.Body.Close()

	if modelsResp.StatusCode == 401 || modelsResp.StatusCode == 403 {
		result.Status = "auth_required"
		result.Latency = time.Since(start).Round(time.Millisecond).String()
		result.Details = "API key invalid or missing permissions"
		return result
	}

	modelsBody, err := io.ReadAll(modelsResp.Body)
	if err != nil {
		result.Status = "error"
		result.Latency = time.Since(start).Round(time.Millisecond).String()
		result.Error = fmt.Sprintf("failed to read models response: %v", err)
		return result
	}
	var modelsData struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(modelsBody, &modelsData); err != nil || len(modelsData.Data) == 0 {
		result.Status = "no_models"
		result.Latency = time.Since(start).Round(time.Millisecond).String()
		result.Details = "No models available"
		return result
	}

	// Make a minimal inference request with the first model
	modelID := modelsData.Data[0].ID
	reqBody := map[string]interface{}{
		"model": modelID,
		"messages": []map[string]string{
			{"role": "user", "content": "hi"},
		},
		"max_tokens": 1, // Minimal tokens to reduce cost/latency
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		result.Status = "error"
		result.Latency = time.Since(start).Round(time.Millisecond).String()
		result.Error = fmt.Sprintf("failed to marshal request: %v", err)
		return result
	}

	inferReq, err := http.NewRequestWithContext(ctx, "POST", completionsURL, bytes.NewReader(bodyBytes))
	if err != nil {
		result.Status = "error"
		result.Latency = "-"
		result.Error = err.Error()
		return result
	}
	// Set both headers for compatibility - API router accepts either X-API-Key or Authorization Bearer
	inferReq.Header.Set("X-API-Key", apiKey)
	inferReq.Header.Set("Authorization", "Bearer "+apiKey)
	inferReq.Header.Set("Content-Type", "application/json")

	inferResp, err := client.Do(inferReq)
	latency := time.Since(start)
	result.Latency = latency.Round(time.Millisecond).String()

	if err != nil {
		result.Status = "backend_error"
		if strings.Contains(err.Error(), "connection refused") {
			result.Error = "Backend connection refused"
			result.Details = "vLLM backend not reachable - check Knative/Istio routing"
		} else if strings.Contains(err.Error(), "timeout") {
			result.Status = "timeout"
			result.Error = "Request timed out"
		} else {
			result.Error = err.Error()
		}
		return result
	}
	defer inferResp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(inferResp.Body, 2048))
	if err != nil {
		result.Status = "error"
		result.Error = fmt.Sprintf("failed to read inference response: %v", err)
		return result
	}

	switch {
	case inferResp.StatusCode == 200:
		if latency > 10*time.Second {
			result.Status = "slow"
			result.Details = fmt.Sprintf("Model: %s (response > 10s)", modelID)
		} else {
			result.Status = "healthy"
			result.Details = fmt.Sprintf("Model: %s", modelID)
		}
	case inferResp.StatusCode == 401 || inferResp.StatusCode == 403:
		result.Status = "auth_required"
		result.Details = "Inference requires valid API key"
	case inferResp.StatusCode == 503:
		result.Status = "backend_error"
		result.Error = "Backend unavailable (503)"
		// Try to extract error details
		var errResp map[string]interface{}
		if json.Unmarshal(respBody, &errResp) == nil {
			if errMsg, ok := errResp["error"].(map[string]interface{}); ok {
				if msg, ok := errMsg["message"].(string); ok {
					result.Details = msg
				}
			}
		}
		if result.Details == "" {
			result.Details = "Check vLLM backend and Knative routing"
		}
	case inferResp.StatusCode >= 500:
		result.Status = "server_error"
		result.Error = fmt.Sprintf("HTTP %d", inferResp.StatusCode)
	default:
		result.Status = "unknown"
		result.StatusCode = inferResp.StatusCode
		result.Details = fmt.Sprintf("HTTP %d", inferResp.StatusCode)
	}

	return result
}
