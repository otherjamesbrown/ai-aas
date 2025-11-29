package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
)

// CreateHTTPClient creates an HTTP client with optional custom CA certificate.
func CreateHTTPClient(caCertFile string) (*http.Client, error) {
	if caCertFile == "" {
		// Use default system certificates
		return &http.Client{}, nil
	}

	// Load custom CA certificate
	caCert, err := os.ReadFile(caCertFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA cert file %s: %w", caCertFile, err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA cert from %s", caCertFile)
	}

	tlsConfig := &tls.Config{
		RootCAs: caCertPool,
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	return client, nil
}
