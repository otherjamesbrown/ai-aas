package huggingface

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetLicenseFullName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "apache-2.0",
			input:    "apache-2.0",
			expected: "Apache License 2.0",
		},
		{
			name:     "llama3",
			input:    "llama3",
			expected: "Meta Llama 3 Community License",
		},
		{
			name:     "case insensitive",
			input:    "MIT",
			expected: "MIT License",
		},
		{
			name:     "unknown license",
			input:    "custom-license",
			expected: "custom-license",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetLicenseFullName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsRestrictiveLicense(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "llama3 is restrictive",
			input:    "llama3",
			expected: true,
		},
		{
			name:     "llama3.1 is restrictive",
			input:    "llama3.1",
			expected: true,
		},
		{
			name:     "gemma is restrictive",
			input:    "gemma",
			expected: true,
		},
		{
			name:     "cc-by-nc is restrictive",
			input:    "cc-by-nc-4.0",
			expected: true,
		},
		{
			name:     "apache-2.0 is not restrictive",
			input:    "apache-2.0",
			expected: false,
		},
		{
			name:     "mit is not restrictive",
			input:    "mit",
			expected: false,
		},
		{
			name:     "cc-by is not restrictive",
			input:    "cc-by-4.0",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRestrictiveLicense(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCommonLicensesCompleteness(t *testing.T) {
	// Ensure we have entries for common model licenses
	requiredLicenses := []string{
		"apache-2.0",
		"mit",
		"llama3",
		"gemma",
	}

	for _, license := range requiredLicenses {
		t.Run(license, func(t *testing.T) {
			_, ok := CommonLicenses[license]
			assert.True(t, ok, "CommonLicenses should contain %s", license)
		})
	}
}

func TestGetLicenseInfo(t *testing.T) {
	tests := []struct {
		name            string
		modelResponse   ModelInfo
		expectedGated   bool
		expectedLicense string
		expectedAuth    bool
	}{
		{
			name: "open model with apache license",
			modelResponse: ModelInfo{
				ID:      "test-org/test-model",
				Gated:   false,
				Private: false,
				CardData: CardData{
					License: "apache-2.0",
				},
			},
			expectedGated:   false,
			expectedLicense: "apache-2.0",
			expectedAuth:    false,
		},
		{
			name: "gated model with llama license",
			modelResponse: ModelInfo{
				ID:      "meta-llama/Llama-3-8B",
				Gated:   true,
				Private: false,
				CardData: CardData{
					License: "llama3",
				},
			},
			expectedGated:   true,
			expectedLicense: "llama3",
			expectedAuth:    true,
		},
		{
			name: "private model",
			modelResponse: ModelInfo{
				ID:      "private-org/private-model",
				Gated:   false,
				Private: true,
				CardData: CardData{
					License: "mit",
				},
			},
			expectedGated:   false,
			expectedLicense: "mit",
			expectedAuth:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(tt.modelResponse)
			}))
			defer server.Close()

			client := NewClient(WithBaseURL(server.URL))

			info, err := client.GetLicenseInfo(context.Background(), tt.modelResponse.ID)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedGated, info.IsGated)
			assert.Equal(t, tt.expectedLicense, info.LicenseType)
			assert.Equal(t, tt.expectedAuth, info.RequiresAuth)
		})
	}
}

func TestCheckAccess(t *testing.T) {
	t.Run("accessible model", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(ModelInfo{ID: "test/model"})
		}))
		defer server.Close()

		client := NewClient(WithBaseURL(server.URL))
		hasAccess, err := client.CheckAccess(context.Background(), "test/model")
		require.NoError(t, err)
		assert.True(t, hasAccess)
	})

	t.Run("forbidden model", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))
		defer server.Close()

		client := NewClient(WithBaseURL(server.URL))
		hasAccess, err := client.CheckAccess(context.Background(), "test/model")
		require.NoError(t, err)
		assert.False(t, hasAccess)
	})
}

func TestIsGatedModel(t *testing.T) {
	t.Run("gated model", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(ModelInfo{ID: "test/model", Gated: true})
		}))
		defer server.Close()

		client := NewClient(WithBaseURL(server.URL))
		isGated, err := client.IsGatedModel(context.Background(), "test/model")
		require.NoError(t, err)
		assert.True(t, isGated)
	})

	t.Run("non-gated model", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(ModelInfo{ID: "test/model", Gated: false})
		}))
		defer server.Close()

		client := NewClient(WithBaseURL(server.URL))
		isGated, err := client.IsGatedModel(context.Background(), "test/model")
		require.NoError(t, err)
		assert.False(t, isGated)
	})
}

func TestRequiresAuth(t *testing.T) {
	tests := []struct {
		name     string
		model    ModelInfo
		expected bool
	}{
		{
			name:     "public model",
			model:    ModelInfo{ID: "test/model", Private: false, Gated: false},
			expected: false,
		},
		{
			name:     "private model",
			model:    ModelInfo{ID: "test/model", Private: true, Gated: false},
			expected: true,
		},
		{
			name:     "gated model",
			model:    ModelInfo{ID: "test/model", Private: false, Gated: true},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(tt.model)
			}))
			defer server.Close()

			client := NewClient(WithBaseURL(server.URL))
			requiresAuth, err := client.RequiresAuth(context.Background(), "test/model")
			require.NoError(t, err)
			assert.Equal(t, tt.expected, requiresAuth)
		})
	}
}

