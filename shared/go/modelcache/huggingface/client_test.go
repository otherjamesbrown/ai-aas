// Package huggingface provides a client for interacting with the HuggingFace Hub API.
package huggingface

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	t.Run("default configuration", func(t *testing.T) {
		client := NewClient()

		if client.baseURL != DefaultBaseURL {
			t.Errorf("expected baseURL %s, got %s", DefaultBaseURL, client.baseURL)
		}
		if client.token != "" {
			t.Errorf("expected empty token, got %s", client.token)
		}
		if client.httpClient == nil {
			t.Error("expected non-nil httpClient")
		}
	})

	t.Run("with token", func(t *testing.T) {
		client := NewClient(WithToken("test-token"))

		if client.token != "test-token" {
			t.Errorf("expected token test-token, got %s", client.token)
		}
	})

	t.Run("with custom base URL", func(t *testing.T) {
		client := NewClient(WithBaseURL("https://custom.api.com"))

		if client.baseURL != "https://custom.api.com" {
			t.Errorf("expected baseURL https://custom.api.com, got %s", client.baseURL)
		}
	})

	t.Run("with custom timeout", func(t *testing.T) {
		client := NewClient(WithTimeout(60 * time.Second))

		if client.httpClient.Timeout != 60*time.Second {
			t.Errorf("expected timeout 60s, got %v", client.httpClient.Timeout)
		}
	})

	t.Run("with custom HTTP client", func(t *testing.T) {
		customClient := &http.Client{Timeout: 120 * time.Second}
		client := NewClient(WithHTTPClient(customClient))

		if client.httpClient != customClient {
			t.Error("expected custom HTTP client to be used")
		}
	})

	t.Run("with multiple options", func(t *testing.T) {
		client := NewClient(
			WithToken("multi-token"),
			WithBaseURL("https://multi.api.com"),
		)

		if client.token != "multi-token" {
			t.Errorf("expected token multi-token, got %s", client.token)
		}
		if client.baseURL != "https://multi.api.com" {
			t.Errorf("expected baseURL https://multi.api.com, got %s", client.baseURL)
		}
	})
}

func TestGetModel(t *testing.T) {
	t.Run("successful request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/models/org/test-model" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			if r.Method != http.MethodGet {
				t.Errorf("expected GET, got %s", r.Method)
			}

			resp := ModelInfo{
				ID:       "org/test-model",
				ModelID:  "org/test-model",
				Author:   "org",
				SHA:      "abc123",
				Private:  false,
				Gated:    false,
				Downloads: 1000,
				Likes:    50,
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := NewClient(WithBaseURL(server.URL))
		info, err := client.GetModel(context.Background(), "org/test-model")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.ID != "org/test-model" {
			t.Errorf("expected ID org/test-model, got %s", info.ID)
		}
		if info.Downloads != 1000 {
			t.Errorf("expected Downloads 1000, got %d", info.Downloads)
		}
	})

	t.Run("model not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client := NewClient(WithBaseURL(server.URL))
		_, err := client.GetModel(context.Background(), "nonexistent/model")

		if err != ErrModelNotFound {
			t.Errorf("expected ErrModelNotFound, got %v", err)
		}
	})

	t.Run("unauthorized", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer server.Close()

		client := NewClient(WithBaseURL(server.URL))
		_, err := client.GetModel(context.Background(), "private/model")

		if err != ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("forbidden", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))
		defer server.Close()

		client := NewClient(WithBaseURL(server.URL))
		_, err := client.GetModel(context.Background(), "gated/model")

		if err != ErrForbidden {
			t.Errorf("expected ErrForbidden, got %v", err)
		}
	})

	t.Run("with auth token", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth != "Bearer test-token" {
				t.Errorf("expected Bearer test-token, got %s", auth)
			}
			json.NewEncoder(w).Encode(ModelInfo{ID: "org/model"})
		}))
		defer server.Close()

		client := NewClient(WithBaseURL(server.URL), WithToken("test-token"))
		_, err := client.GetModel(context.Background(), "org/model")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("context cancelled", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			json.NewEncoder(w).Encode(ModelInfo{ID: "org/model"})
		}))
		defer server.Close()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		client := NewClient(WithBaseURL(server.URL))
		_, err := client.GetModel(ctx, "org/model")

		if err == nil {
			t.Error("expected error for cancelled context")
		}
	})
}

func TestModelExists(t *testing.T) {
	t.Run("model exists", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(ModelInfo{ID: "org/model"})
		}))
		defer server.Close()

		client := NewClient(WithBaseURL(server.URL))
		exists, err := client.ModelExists(context.Background(), "org/model")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !exists {
			t.Error("expected model to exist")
		}
	})

	t.Run("model does not exist", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client := NewClient(WithBaseURL(server.URL))
		exists, err := client.ModelExists(context.Background(), "nonexistent/model")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if exists {
			t.Error("expected model to not exist")
		}
	})
}

func TestGetModelRevision(t *testing.T) {
	t.Run("successful request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/models/org/model/revision/v1.0" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			json.NewEncoder(w).Encode(ModelInfo{ID: "org/model", SHA: "rev123"})
		}))
		defer server.Close()

		client := NewClient(WithBaseURL(server.URL))
		info, err := client.GetModelRevision(context.Background(), "org/model", "v1.0")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.SHA != "rev123" {
			t.Errorf("expected SHA rev123, got %s", info.SHA)
		}
	})

	t.Run("revision not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client := NewClient(WithBaseURL(server.URL))
		_, err := client.GetModelRevision(context.Background(), "org/model", "nonexistent")

		if err != ErrRevisionNotFound {
			t.Errorf("expected ErrRevisionNotFound, got %v", err)
		}
	})
}

func TestWhoAmI(t *testing.T) {
	t.Run("successful request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/whoami-v2" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			resp := UserInfo{
				Type:     "user",
				ID:       "user123",
				Name:     "testuser",
				Fullname: "Test User",
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := NewClient(WithBaseURL(server.URL), WithToken("valid-token"))
		info, err := client.WhoAmI(context.Background())

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.Name != "testuser" {
			t.Errorf("expected Name testuser, got %s", info.Name)
		}
	})

	t.Run("no token", func(t *testing.T) {
		client := NewClient()
		_, err := client.WhoAmI(context.Background())

		if err != ErrNoToken {
			t.Errorf("expected ErrNoToken, got %v", err)
		}
	})

	t.Run("unauthorized", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer server.Close()

		client := NewClient(WithBaseURL(server.URL), WithToken("invalid-token"))
		_, err := client.WhoAmI(context.Background())

		if err != ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})
}

func TestValidateToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(UserInfo{Name: "testuser"})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL), WithToken("valid-token"))
	info, err := client.ValidateToken(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Name != "testuser" {
		t.Errorf("expected Name testuser, got %s", info.Name)
	}
}

func TestListFiles(t *testing.T) {
	t.Run("successful request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/models/org/model/tree/main" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			resp := []RepoFile{
				{Type: "file", Path: "config.json", Size: 1000},
				{Type: "file", Path: "model.safetensors", Size: 1000000},
				{Type: "directory", Path: "subdir"},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := NewClient(WithBaseURL(server.URL))
		files, err := client.ListFiles(context.Background(), "org/model", "main")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 3 {
			t.Errorf("expected 3 files, got %d", len(files))
		}
	})

	t.Run("default revision", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/models/org/model/tree/main" {
				t.Errorf("expected main revision in path, got %s", r.URL.Path)
			}
			json.NewEncoder(w).Encode([]RepoFile{})
		}))
		defer server.Close()

		client := NewClient(WithBaseURL(server.URL))
		_, err := client.ListFiles(context.Background(), "org/model", "")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("model not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client := NewClient(WithBaseURL(server.URL))
		_, err := client.ListFiles(context.Background(), "nonexistent/model", "main")

		if err != ErrModelNotFound {
			t.Errorf("expected ErrModelNotFound, got %v", err)
		}
	})
}

func TestSetHeaders(t *testing.T) {
	t.Run("without token", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Accept") != "application/json" {
				t.Errorf("expected Accept application/json")
			}
			if r.Header.Get("Authorization") != "" {
				t.Error("expected no Authorization header")
			}
			json.NewEncoder(w).Encode(ModelInfo{})
		}))
		defer server.Close()

		client := NewClient(WithBaseURL(server.URL))
		client.GetModel(context.Background(), "org/model")
	})

	t.Run("with token", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer mytoken" {
				t.Errorf("expected Bearer mytoken, got %s", r.Header.Get("Authorization"))
			}
			json.NewEncoder(w).Encode(ModelInfo{})
		}))
		defer server.Close()

		client := NewClient(WithBaseURL(server.URL), WithToken("mytoken"))
		client.GetModel(context.Background(), "org/model")
	})
}

func TestGetModel_UnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	_, err := client.GetModel(context.Background(), "org/model")

	if err == nil {
		t.Error("expected error for 500 status")
	}
}

func TestGetModelRevision_UnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	_, err := client.GetModelRevision(context.Background(), "org/model", "main")

	if err == nil {
		t.Error("expected error for 500 status")
	}
}

func TestWhoAmI_UnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL), WithToken("token"))
	_, err := client.WhoAmI(context.Background())

	if err == nil {
		t.Error("expected error for 500 status")
	}
}

func TestListFiles_UnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	_, err := client.ListFiles(context.Background(), "org/model", "main")

	if err == nil {
		t.Error("expected error for 500 status")
	}
}

func TestModelExists_OtherError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	_, err := client.ModelExists(context.Background(), "org/model")

	if err == nil {
		t.Error("expected error for 500 status")
	}
}

func TestGetModel_DecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	_, err := client.GetModel(context.Background(), "org/model")

	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestListFiles_DecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	_, err := client.ListFiles(context.Background(), "org/model", "main")

	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
