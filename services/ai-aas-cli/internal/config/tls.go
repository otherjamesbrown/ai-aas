package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"time"
)

// CreateHTTPClient creates an HTTP client with optional custom CA certificate.
// Deprecated: Use CreateHTTPClientWithOptions instead for TLS insecure support.
func CreateHTTPClient(caCertFile string) (*http.Client, error) {
	return CreateHTTPClientWithOptions(caCertFile, false)
}

// CreateHTTPClientWithOptions creates an HTTP client with optional custom CA certificate
// and TLS insecure skip verify option.
func CreateHTTPClientWithOptions(caCertFile string, tlsInsecure bool) (*http.Client, error) {
	tlsConfig := &tls.Config{}

	// Handle TLS insecure (skip certificate verification)
	if tlsInsecure {
		tlsConfig.InsecureSkipVerify = true //nolint:gosec // User-requested for dev environments
	}

	// Load custom CA certificate if provided
	if caCertFile != "" {
		caCert, err := os.ReadFile(caCertFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA cert file %s: %w", caCertFile, err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA cert from %s", caCertFile)
		}

		tlsConfig.RootCAs = caCertPool
	}

	// Return default client if no TLS configuration needed
	if !tlsInsecure && caCertFile == "" {
		return &http.Client{Timeout: 30 * time.Second}, nil
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	return client, nil
}
