// Package huggingface provides a client for interacting with the HuggingFace Hub API.
package huggingface

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetLicenseInfo(t *testing.T) {
	t.Run("non-gated model with license", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := ModelInfo{
				ID:      "org/model",
				Gated:   false,
				Private: false,
				CardData: CardData{
					License: "apache-2.0",
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := NewClient(WithBaseURL(server.URL))
		info, err := client.GetLicenseInfo(context.Background(), "org/model")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.IsGated {
			t.Error("expected non-gated model")
		}
		if info.LicenseType != "apache-2.0" {
			t.Errorf("expected license apache-2.0, got %s", info.LicenseType)
		}
		if info.RequiresAuth {
			t.Error("expected no auth required")
		}
	})

	t.Run("gated model", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := ModelInfo{
				ID:      "meta-llama/Llama-2-7b",
				Gated:   true,
				Private: false,
				CardData: CardData{
					License: "llama2",
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := NewClient(WithBaseURL(server.URL))
		info, err := client.GetLicenseInfo(context.Background(), "meta-llama/Llama-2-7b")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !info.IsGated {
			t.Error("expected gated model")
		}
		if !info.RequiresAuth {
			t.Error("expected auth required for gated model")
		}
		if info.AcceptanceURL == "" {
			t.Error("expected acceptance URL for gated model")
		}
	})

	t.Run("private model", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := ModelInfo{
				ID:      "org/private-model",
				Private: true,
				Gated:   false,
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := NewClient(WithBaseURL(server.URL))
		info, err := client.GetLicenseInfo(context.Background(), "org/private-model")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !info.RequiresAuth {
			t.Error("expected auth required for private model")
		}
	})

	t.Run("model with license link", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := ModelInfo{
				ID: "org/model",
				CardData: CardData{
					License:     "custom",
					LicenseLink: "https://example.com/license",
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := NewClient(WithBaseURL(server.URL))
		info, err := client.GetLicenseInfo(context.Background(), "org/model")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.LicenseURL != "https://example.com/license" {
			t.Errorf("expected custom license URL, got %s", info.LicenseURL)
		}
	})

	t.Run("model with multiple licenses", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := ModelInfo{
				ID: "org/model",
				CardData: CardData{
					License: []interface{}{"mit", "apache-2.0"},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := NewClient(WithBaseURL(server.URL))
		info, err := client.GetLicenseInfo(context.Background(), "org/model")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.LicenseType != "mit" {
			t.Errorf("expected first license mit, got %s", info.LicenseType)
		}
	})

	t.Run("authenticated user can access", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(ModelInfo{ID: "org/model"})
		}))
		defer server.Close()

		client := NewClient(WithBaseURL(server.URL), WithToken("valid-token"))
		info, err := client.GetLicenseInfo(context.Background(), "org/model")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !info.CanAccess {
			t.Error("expected CanAccess to be true for authenticated user")
		}
	})
}

func TestCheckAccess(t *testing.T) {
	t.Run("has access", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(ModelInfo{ID: "org/model"})
		}))
		defer server.Close()

		client := NewClient(WithBaseURL(server.URL))
		hasAccess, err := client.CheckAccess(context.Background(), "org/model")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !hasAccess {
			t.Error("expected access")
		}
	})

	t.Run("forbidden", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))
		defer server.Close()

		client := NewClient(WithBaseURL(server.URL))
		hasAccess, err := client.CheckAccess(context.Background(), "gated/model")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if hasAccess {
			t.Error("expected no access for forbidden model")
		}
	})

	t.Run("unauthorized", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer server.Close()

		client := NewClient(WithBaseURL(server.URL))
		hasAccess, err := client.CheckAccess(context.Background(), "private/model")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if hasAccess {
			t.Error("expected no access for unauthorized model")
		}
	})
}

func TestIsGatedModel(t *testing.T) {
	t.Run("gated model", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(ModelInfo{ID: "org/model", Gated: true})
		}))
		defer server.Close()

		client := NewClient(WithBaseURL(server.URL))
		isGated, err := client.IsGatedModel(context.Background(), "org/model")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !isGated {
			t.Error("expected gated model")
		}
	})

	t.Run("non-gated model", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(ModelInfo{ID: "org/model", Gated: false})
		}))
		defer server.Close()

		client := NewClient(WithBaseURL(server.URL))
		isGated, err := client.IsGatedModel(context.Background(), "org/model")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if isGated {
			t.Error("expected non-gated model")
		}
	})
}

func TestRequiresAuth(t *testing.T) {
	t.Run("private model", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(ModelInfo{ID: "org/model", Private: true})
		}))
		defer server.Close()

		client := NewClient(WithBaseURL(server.URL))
		requiresAuth, err := client.RequiresAuth(context.Background(), "org/model")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !requiresAuth {
			t.Error("expected auth required for private model")
		}
	})

	t.Run("gated model", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(ModelInfo{ID: "org/model", Gated: true})
		}))
		defer server.Close()

		client := NewClient(WithBaseURL(server.URL))
		requiresAuth, err := client.RequiresAuth(context.Background(), "org/model")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !requiresAuth {
			t.Error("expected auth required for gated model")
		}
	})

	t.Run("public model", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(ModelInfo{ID: "org/model", Private: false, Gated: false})
		}))
		defer server.Close()

		client := NewClient(WithBaseURL(server.URL))
		requiresAuth, err := client.RequiresAuth(context.Background(), "org/model")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if requiresAuth {
			t.Error("expected no auth required for public model")
		}
	})
}

func TestGetLicenseFullName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"apache-2.0", "Apache License 2.0"},
		{"APACHE-2.0", "Apache License 2.0"}, // Case insensitive
		{"mit", "MIT License"},
		{"llama2", "Meta Llama 2 Community License"},
		{"llama3", "Meta Llama 3 Community License"},
		{"llama3.1", "Meta Llama 3.1 Community License"},
		{"gemma", "Gemma Terms of Use"},
		{"unknown-license", "unknown-license"}, // Returns input for unknown
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := GetLicenseFullName(tc.input)
			if result != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, result)
			}
		})
	}
}

func TestIsRestrictiveLicense(t *testing.T) {
	tests := []struct {
		input       string
		restrictive bool
	}{
		{"llama2", true},
		{"llama3", true},
		{"llama3.1", true},
		{"llama3.2", true},
		{"gemma", true},
		{"qwen", true},
		{"cc-by-nc-4.0", true},
		{"cc-by-nc-nd-4.0", true},
		{"apache-2.0", false},
		{"mit", false},
		{"gpl-3.0", false},
		{"cc-by-4.0", false},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := IsRestrictiveLicense(tc.input)
			if result != tc.restrictive {
				t.Errorf("expected restrictive=%v for %s, got %v", tc.restrictive, tc.input, result)
			}
		})
	}
}

func TestGetLicenseInfo_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	_, err := client.GetLicenseInfo(context.Background(), "nonexistent/model")

	if err == nil {
		t.Error("expected error for nonexistent model")
	}
}

func TestCheckAccess_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	_, err := client.CheckAccess(context.Background(), "org/model")

	if err == nil {
		t.Error("expected error for 500 status")
	}
}

func TestIsGatedModel_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	_, err := client.IsGatedModel(context.Background(), "nonexistent/model")

	if err == nil {
		t.Error("expected error for nonexistent model")
	}
}

func TestRequiresAuth_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	_, err := client.RequiresAuth(context.Background(), "nonexistent/model")

	if err == nil {
		t.Error("expected error for nonexistent model")
	}
}

func TestGetLicenseInfo_NoLicense(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ModelInfo{
			ID:      "org/model",
			Gated:   false,
			Private: false,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	info, err := client.GetLicenseInfo(context.Background(), "org/model")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.LicenseType != "" {
		t.Errorf("expected empty license type, got %s", info.LicenseType)
	}
	if info.LicenseURL != "" {
		t.Errorf("expected empty license URL, got %s", info.LicenseURL)
	}
}

func TestGetLicenseInfo_EmptyLicenseArray(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ModelInfo{
			ID: "org/model",
			CardData: CardData{
				License: []interface{}{},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	info, err := client.GetLicenseInfo(context.Background(), "org/model")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.LicenseType != "" {
		t.Errorf("expected empty license type for empty array, got %s", info.LicenseType)
	}
}

func TestGetLicenseInfo_NonStringLicenseInArray(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ModelInfo{
			ID: "org/model",
			CardData: CardData{
				License: []interface{}{123}, // non-string
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	info, err := client.GetLicenseInfo(context.Background(), "org/model")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.LicenseType != "" {
		t.Errorf("expected empty license type for non-string, got %s", info.LicenseType)
	}
}
