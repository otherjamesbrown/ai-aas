package service

import (
	"testing"

	"go.uber.org/zap"
)

func TestValidateBackendURL(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		url         string
		wantErr     bool
		errContains string
	}{
		{
			name:        "local environment allows localhost",
			environment: "local",
			url:         "http://localhost:8080",
			wantErr:     false,
		},
		{
			name:        "local environment allows 127.0.0.1",
			environment: "local",
			url:         "http://127.0.0.1:8080/v1/chat",
			wantErr:     false,
		},
		{
			name:        "development rejects localhost",
			environment: "development",
			url:         "http://localhost:8080",
			wantErr:     true,
			errContains: "localhost URLs not allowed",
		},
		{
			name:        "development rejects 127.0.0.1",
			environment: "development",
			url:         "http://127.0.0.1:8080/v1/chat",
			wantErr:     true,
			errContains: "localhost URLs not allowed",
		},
		{
			name:        "development rejects ::1",
			environment: "development",
			url:         "http://[::1]:8080",
			wantErr:     true,
			errContains: "localhost URLs not allowed",
		},
		{
			name:        "development allows kubernetes DNS",
			environment: "development",
			url:         "http://vllm-service.system.svc.cluster.local:8000",
			wantErr:     false,
		},
		{
			name:        "staging rejects localhost",
			environment: "staging",
			url:         "http://localhost:8000/v1/chat/completions",
			wantErr:     true,
			errContains: "localhost URLs not allowed",
		},
		{
			name:        "staging allows kubernetes DNS",
			environment: "staging",
			url:         "http://triton-service.inference.svc.cluster.local:8001",
			wantErr:     false,
		},
		{
			name:        "production rejects localhost",
			environment: "production",
			url:         "http://localhost:8000",
			wantErr:     true,
			errContains: "localhost URLs not allowed",
		},
		{
			name:        "production allows kubernetes DNS",
			environment: "production",
			url:         "http://model-service.default.svc.cluster.local:80",
			wantErr:     false,
		},
		{
			name:        "allows external domain names",
			environment: "development",
			url:         "https://api.example.com/v1/chat",
			wantErr:     false,
		},
		{
			name:        "error message suggests kubernetes DNS",
			environment: "development",
			url:         "http://localhost:8080",
			wantErr:     true,
			errContains: "svc.cluster.local",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &PolicyService{
				environment: tt.environment,
				logger:      zap.NewNop(),
			}

			err := svc.validateBackendURL(tt.url)

			if tt.wantErr {
				if err == nil {
					t.Errorf("validateBackendURL() expected error but got nil")
				} else if tt.errContains != "" && !urlContains(err.Error(), tt.errContains) {
					t.Errorf("validateBackendURL() error = %v, want error containing %q", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("validateBackendURL() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestContainsLocalhost(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{"localhost with http", "http://localhost:8080", true},
		{"localhost with https", "https://localhost:443", true},
		{"127.0.0.1", "http://127.0.0.1:8000", true},
		{"ipv6 localhost", "http://[::1]:8000", true},
		{"kubernetes DNS", "http://service.namespace.svc.cluster.local", false},
		{"external domain", "https://api.example.com", false},
		{"raw IP (not localhost)", "http://10.0.0.1:8080", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsLocalhost(tt.url)
			if got != tt.want {
				t.Errorf("containsLocalhost(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestContainsRawIP(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{"raw IP http", "http://10.0.0.1:8080", true},
		{"raw IP https", "https://192.168.1.1:443", true},
		{"localhost IP", "http://127.0.0.1:8080", true},
		{"kubernetes DNS", "http://service.namespace.svc.cluster.local:8000", false},
		{"domain name", "https://api.example.com", false},
		{"domain starting with number", "https://3scale.example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsRawIP(tt.url)
			if got != tt.want {
				t.Errorf("containsRawIP(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}
