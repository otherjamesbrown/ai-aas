// Package integration provides integration tests for the API Router Service.
//
// Purpose:
//
//	This file tests backend connectivity to catch NetworkPolicy misconfigurations
//	early. NetworkPolicy issues can silently block inference traffic, causing
//	mysterious timeouts that are hard to debug.
//
// Key Responsibilities:
//   - Verify TCP connectivity to configured backends
//   - Distinguish between "backend not running" vs "network blocked"
//   - Fail fast with clear error messages
//   - Test both static (BACKEND_ENDPOINTS) and dynamic (model registry) backends
//
// Issue: ai-aas-isti
// Related: ai-aas-ua9s (NetworkPolicy blocking vLLM traffic)
package integration

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/config"
)

// TestBackendConnectivity verifies that all configured backends are reachable.
//
// This test catches NetworkPolicy misconfigurations by attempting TCP connections
// to backend endpoints. It helps distinguish between:
//
//   - Timeout (2-3s): Likely NetworkPolicy blocking traffic
//   - Connection refused (immediate): Backend not running (expected in CI)
//   - Success: Connectivity works
//
// The test validates both:
//   - Static backends from BACKEND_ENDPOINTS environment variable
//   - Dynamic backends from VLLM_BACKEND_URL (if set for E2E tests)
//
// Usage:
//
//	go test -v ./test/integration -run TestBackendConnectivity
//
// Environment Variables:
//
//	BACKEND_ENDPOINTS - Static backend URIs (id1:uri1,id2:uri2)
//	VLLM_BACKEND_URL  - Real vLLM backend for E2E tests (optional)
func TestBackendConnectivity(t *testing.T) {
	// Load configuration to get backend endpoints
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Create backend registry
	backendRegistry := config.NewBackendRegistry(cfg)

	// Get all configured backends
	backendIDs := backendRegistry.ListBackends()
	if len(backendIDs) == 0 {
		t.Log("No static backends configured in BACKEND_ENDPOINTS - this is OK for tests")
	}

	// Test each backend
	for _, backendID := range backendIDs {
		t.Run(backendID, func(t *testing.T) {
			backend, err := backendRegistry.GetBackend(backendID)
			if err != nil {
				t.Fatalf("failed to get backend %s: %v", backendID, err)
			}

			testBackendConnectivity(t, backendID, backend.URI)
		})
	}

	// Also test VLLM_BACKEND_URL if set (for E2E tests)
	if vllmURL := os.Getenv("VLLM_BACKEND_URL"); vllmURL != "" {
		t.Run("vllm-e2e-backend", func(t *testing.T) {
			testBackendConnectivity(t, "vllm-e2e", vllmURL)
		})
	}

	// If no backends configured, verify we get a clear message
	if len(backendIDs) == 0 && os.Getenv("VLLM_BACKEND_URL") == "" {
		t.Log("✓ No backends configured - test completed successfully (no connectivity to verify)")
	}
}

// testBackendConnectivity tests connectivity to a single backend endpoint.
func testBackendConnectivity(t *testing.T, backendID, backendURI string) {
	// Parse URI to extract host:port
	parsedURL, err := url.Parse(backendURI)
	if err != nil {
		t.Errorf("❌ Invalid backend URI %s: %v", backendURI, err)
		return
	}

	host := parsedURL.Host
	if !strings.Contains(host, ":") {
		// Add default port if missing
		if parsedURL.Scheme == "https" {
			host = host + ":443"
		} else {
			host = host + ":80"
		}
	}

	t.Logf("Testing connectivity to %s (%s)", backendID, host)

	// Attempt TCP connection with timeout
	timeout := 3 * time.Second
	startTime := time.Now()
	conn, err := net.DialTimeout("tcp", host, timeout)
	latency := time.Since(startTime)

	if err == nil {
		// Connection successful
		_ = conn.Close()
		t.Logf("✓ Backend %s is reachable at %s (latency: %v)", backendID, host, latency)

		// Try HTTP health check if endpoint looks like it has one
		tryHealthCheck(t, backendID, backendURI)
		return
	}

	// Connection failed - analyze the error
	analyzeConnectivityError(t, backendID, host, err, latency, timeout)
}

// analyzeConnectivityError provides detailed diagnostics for connectivity failures.
func analyzeConnectivityError(t *testing.T, backendID, host string, err error, latency, timeout time.Duration) {
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		// Timeout - likely NetworkPolicy issue
		t.Errorf("❌ CRITICAL: Backend %s timed out after %v", backendID, latency)
		t.Errorf("   This is LIKELY a NetworkPolicy misconfiguration blocking traffic!")
		t.Errorf("   ")
		t.Errorf("   Troubleshooting steps:")
		t.Errorf("   1. Check NetworkPolicy egress rules in namespace")
		t.Errorf("   2. Verify backend namespace matches policy selectors")
		t.Errorf("   3. Check if backend service exists: kubectl get svc -A | grep %s", extractServiceName(host))
		t.Errorf("   4. Test manually: kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- curl -v %s", host)
		t.Errorf("   ")
		t.Errorf("   Related issue: ai-aas-ua9s (NetworkPolicy blocking vLLM)")
		return
	}

	if strings.Contains(err.Error(), "connection refused") {
		// Connection refused - backend not running (expected in CI/local dev)
		t.Logf("⚠ Backend %s connection refused at %s (backend not running - expected in test environment)", backendID, host)
		t.Logf("   This is EXPECTED if backend is not deployed in test environment")
		t.Logf("   To test with real backend, set VLLM_BACKEND_URL and run E2E tests")
		// Don't fail the test - this is expected in unit/integration test environments
		return
	}

	// Other errors
	t.Logf("⚠ Backend %s connectivity check failed: %v", backendID, err)
	t.Logf("   This may indicate:")
	t.Logf("   - Invalid hostname/DNS resolution failure")
	t.Logf("   - Network routing issues")
	t.Logf("   - Backend not deployed (expected in test environment)")
	// Don't fail - might be expected depending on test environment
}

// tryHealthCheck attempts an HTTP health check if the backend has a /health endpoint.
func tryHealthCheck(t *testing.T, backendID, backendURI string) {
	// Parse base URL
	parsedURL, err := url.Parse(backendURI)
	if err != nil {
		return
	}

	// Try common health check paths
	healthPaths := []string{"/health", "/healthz", "/v1/health"}

	for _, healthPath := range healthPaths {
		healthURL := fmt.Sprintf("%s://%s%s", parsedURL.Scheme, parsedURL.Host, healthPath)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
		if err != nil {
			continue
		}

		resp, err := http.DefaultClient.Do(req)
		if err == nil && resp != nil {
			_ = resp.Body.Close()
			if resp.StatusCode == 200 {
				t.Logf("✓ Backend %s health check passed: %s (HTTP %d)", backendID, healthURL, resp.StatusCode)
				return
			}
		}
	}

	// No health check found - that's OK, we already verified TCP connectivity
	t.Logf("  (no health check endpoint found, but TCP connectivity verified)")
}

// extractServiceName extracts the service name from a host string.
// For example: "my-service.namespace.svc.cluster.local:8000" -> "my-service"
func extractServiceName(host string) string {
	// Remove port
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}

	// Extract first component (service name)
	if idx := strings.Index(host, "."); idx != -1 {
		return host[:idx]
	}

	return host
}

// TestBackendConnectivity_NetworkPolicyScenario demonstrates the test behavior
// in different failure scenarios. This test is informational only - it shows
// what different failure modes look like without failing the test suite.
func TestBackendConnectivity_NetworkPolicyScenario(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network policy scenario test in short mode")
	}

	testCases := []struct {
		name           string
		backendID      string
		backendURI     string
		expectedResult string
		skipReason     string
	}{
		{
			name:           "connection_refused",
			backendID:      "test-backend-down",
			backendURI:     "http://localhost:9999/v1/completions",
			expectedResult: "expected: backend not running",
		},
		{
			name:           "timeout_networkpolicy",
			backendID:      "test-backend-blocked",
			backendURI:     "http://192.0.2.1:8000/v1/completions", // RFC 5737 TEST-NET-1
			expectedResult: "critical: likely NetworkPolicy issue",
			skipReason:     "Skip by default (takes 3s to timeout) - uncomment to see NetworkPolicy detection",
		},
		{
			name:           "invalid_dns",
			backendID:      "test-backend-invalid-dns",
			backendURI:     "http://nonexistent.invalid:8000/v1/completions",
			expectedResult: "warning: DNS or routing issue",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skipReason != "" {
				t.Skip(tc.skipReason)
			}

			t.Logf("Testing scenario: %s", tc.name)
			t.Logf("Expected result: %s", tc.expectedResult)

			// Run connectivity test (will log results, not fail)
			testBackendConnectivity(t, tc.backendID, tc.backendURI)
		})
	}
}

// TestBackendConnectivity_RealBackend tests against a real vLLM backend if available.
// This is the test that would have caught the NetworkPolicy issue in ai-aas-ua9s.
func TestBackendConnectivity_RealBackend(t *testing.T) {
	vllmURL := os.Getenv("VLLM_BACKEND_URL")
	if vllmURL == "" {
		t.Skip("Skipping real backend test: VLLM_BACKEND_URL not set")
	}

	t.Logf("Testing against real vLLM backend: %s", vllmURL)

	// Test TCP connectivity
	testBackendConnectivity(t, "vllm-real", vllmURL)

	// Test HTTP health check
	healthURL := fmt.Sprintf("%s/health", strings.TrimSuffix(vllmURL, "/v1/chat/completions"))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		t.Fatalf("failed to create health check request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Errorf("❌ CRITICAL: Health check failed: %v", err)
		t.Errorf("   This could indicate:")
		t.Errorf("   - NetworkPolicy blocking HTTP traffic")
		t.Errorf("   - Backend not healthy")
		t.Errorf("   - Incorrect health endpoint path")
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		t.Errorf("❌ Health check returned HTTP %d (expected 200)", resp.StatusCode)
		return
	}

	t.Logf("✓ Real backend connectivity verified:")
	t.Logf("  - TCP connection: OK")
	t.Logf("  - Health check: HTTP 200 OK")
	t.Logf("  - Backend URL: %s", vllmURL)
}
