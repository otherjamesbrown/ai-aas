// Package health provides service dependency health checks for the Admin CLI.
//
// Purpose:
//
//	Perform upfront health checks for required service dependencies (user-org-service,
//	analytics-service) before executing operations. Fails fast with clear error messages
//	when services are unavailable, avoiding partial operations and unclear error states.
//
// Dependencies:
//   - net/http: HTTP client for health checks
//   - context: Timeout control
//
// Requirements Reference:
//   - specs/009-admin-cli/spec.md#FR-008 (health checks for service dependencies)
//   - specs/009-admin-cli/spec.md#NFR-007 (fail fast when services unavailable)
//
package health

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"
)

// Checker performs health checks on service dependencies.
type Checker struct {
	client  *http.Client
	timeout time.Duration
}

// NewChecker creates a new health checker with default timeout.
// Uses a custom transport that skips TLS verification for development environments
// with self-signed certificates.
func NewChecker(timeout time.Duration) *Checker {
	if timeout == 0 {
		timeout = 5 * time.Second // Default 5 seconds per NFR-007
	}
	return &Checker{
		client: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // Development environments use self-signed certs
			},
		},
		timeout: timeout,
	}
}

// NewCheckerWithClient creates a new health checker with a custom HTTP client.
func NewCheckerWithClient(timeout time.Duration, httpClient *http.Client) *Checker {
	if timeout == 0 {
		timeout = 5 * time.Second // Default 5 seconds per NFR-007
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: timeout}
	} else {
		httpClient.Timeout = timeout
	}
	return &Checker{
		client:  httpClient,
		timeout: timeout,
	}
}

// ServiceHealth represents the health status of a service.
type ServiceHealth struct {
	Service string
	Healthy bool
	URL     string
	Error   error
}

// CheckService performs a health check on a single service.
func (c *Checker) CheckService(ctx context.Context, name, endpoint string) ServiceHealth {
	var healthURL string
	switch name {
	case "user-org-service":
		healthURL = fmt.Sprintf("%s/healthz", endpoint)
	case "analytics-service":
		healthURL = fmt.Sprintf("%s/analytics/v1/status/healthz", endpoint)
	default:
		healthURL = fmt.Sprintf("%s/health", endpoint)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		return ServiceHealth{
			Service: name,
			Healthy: false,
			URL:     healthURL,
			Error:   fmt.Errorf("failed to create request: %w", err),
		}
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return ServiceHealth{
			Service: name,
			Healthy: false,
			URL:     healthURL,
			Error:   fmt.Errorf("service unreachable: %w", err),
		}
	}
	defer resp.Body.Close()

	healthy := resp.StatusCode >= 200 && resp.StatusCode < 300
	if !healthy {
		return ServiceHealth{
			Service: name,
			Healthy: false,
			URL:     healthURL,
			Error:   fmt.Errorf("service returned status %d", resp.StatusCode),
		}
	}

	return ServiceHealth{
		Service: name,
		Healthy: true,
		URL:     healthURL,
	}
}

// CheckRequired performs health checks on all required services and returns
// a list of unhealthy services. Returns error if any required service is unavailable.
func (c *Checker) CheckRequired(ctx context.Context, requiredServices map[string]string) ([]ServiceHealth, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var results []ServiceHealth
	var unhealthy []ServiceHealth

	for name, endpoint := range requiredServices {
		health := c.CheckService(ctx, name, endpoint)
		results = append(results, health)
		if !health.Healthy {
			unhealthy = append(unhealthy, health)
		}
	}

	if len(unhealthy) > 0 {
		errMsg := "Required services are unavailable:\n"
		for _, h := range unhealthy {
			errMsg += fmt.Sprintf("  - %s (%s): %v\n", h.Service, h.URL, h.Error)
		}
		errMsg += "\nTroubleshooting:\n"
		errMsg += "  - Verify services are running\n"
		errMsg += "  - Check endpoint URLs are correct\n"
		errMsg += "  - Ensure network connectivity\n"
		return results, fmt.Errorf("%s", errMsg)
	}

	return results, nil
}

