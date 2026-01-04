package controllers

import (
	"context"
	"fmt"
	"testing"

	"github.com/ai-aas/ai-model-operator/internal/adminapi"
	aimodelv1alpha1 "github.com/ai-aas/ai-model-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// mockAdminAPIClient is a mock implementation of AdminAPIClient for testing
type mockAdminAPIClient struct {
	createDeploymentCalled      bool
	updateDeploymentCalled      bool
	deleteDeploymentCalled      bool
	listRoutingPoliciesCalled   bool
	createRoutingPolicyCalled   bool
	listRoutingPoliciesResponse *adminapi.RoutingPolicyListResponse
	listRoutingPoliciesError    error
	createRoutingPolicyResponse *adminapi.RoutingPolicy
	createRoutingPolicyError    error
	createRoutingPolicyRequest  *adminapi.RoutingPolicyCreate
}

func (m *mockAdminAPIClient) CreateDeployment(ctx context.Context, req adminapi.CreateDeploymentRequest) error {
	m.createDeploymentCalled = true
	return nil
}

func (m *mockAdminAPIClient) UpdateDeploymentStatus(ctx context.Context, modelName, environment string, status adminapi.DeploymentStatus) error {
	m.updateDeploymentCalled = true
	return nil
}

func (m *mockAdminAPIClient) DeleteDeployment(ctx context.Context, modelName, environment string) error {
	m.deleteDeploymentCalled = true
	return nil
}

func (m *mockAdminAPIClient) ListRoutingPolicies(ctx context.Context, model, organizationID string) (*adminapi.RoutingPolicyListResponse, error) {
	m.listRoutingPoliciesCalled = true
	return m.listRoutingPoliciesResponse, m.listRoutingPoliciesError
}

func (m *mockAdminAPIClient) CreateRoutingPolicy(ctx context.Context, policy adminapi.RoutingPolicyCreate) (*adminapi.RoutingPolicy, error) {
	m.createRoutingPolicyCalled = true
	m.createRoutingPolicyRequest = &policy
	return m.createRoutingPolicyResponse, m.createRoutingPolicyError
}

func TestEnsureRoutingPolicy(t *testing.T) {
	tests := []struct {
		name                      string
		aiModel                   *aimodelv1alpha1.AIModel
		mockListResponse          *adminapi.RoutingPolicyListResponse
		mockListError             error
		mockCreateResponse        *adminapi.RoutingPolicy
		mockCreateError           error
		expectListCalled          bool
		expectCreateCalled        bool
		expectedBackendID         string
		expectedModel             string
		expectedOrganizationID    string
	}{
		{
			name: "creates routing policy when none exists",
			aiModel: &aimodelv1alpha1.AIModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "llama-7b",
					Namespace: "system",
				},
				Spec: aimodelv1alpha1.AIModelSpec{
					ModelID:      "meta-llama/Llama-2-7b-hf",
					ExternalName: "llama-7b",
				},
				Status: aimodelv1alpha1.AIModelStatus{
					InferenceServiceName: "llama-7b",
				},
			},
			mockListResponse: &adminapi.RoutingPolicyListResponse{
				Policies: []adminapi.RoutingPolicy{},
			},
			mockListError: nil,
			mockCreateResponse: &adminapi.RoutingPolicy{
				PolicyID:       "policy-123",
				OrganizationID: "*",
				Model:          "llama-7b",
				Enabled:        true,
			},
			mockCreateError:        nil,
			expectListCalled:       true,
			expectCreateCalled:     true,
			expectedBackendID:      "llama-7b",
			expectedModel:          "llama-7b",
			expectedOrganizationID: "*",
		},
		{
			name: "skips creation when policy already exists",
			aiModel: &aimodelv1alpha1.AIModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "llama-7b",
					Namespace: "system",
				},
				Spec: aimodelv1alpha1.AIModelSpec{
					ModelID:      "meta-llama/Llama-2-7b-hf",
					ExternalName: "llama-7b",
				},
				Status: aimodelv1alpha1.AIModelStatus{
					InferenceServiceName: "llama-7b",
				},
			},
			mockListResponse: &adminapi.RoutingPolicyListResponse{
				Policies: []adminapi.RoutingPolicy{
					{
						PolicyID:       "existing-policy",
						OrganizationID: "*",
						Model:          "llama-7b",
						Enabled:        true,
					},
				},
			},
			mockListError:      nil,
			expectListCalled:   true,
			expectCreateCalled: false,
		},
		{
			name: "derives external name from ModelID when not set",
			aiModel: &aimodelv1alpha1.AIModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "llama-7b",
					Namespace: "system",
				},
				Spec: aimodelv1alpha1.AIModelSpec{
					ModelID: "meta-llama/Llama-2-7b-hf",
					// ExternalName is empty - should derive from ModelID
				},
				Status: aimodelv1alpha1.AIModelStatus{
					InferenceServiceName: "llama-7b",
				},
			},
			mockListResponse: &adminapi.RoutingPolicyListResponse{
				Policies: []adminapi.RoutingPolicy{},
			},
			mockListError: nil,
			mockCreateResponse: &adminapi.RoutingPolicy{
				PolicyID:       "policy-456",
				OrganizationID: "*",
				Model:          "Llama-2-7b-hf",
				Enabled:        true,
			},
			mockCreateError:        nil,
			expectListCalled:       true,
			expectCreateCalled:     true,
			expectedBackendID:      "llama-7b",
			expectedModel:          "Llama-2-7b-hf", // Derived from ModelID
			expectedOrganizationID: "*",
		},
		{
			name: "handles list error gracefully",
			aiModel: &aimodelv1alpha1.AIModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "llama-7b",
					Namespace: "system",
				},
				Spec: aimodelv1alpha1.AIModelSpec{
					ModelID:      "meta-llama/Llama-2-7b-hf",
					ExternalName: "llama-7b",
				},
			},
			mockListResponse:   nil,
			mockListError:      fmt.Errorf("API error"),
			expectListCalled:   true,
			expectCreateCalled: false,
		},
		{
			name: "handles create error gracefully",
			aiModel: &aimodelv1alpha1.AIModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "llama-7b",
					Namespace: "system",
				},
				Spec: aimodelv1alpha1.AIModelSpec{
					ModelID:      "meta-llama/Llama-2-7b-hf",
					ExternalName: "llama-7b",
				},
				Status: aimodelv1alpha1.AIModelStatus{
					InferenceServiceName: "llama-7b",
				},
			},
			mockListResponse: &adminapi.RoutingPolicyListResponse{
				Policies: []adminapi.RoutingPolicy{},
			},
			mockListError:          nil,
			mockCreateResponse:     nil,
			mockCreateError:        fmt.Errorf("creation failed"),
			expectListCalled:       true,
			expectCreateCalled:     true,
			expectedBackendID:      "llama-7b",
			expectedModel:          "llama-7b",
			expectedOrganizationID: "*",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock client
			mockClient := &mockAdminAPIClient{
				listRoutingPoliciesResponse: tt.mockListResponse,
				listRoutingPoliciesError:    tt.mockListError,
				createRoutingPolicyResponse: tt.mockCreateResponse,
				createRoutingPolicyError:    tt.mockCreateError,
			}

			// Create reconciler with mock client
			reconciler := &AIModelReconciler{
				AdminAPIClient: mockClient,
			}

			// Execute
			err := reconciler.ensureRoutingPolicy(context.Background(), tt.aiModel)

			// Should never return error (errors are logged but don't fail reconciliation)
			assert.NoError(t, err)

			// Verify mock calls
			assert.Equal(t, tt.expectListCalled, mockClient.listRoutingPoliciesCalled, "ListRoutingPolicies call mismatch")
			assert.Equal(t, tt.expectCreateCalled, mockClient.createRoutingPolicyCalled, "CreateRoutingPolicy call mismatch")

			// If create was called, verify the request
			if tt.expectCreateCalled {
				require.NotNil(t, mockClient.createRoutingPolicyRequest, "CreateRoutingPolicy was called but request is nil")
				assert.Equal(t, tt.expectedModel, mockClient.createRoutingPolicyRequest.Model, "Model mismatch in create request")
				assert.Equal(t, tt.expectedOrganizationID, mockClient.createRoutingPolicyRequest.OrganizationID, "OrganizationID mismatch")
				require.Len(t, mockClient.createRoutingPolicyRequest.Backends, 1, "Expected exactly 1 backend")
				assert.Equal(t, tt.expectedBackendID, mockClient.createRoutingPolicyRequest.Backends[0].BackendID, "BackendID mismatch")
				assert.Equal(t, 100, mockClient.createRoutingPolicyRequest.Backends[0].Weight, "Weight should be 100")
				require.NotNil(t, mockClient.createRoutingPolicyRequest.Enabled, "Enabled should not be nil")
				assert.True(t, *mockClient.createRoutingPolicyRequest.Enabled, "Enabled should be true")
			}
		})
	}
}

func TestEnsureRoutingPolicy_NoAdminClient(t *testing.T) {
	reconciler := &AIModelReconciler{
		AdminAPIClient: nil, // No admin client configured
	}

	aiModel := &aimodelv1alpha1.AIModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "llama-7b",
			Namespace: "system",
		},
		Spec: aimodelv1alpha1.AIModelSpec{
			ModelID:      "meta-llama/Llama-2-7b-hf",
			ExternalName: "llama-7b",
		},
	}

	err := reconciler.ensureRoutingPolicy(context.Background(), aiModel)
	assert.NoError(t, err, "Should not error when AdminAPIClient is nil")
}

func TestEnsureRoutingPolicy_NoExternalName(t *testing.T) {
	mockClient := &mockAdminAPIClient{
		listRoutingPoliciesResponse: &adminapi.RoutingPolicyListResponse{
			Policies: []adminapi.RoutingPolicy{},
		},
	}

	reconciler := &AIModelReconciler{
		AdminAPIClient: mockClient,
	}

	aiModel := &aimodelv1alpha1.AIModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "llama-7b",
			Namespace: "system",
		},
		Spec: aimodelv1alpha1.AIModelSpec{
			ModelID: "", // No ModelID or ExternalName - cannot derive name
		},
	}

	err := reconciler.ensureRoutingPolicy(context.Background(), aiModel)
	assert.NoError(t, err, "Should not error when external name is empty")
	assert.False(t, mockClient.listRoutingPoliciesCalled, "Should skip policy operations when no external name")
	assert.False(t, mockClient.createRoutingPolicyCalled, "Should skip policy operations when no external name")
}

func TestCheckRoutingPolicyExists(t *testing.T) {
	tests := []struct {
		name             string
		aiModel          *aimodelv1alpha1.AIModel
		adminAPIClient   AdminAPIClient
		mockListResponse *adminapi.RoutingPolicyListResponse
		mockListError    error
		expectedResult   bool
	}{
		{
			name: "returns true when routing policy exists",
			aiModel: &aimodelv1alpha1.AIModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "llama-7b",
					Namespace: "system",
				},
				Spec: aimodelv1alpha1.AIModelSpec{
					ModelID:      "meta-llama/Llama-2-7b-hf",
					ExternalName: "llama-7b",
				},
			},
			mockListResponse: &adminapi.RoutingPolicyListResponse{
				Policies: []adminapi.RoutingPolicy{
					{
						PolicyID:       "policy-123",
						OrganizationID: "*",
						Model:          "llama-7b",
						Enabled:        true,
					},
				},
			},
			mockListError:  nil,
			expectedResult: true,
		},
		{
			name: "returns false when no routing policy exists",
			aiModel: &aimodelv1alpha1.AIModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "llama-7b",
					Namespace: "system",
				},
				Spec: aimodelv1alpha1.AIModelSpec{
					ModelID:      "meta-llama/Llama-2-7b-hf",
					ExternalName: "llama-7b",
				},
			},
			mockListResponse: &adminapi.RoutingPolicyListResponse{
				Policies: []adminapi.RoutingPolicy{},
			},
			mockListError:  nil,
			expectedResult: false,
		},
		{
			name: "returns false when API call fails",
			aiModel: &aimodelv1alpha1.AIModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "llama-7b",
					Namespace: "system",
				},
				Spec: aimodelv1alpha1.AIModelSpec{
					ModelID:      "meta-llama/Llama-2-7b-hf",
					ExternalName: "llama-7b",
				},
			},
			mockListResponse: nil,
			mockListError:    fmt.Errorf("API error"),
			expectedResult:   false,
		},
		{
			name: "returns false when external name is empty",
			aiModel: &aimodelv1alpha1.AIModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "llama-7b",
					Namespace: "system",
				},
				Spec: aimodelv1alpha1.AIModelSpec{
					ModelID: "meta-llama/Llama-2-7b-hf",
					// ExternalName is empty
				},
			},
			mockListResponse: nil,
			mockListError:    nil,
			expectedResult:   false,
		},
		{
			name: "returns true when Admin API client is nil (no validation possible)",
			aiModel: &aimodelv1alpha1.AIModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "llama-7b",
					Namespace: "system",
				},
				Spec: aimodelv1alpha1.AIModelSpec{
					ModelID:      "meta-llama/Llama-2-7b-hf",
					ExternalName: "llama-7b",
				},
			},
			adminAPIClient: nil, // No Admin API client configured
			mockListResponse: nil,
			mockListError:    nil,
			expectedResult:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var reconciler *AIModelReconciler

			if tt.adminAPIClient == nil && tt.name == "returns true when Admin API client is nil (no validation possible)" {
				// Test case with nil Admin API client
				reconciler = &AIModelReconciler{
					AdminAPIClient: nil,
				}
			} else {
				// Setup mock Admin API client
				mockClient := &mockAdminAPIClient{
					listRoutingPoliciesResponse: tt.mockListResponse,
					listRoutingPoliciesError:    tt.mockListError,
				}
				reconciler = &AIModelReconciler{
					AdminAPIClient: mockClient,
				}
			}

			result := reconciler.checkRoutingPolicyExists(context.Background(), tt.aiModel)
			assert.Equal(t, tt.expectedResult, result, "checkRoutingPolicyExists result should match expected")
		})
	}
}

func TestDetectRoutingPolicyDrift(t *testing.T) {
	tests := []struct {
		name             string
		aiModel          *aimodelv1alpha1.AIModel
		mockListResponse *adminapi.RoutingPolicyListResponse
		mockListError    error
		expectedDrift    bool
	}{
		{
			name: "detects drift when backend_id differs from InferenceServiceName",
			aiModel: &aimodelv1alpha1.AIModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "llama-7b",
					Namespace: "system",
				},
				Spec: aimodelv1alpha1.AIModelSpec{
					ModelID:      "meta-llama/Llama-2-7b-hf",
					ExternalName: "llama-7b",
				},
				Status: aimodelv1alpha1.AIModelStatus{
					InferenceServiceName: "llama-7b-v2", // Expected backend ID
				},
			},
			mockListResponse: &adminapi.RoutingPolicyListResponse{
				Policies: []adminapi.RoutingPolicy{
					{
						PolicyID:       "policy-123",
						OrganizationID: "*",
						Model:          "llama-7b",
						Backends: []adminapi.Backend{
							{
								BackendID: "llama-7b-v1", // Actual backend ID (drift!)
								Weight:    100,
							},
						},
						Enabled: true,
					},
				},
			},
			mockListError: nil,
			expectedDrift: true,
		},
		{
			name: "no drift when backend_id matches InferenceServiceName",
			aiModel: &aimodelv1alpha1.AIModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "llama-7b",
					Namespace: "system",
				},
				Spec: aimodelv1alpha1.AIModelSpec{
					ModelID:      "meta-llama/Llama-2-7b-hf",
					ExternalName: "llama-7b",
				},
				Status: aimodelv1alpha1.AIModelStatus{
					InferenceServiceName: "llama-7b",
				},
			},
			mockListResponse: &adminapi.RoutingPolicyListResponse{
				Policies: []adminapi.RoutingPolicy{
					{
						PolicyID:       "policy-123",
						OrganizationID: "*",
						Model:          "llama-7b",
						Backends: []adminapi.Backend{
							{
								BackendID: "llama-7b", // Matches!
								Weight:    100,
							},
						},
						Enabled: true,
					},
				},
			},
			mockListError: nil,
			expectedDrift: false,
		},
		{
			name: "no drift when no routing policies exist",
			aiModel: &aimodelv1alpha1.AIModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "llama-7b",
					Namespace: "system",
				},
				Spec: aimodelv1alpha1.AIModelSpec{
					ModelID:      "meta-llama/Llama-2-7b-hf",
					ExternalName: "llama-7b",
				},
				Status: aimodelv1alpha1.AIModelStatus{
					InferenceServiceName: "llama-7b",
				},
			},
			mockListResponse: &adminapi.RoutingPolicyListResponse{
				Policies: []adminapi.RoutingPolicy{},
			},
			mockListError: nil,
			expectedDrift: false,
		},
		{
			name: "no drift when InferenceServiceName is not set",
			aiModel: &aimodelv1alpha1.AIModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "llama-7b",
					Namespace: "system",
				},
				Spec: aimodelv1alpha1.AIModelSpec{
					ModelID:      "meta-llama/Llama-2-7b-hf",
					ExternalName: "llama-7b",
				},
				Status: aimodelv1alpha1.AIModelStatus{
					InferenceServiceName: "", // Not set yet
				},
			},
			mockListResponse: &adminapi.RoutingPolicyListResponse{
				Policies: []adminapi.RoutingPolicy{
					{
						PolicyID:       "policy-123",
						OrganizationID: "*",
						Model:          "llama-7b",
						Backends: []adminapi.Backend{
							{
								BackendID: "some-backend",
								Weight:    100,
							},
						},
						Enabled: true,
					},
				},
			},
			mockListError: nil,
			expectedDrift: false, // Not drift - just not ready yet
		},
		{
			name: "no drift when Admin API client is nil",
			aiModel: &aimodelv1alpha1.AIModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "llama-7b",
					Namespace: "system",
				},
				Spec: aimodelv1alpha1.AIModelSpec{
					ModelID:      "meta-llama/Llama-2-7b-hf",
					ExternalName: "llama-7b",
				},
				Status: aimodelv1alpha1.AIModelStatus{
					InferenceServiceName: "llama-7b",
				},
			},
			mockListResponse: nil,
			mockListError:    nil,
			expectedDrift:    false,
		},
		{
			name: "no drift when API call fails",
			aiModel: &aimodelv1alpha1.AIModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "llama-7b",
					Namespace: "system",
				},
				Spec: aimodelv1alpha1.AIModelSpec{
					ModelID:      "meta-llama/Llama-2-7b-hf",
					ExternalName: "llama-7b",
				},
				Status: aimodelv1alpha1.AIModelStatus{
					InferenceServiceName: "llama-7b",
				},
			},
			mockListResponse: nil,
			mockListError:    fmt.Errorf("API error"),
			expectedDrift:    false,
		},
		{
			name: "detects drift with multiple backends (one mismatched)",
			aiModel: &aimodelv1alpha1.AIModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "llama-7b",
					Namespace: "system",
				},
				Spec: aimodelv1alpha1.AIModelSpec{
					ModelID:      "meta-llama/Llama-2-7b-hf",
					ExternalName: "llama-7b",
				},
				Status: aimodelv1alpha1.AIModelStatus{
					InferenceServiceName: "llama-7b-v2",
				},
			},
			mockListResponse: &adminapi.RoutingPolicyListResponse{
				Policies: []adminapi.RoutingPolicy{
					{
						PolicyID:       "policy-123",
						OrganizationID: "*",
						Model:          "llama-7b",
						Backends: []adminapi.Backend{
							{
								BackendID: "llama-7b-v1", // Drift!
								Weight:    50,
							},
							{
								BackendID: "llama-7b-v2", // Matches
								Weight:    50,
							},
						},
						Enabled: true,
					},
				},
			},
			mockListError: nil,
			expectedDrift: true,
		},
		{
			name: "detects drift across multiple policies",
			aiModel: &aimodelv1alpha1.AIModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "llama-7b",
					Namespace: "system",
				},
				Spec: aimodelv1alpha1.AIModelSpec{
					ModelID:      "meta-llama/Llama-2-7b-hf",
					ExternalName: "llama-7b",
				},
				Status: aimodelv1alpha1.AIModelStatus{
					InferenceServiceName: "llama-7b-current",
				},
			},
			mockListResponse: &adminapi.RoutingPolicyListResponse{
				Policies: []adminapi.RoutingPolicy{
					{
						PolicyID:       "policy-org1",
						OrganizationID: "org1",
						Model:          "llama-7b",
						Backends: []adminapi.Backend{
							{
								BackendID: "llama-7b-current", // Matches
								Weight:    100,
							},
						},
						Enabled: true,
					},
					{
						PolicyID:       "policy-org2",
						OrganizationID: "org2",
						Model:          "llama-7b",
						Backends: []adminapi.Backend{
							{
								BackendID: "llama-7b-old", // Drift!
								Weight:    100,
							},
						},
						Enabled: true,
					},
				},
			},
			mockListError: nil,
			expectedDrift: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var reconciler *AIModelReconciler

			if tt.name == "no drift when Admin API client is nil" {
				// Test case with nil Admin API client
				reconciler = &AIModelReconciler{
					AdminAPIClient: nil,
				}
			} else {
				// Setup mock Admin API client
				mockClient := &mockAdminAPIClient{
					listRoutingPoliciesResponse: tt.mockListResponse,
					listRoutingPoliciesError:    tt.mockListError,
				}
				reconciler = &AIModelReconciler{
					AdminAPIClient: mockClient,
				}
			}

			// Execute
			driftDetected := reconciler.detectRoutingPolicyDrift(context.Background(), tt.aiModel)

			// Verify
			assert.Equal(t, tt.expectedDrift, driftDetected,
				"Drift detection result mismatch")
		})
	}
}
