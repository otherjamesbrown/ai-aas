package main

import (
	"testing"
)

func TestExtractHostPort(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		wantHost string
		wantPort string
		wantErr  bool
	}{
		{
			name:     "http with explicit port and path",
			uri:      "http://localhost:8000/v1/completions",
			wantHost: "localhost",
			wantPort: "8000",
			wantErr:  false,
		},
		{
			name:     "https with explicit port and path",
			uri:      "https://model.svc.cluster.local:8080/v1",
			wantHost: "model.svc.cluster.local",
			wantPort: "8080",
			wantErr:  false,
		},
		{
			name:     "http without port",
			uri:      "http://10.0.0.1/v1",
			wantHost: "10.0.0.1",
			wantPort: "80",
			wantErr:  false,
		},
		{
			name:     "https without port",
			uri:      "https://10.0.0.1/v1",
			wantHost: "10.0.0.1",
			wantPort: "443",
			wantErr:  false,
		},
		{
			name:     "http with port no path",
			uri:      "http://localhost:8000",
			wantHost: "localhost",
			wantPort: "8000",
			wantErr:  false,
		},
		{
			name:     "kubernetes service DNS",
			uri:      "http://vllm-service.system.svc.cluster.local:8000/v1/completions",
			wantHost: "vllm-service.system.svc.cluster.local",
			wantPort: "8000",
			wantErr:  false,
		},
		{
			name:     "ipv6 with port",
			uri:      "http://[::1]:8000/v1",
			wantHost: "::1",
			wantPort: "8000",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotHost, gotPort, err := extractHostPort(tt.uri)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractHostPort() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotHost != tt.wantHost {
				t.Errorf("extractHostPort() gotHost = %v, want %v", gotHost, tt.wantHost)
			}
			if gotPort != tt.wantPort {
				t.Errorf("extractHostPort() gotPort = %v, want %v", gotPort, tt.wantPort)
			}
		})
	}
}
