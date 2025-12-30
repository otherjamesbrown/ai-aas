package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient(t *testing.T) {
	client := NewClient("https://api.example.com", "test_key")

	if client.baseURL != "https://api.example.com" {
		t.Errorf("baseURL = %q, want %q", client.baseURL, "https://api.example.com")
	}
	if client.apiKey != "test_key" {
		t.Errorf("apiKey = %q, want %q", client.apiKey, "test_key")
	}
}

func TestSetAPIKey(t *testing.T) {
	client := NewClient("https://api.example.com", "old_key")
	client.SetAPIKey("new_key")

	if client.apiKey != "new_key" {
		t.Errorf("apiKey = %q, want %q", client.apiKey, "new_key")
	}
}

func TestRedeemBootstrapKey(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request
		if r.URL.Path != "/v1/bootstrap-keys/redeem" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}

		// Return mock response
		resp := RedeemBootstrapKeyResponse{
			APIKey:      "key_abc123",
			OrgID:       "org_123",
			OrgName:     "Test Org",
			AdminUserID: "usr_admin",
			AdminEmail:  "admin@test.com",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	result, err := client.RedeemBootstrapKey(context.Background(), "bsk_test123")

	if err != nil {
		t.Fatalf("RedeemBootstrapKey() error = %v", err)
	}
	if result.APIKey != "key_abc123" {
		t.Errorf("APIKey = %q, want %q", result.APIKey, "key_abc123")
	}
	if result.OrgID != "org_123" {
		t.Errorf("OrgID = %q, want %q", result.OrgID, "org_123")
	}
}

func TestRedeemBootstrapKey_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "not found"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	_, err := client.RedeemBootstrapKey(context.Background(), "bsk_invalid")

	if err == nil {
		t.Error("expected error for not found, got nil")
	}
}

func TestRedeemBootstrapKey_Expired(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGone)
		w.Write([]byte(`{"error": "expired"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	_, err := client.RedeemBootstrapKey(context.Background(), "bsk_expired")

	if err == nil {
		t.Error("expected error for expired key, got nil")
	}
}

func TestListUsers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/orgs/org_123/users" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test_key" {
			t.Errorf("missing or invalid authorization header")
		}

		// API returns raw array of users, not a wrapper object
		resp := []User{
			{ID: "usr_1", Email: "user1@test.com", Name: "User One"},
			{ID: "usr_2", Email: "user2@test.com", Name: "User Two"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test_key")
	result, err := client.ListUsers(context.Background(), "org_123", 0, 0)

	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if len(result.Users) != 2 {
		t.Errorf("got %d users, want 2", len(result.Users))
	}
}

func TestAPIError(t *testing.T) {
	err := &APIError{
		StatusCode: 500,
		Message:    "Internal error",
		Details:    "Something went wrong",
	}

	errMsg := err.Error()
	if errMsg != "Internal error: Something went wrong" {
		t.Errorf("Error() = %q, want %q", errMsg, "Internal error: Something went wrong")
	}

	errNoDetails := &APIError{
		StatusCode: 400,
		Message:    "Bad request",
	}
	if errNoDetails.Error() != "Bad request" {
		t.Errorf("Error() = %q, want %q", errNoDetails.Error(), "Bad request")
	}
}
