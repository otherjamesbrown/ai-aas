package adminapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListRoutingPolicies(t *testing.T) {
	tests := []struct {
		name           string
		model          string
		organizationID string
		serverResponse RoutingPolicyListResponse
		serverStatus   int
		expectError    bool
	}{
		{
			name:           "successful list with policies",
			model:          "gpt-oss-20b",
			organizationID: "*",
			serverResponse: RoutingPolicyListResponse{
				Policies: []RoutingPolicy{
					{
						PolicyID:       "policy-1",
						OrganizationID: "*",
						Model:          "gpt-oss-20b",
						Backends: []Backend{
							{BackendID: "backend-1", Weight: 100},
						},
						Enabled: true,
					},
				},
			},
			serverStatus: http.StatusOK,
			expectError:  false,
		},
		{
			name:           "successful list with no policies",
			model:          "gpt-oss-20b",
			organizationID: "*",
			serverResponse: RoutingPolicyListResponse{
				Policies: []RoutingPolicy{},
			},
			serverStatus: http.StatusOK,
			expectError:  false,
		},
		{
			name:           "server error",
			model:          "gpt-oss-20b",
			organizationID: "*",
			serverStatus:   http.StatusInternalServerError,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Contains(t, r.URL.Path, "/v1/routing/policies")
				assert.Equal(t, tt.model, r.URL.Query().Get("model"))
				assert.Equal(t, tt.organizationID, r.URL.Query().Get("organization_id"))

				// Send response
				w.WriteHeader(tt.serverStatus)
				if tt.serverStatus == http.StatusOK {
					json.NewEncoder(w).Encode(tt.serverResponse)
				}
			}))
			defer server.Close()

			// Create client
			client := NewClient(server.URL, "test-api-key")

			// Execute
			result, err := client.ListRoutingPolicies(context.Background(), tt.model, tt.organizationID)

			// Verify
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, len(tt.serverResponse.Policies), len(result.Policies))
			}
		})
	}
}

func TestCreateRoutingPolicy(t *testing.T) {
	tests := []struct {
		name           string
		policy         RoutingPolicyCreate
		serverResponse RoutingPolicy
		serverStatus   int
		expectError    bool
	}{
		{
			name: "successful creation",
			policy: RoutingPolicyCreate{
				OrganizationID: "*",
				Model:          "gpt-oss-20b",
				Backends: []Backend{
					{BackendID: "backend-1", Weight: 100},
				},
				Enabled: ptrBool(true),
			},
			serverResponse: RoutingPolicy{
				PolicyID:       "policy-123",
				OrganizationID: "*",
				Model:          "gpt-oss-20b",
				Backends: []Backend{
					{BackendID: "backend-1", Weight: 100},
				},
				Enabled: true,
			},
			serverStatus: http.StatusCreated,
			expectError:  false,
		},
		{
			name: "creation with validation error",
			policy: RoutingPolicyCreate{
				Model: "invalid",
			},
			serverStatus: http.StatusBadRequest,
			expectError:  true,
		},
		{
			name: "server error",
			policy: RoutingPolicyCreate{
				Model: "gpt-oss-20b",
				Backends: []Backend{
					{BackendID: "backend-1", Weight: 100},
				},
			},
			serverStatus: http.StatusInternalServerError,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "/v1/routing/policies", r.URL.Path)

				// Decode request body
				var receivedPolicy RoutingPolicyCreate
				err := json.NewDecoder(r.Body).Decode(&receivedPolicy)
				assert.NoError(t, err)

				// Send response
				w.WriteHeader(tt.serverStatus)
				if tt.serverStatus == http.StatusCreated {
					json.NewEncoder(w).Encode(tt.serverResponse)
				}
			}))
			defer server.Close()

			// Create client
			client := NewClient(server.URL, "test-api-key")

			// Execute
			result, err := client.CreateRoutingPolicy(context.Background(), tt.policy)

			// Verify
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tt.serverResponse.PolicyID, result.PolicyID)
				assert.Equal(t, tt.serverResponse.Model, result.Model)
			}
		})
	}
}

// ptrBool returns a pointer to a bool value
func ptrBool(b bool) *bool {
	return &b
}
