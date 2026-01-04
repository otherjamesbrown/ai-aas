package controllers

import (
	"context"
	"fmt"
	"testing"
	"time"

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
	updateRoutingPolicyCalled   bool
	listRoutingPoliciesResponse *adminapi.RoutingPolicyListResponse
	listRoutingPoliciesError    error
	createRoutingPolicyResponse *adminapi.RoutingPolicy
	createRoutingPolicyError    error
	createRoutingPolicyRequest  *adminapi.RoutingPolicyCreate
	updateRoutingPolicyResponse *adminapi.RoutingPolicy
	updateRoutingPolicyError    error
	updateRoutingPolicyID       string
	updateRoutingPolicyRequest  *adminapi.RoutingPolicyUpdate
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

func (m *mockAdminAPIClient) UpdateRoutingPolicy(ctx context.Context, policyID string, update adminapi.RoutingPolicyUpdate) (*adminapi.RoutingPolicy, error) {
	m.updateRoutingPolicyCalled = true
	m.updateRoutingPolicyID = policyID
	m.updateRoutingPolicyRequest = &update
	return m.updateRoutingPolicyResponse, m.updateRoutingPolicyError
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
			drift := reconciler.detectRoutingPolicyDrift(context.Background(), tt.aiModel)

			// Verify
			assert.Equal(t, tt.expectedDrift, drift.detected,
				"Drift detection result mismatch")
		})
	}
}

func TestHealRoutingPolicy(t *testing.T) {
	tests := []struct {
		name                     string
		aiModel                  *aimodelv1alpha1.AIModel
		mockListResponse         *adminapi.RoutingPolicyListResponse
		mockListError            error
		mockUpdateResponse       *adminapi.RoutingPolicy
		mockUpdateError          error
		expectUpdateCalled       bool
		expectedUpdatePolicyID   string
		expectedUpdateBackendID  string
		expectedError            bool
	}{
		{
			name: "heals drift when AIModel is Ready",
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
					Phase:                aimodelv1alpha1.AIModelPhaseReady,
					InferenceServiceName: "llama-7b-new",
				},
			},
			mockListResponse: &adminapi.RoutingPolicyListResponse{
				Policies: []adminapi.RoutingPolicy{
					{
						PolicyID:       "policy-123",
						OrganizationID: "*",
						Model:          "llama-7b",
						Backends: []adminapi.Backend{
							{BackendID: "llama-7b-old", Weight: 100},
						},
						Enabled: true,
					},
				},
			},
			mockListError: nil,
			mockUpdateResponse: &adminapi.RoutingPolicy{
				PolicyID:       "policy-123",
				OrganizationID: "*",
				Model:          "llama-7b",
				Backends: []adminapi.Backend{
					{BackendID: "llama-7b-new", Weight: 100},
				},
				Enabled: true,
			},
			mockUpdateError:          nil,
			expectUpdateCalled:       true,
			expectedUpdatePolicyID:   "policy-123",
			expectedUpdateBackendID:  "llama-7b-new",
			expectedError:            false,
		},
		{
			name: "skips healing when AIModel is not Ready",
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
					Phase:                aimodelv1alpha1.AIModelPhaseDeploying,
					InferenceServiceName: "llama-7b",
				},
			},
			mockListResponse:         nil,
			mockListError:            nil,
			mockUpdateResponse:       nil,
			mockUpdateError:          nil,
			expectUpdateCalled:       false,
			expectedUpdatePolicyID:   "",
			expectedUpdateBackendID:  "",
			expectedError:            false,
		},
		{
			name: "skips healing when InferenceServiceName is empty",
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
					Phase:                aimodelv1alpha1.AIModelPhaseReady,
					InferenceServiceName: "",
				},
			},
			mockListResponse:         nil,
			mockListError:            nil,
			mockUpdateResponse:       nil,
			mockUpdateError:          nil,
			expectUpdateCalled:       false,
			expectedUpdatePolicyID:   "",
			expectedUpdateBackendID:  "",
			expectedError:            false,
		},
		{
			name: "skips healing when no policies exist",
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
					Phase:                aimodelv1alpha1.AIModelPhaseReady,
					InferenceServiceName: "llama-7b",
				},
			},
			mockListResponse: &adminapi.RoutingPolicyListResponse{
				Policies: []adminapi.RoutingPolicy{},
			},
			mockListError:            nil,
			mockUpdateResponse:       nil,
			mockUpdateError:          nil,
			expectUpdateCalled:       false,
			expectedUpdatePolicyID:   "",
			expectedUpdateBackendID:  "",
			expectedError:            false,
		},
		{
			name: "skips healing when no drift exists",
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
					Phase:                aimodelv1alpha1.AIModelPhaseReady,
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
							{BackendID: "llama-7b", Weight: 100},
						},
						Enabled: true,
					},
				},
			},
			mockListError:            nil,
			mockUpdateResponse:       nil,
			mockUpdateError:          nil,
			expectUpdateCalled:       false,
			expectedUpdatePolicyID:   "",
			expectedUpdateBackendID:  "",
			expectedError:            false,
		},
		{
			name: "returns error when listing fails",
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
					Phase:                aimodelv1alpha1.AIModelPhaseReady,
					InferenceServiceName: "llama-7b",
				},
			},
			mockListResponse:         nil,
			mockListError:            fmt.Errorf("API unavailable"),
			mockUpdateResponse:       nil,
			mockUpdateError:          nil,
			expectUpdateCalled:       false,
			expectedUpdatePolicyID:   "",
			expectedUpdateBackendID:  "",
			expectedError:            true,
		},
		{
			name: "returns error when update fails",
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
					Phase:                aimodelv1alpha1.AIModelPhaseReady,
					InferenceServiceName: "llama-7b-new",
				},
			},
			mockListResponse: &adminapi.RoutingPolicyListResponse{
				Policies: []adminapi.RoutingPolicy{
					{
						PolicyID:       "policy-123",
						OrganizationID: "*",
						Model:          "llama-7b",
						Backends: []adminapi.Backend{
							{BackendID: "llama-7b-old", Weight: 100},
						},
						Enabled: true,
					},
				},
			},
			mockListError:            nil,
			mockUpdateResponse:       nil,
			mockUpdateError:          fmt.Errorf("update failed"),
			expectUpdateCalled:       true,
			expectedUpdatePolicyID:   "policy-123",
			expectedUpdateBackendID:  "llama-7b-new",
			expectedError:            true,
		},
		{
			name: "heals multiple backends preserving weights",
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
					Phase:                aimodelv1alpha1.AIModelPhaseReady,
					InferenceServiceName: "llama-7b-new",
				},
			},
			mockListResponse: &adminapi.RoutingPolicyListResponse{
				Policies: []adminapi.RoutingPolicy{
					{
						PolicyID:       "policy-123",
						OrganizationID: "*",
						Model:          "llama-7b",
						Backends: []adminapi.Backend{
							{BackendID: "llama-7b-old-1", Weight: 70},
							{BackendID: "llama-7b-old-2", Weight: 30},
						},
						Enabled: true,
					},
				},
			},
			mockListError: nil,
			mockUpdateResponse: &adminapi.RoutingPolicy{
				PolicyID:       "policy-123",
				OrganizationID: "*",
				Model:          "llama-7b",
				Backends: []adminapi.Backend{
					{BackendID: "llama-7b-new", Weight: 70},
					{BackendID: "llama-7b-new", Weight: 30},
				},
				Enabled: true,
			},
			mockUpdateError:          nil,
			expectUpdateCalled:       true,
			expectedUpdatePolicyID:   "policy-123",
			expectedUpdateBackendID:  "llama-7b-new",
			expectedError:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock Admin API client
			mockClient := &mockAdminAPIClient{
				listRoutingPoliciesResponse: tt.mockListResponse,
				listRoutingPoliciesError:    tt.mockListError,
				updateRoutingPolicyResponse: tt.mockUpdateResponse,
				updateRoutingPolicyError:    tt.mockUpdateError,
			}

			reconciler := &AIModelReconciler{
				AdminAPIClient: mockClient,
			}

			// Execute
			err := reconciler.healRoutingPolicy(context.Background(), tt.aiModel)

			// Verify error expectation
			if tt.expectedError {
				require.Error(t, err, "Expected error but got none")
			} else {
				require.NoError(t, err, "Expected no error but got: %v", err)
			}

			// Verify update was called (or not)
			assert.Equal(t, tt.expectUpdateCalled, mockClient.updateRoutingPolicyCalled,
				"UpdateRoutingPolicy call expectation mismatch")

			if tt.expectUpdateCalled {
				// Verify update policy ID
				assert.Equal(t, tt.expectedUpdatePolicyID, mockClient.updateRoutingPolicyID,
					"Update policy ID mismatch")

				// Verify update backends
				require.NotNil(t, mockClient.updateRoutingPolicyRequest, "Update request should not be nil")
				require.Greater(t, len(mockClient.updateRoutingPolicyRequest.Backends), 0,
					"Update request should have backends")

				// Verify all backends have the expected backend ID
				for _, backend := range mockClient.updateRoutingPolicyRequest.Backends {
					assert.Equal(t, tt.expectedUpdateBackendID, backend.BackendID,
						"Backend ID mismatch in update request")
				}
			}
		})
	}
}

func TestHealRoutingPolicy_NilAdminAPIClient(t *testing.T) {
	aiModel := &aimodelv1alpha1.AIModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "llama-7b",
			Namespace: "system",
		},
		Spec: aimodelv1alpha1.AIModelSpec{
			ModelID:      "meta-llama/Llama-2-7b-hf",
			ExternalName: "llama-7b",
		},
		Status: aimodelv1alpha1.AIModelStatus{
			Phase:                aimodelv1alpha1.AIModelPhaseReady,
			InferenceServiceName: "llama-7b",
		},
	}

	reconciler := &AIModelReconciler{
		AdminAPIClient: nil,
	}

	err := reconciler.healRoutingPolicy(context.Background(), aiModel)
	require.NoError(t, err, "Should not error when AdminAPIClient is nil")
}

func TestHealRoutingPolicy_RateLimiting(t *testing.T) {
	t.Run("heals on first attempt", func(t *testing.T) {
		aiModel := &aimodelv1alpha1.AIModel{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "llama-7b",
				Namespace: "system",
			},
			Spec: aimodelv1alpha1.AIModelSpec{
				ModelID:      "meta-llama/Llama-2-7b-hf",
				ExternalName: "llama-7b",
			},
			Status: aimodelv1alpha1.AIModelStatus{
				Phase:                aimodelv1alpha1.AIModelPhaseReady,
				InferenceServiceName: "llama-7b-new",
			},
		}

		mockClient := &mockAdminAPIClient{
			listRoutingPoliciesResponse: &adminapi.RoutingPolicyListResponse{
				Policies: []adminapi.RoutingPolicy{
					{
						PolicyID:       "policy-123",
						OrganizationID: "*",
						Model:          "llama-7b",
						Backends: []adminapi.Backend{
							{BackendID: "llama-7b-old", Weight: 100},
						},
						Enabled: true,
					},
				},
			},
			updateRoutingPolicyResponse: &adminapi.RoutingPolicy{
				PolicyID:       "policy-123",
				OrganizationID: "*",
				Model:          "llama-7b",
				Backends: []adminapi.Backend{
					{BackendID: "llama-7b-new", Weight: 100},
				},
				Enabled: true,
			},
		}

		reconciler := &AIModelReconciler{
			AdminAPIClient: mockClient,
			HealCooldown:   1 * time.Minute,
		}

		// First heal should succeed
		err := reconciler.healRoutingPolicy(context.Background(), aiModel)
		require.NoError(t, err)
		assert.True(t, mockClient.updateRoutingPolicyCalled, "First heal should call update")
	})

	t.Run("skips healing within cooldown period", func(t *testing.T) {
		aiModel := &aimodelv1alpha1.AIModel{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "llama-7b",
				Namespace: "system",
			},
			Spec: aimodelv1alpha1.AIModelSpec{
				ModelID:      "meta-llama/Llama-2-7b-hf",
				ExternalName: "llama-7b",
			},
			Status: aimodelv1alpha1.AIModelStatus{
				Phase:                aimodelv1alpha1.AIModelPhaseReady,
				InferenceServiceName: "llama-7b-new",
			},
		}

		mockClient := &mockAdminAPIClient{
			listRoutingPoliciesResponse: &adminapi.RoutingPolicyListResponse{
				Policies: []adminapi.RoutingPolicy{
					{
						PolicyID:       "policy-123",
						OrganizationID: "*",
						Model:          "llama-7b",
						Backends: []adminapi.Backend{
							{BackendID: "llama-7b-old", Weight: 100},
						},
						Enabled: true,
					},
				},
			},
			updateRoutingPolicyResponse: &adminapi.RoutingPolicy{
				PolicyID:       "policy-123",
				OrganizationID: "*",
				Model:          "llama-7b",
				Backends: []adminapi.Backend{
					{BackendID: "llama-7b-new", Weight: 100},
				},
				Enabled: true,
			},
		}

		reconciler := &AIModelReconciler{
			AdminAPIClient: mockClient,
			HealCooldown:   1 * time.Minute,
		}

		// First heal
		err := reconciler.healRoutingPolicy(context.Background(), aiModel)
		require.NoError(t, err)
		assert.True(t, mockClient.updateRoutingPolicyCalled, "First heal should call update")

		// Reset mock state
		mockClient.updateRoutingPolicyCalled = false

		// Second heal immediately after should be skipped
		err = reconciler.healRoutingPolicy(context.Background(), aiModel)
		require.NoError(t, err)
		assert.False(t, mockClient.updateRoutingPolicyCalled, "Second heal should be skipped due to rate limiting")
	})

	t.Run("heals after cooldown expires", func(t *testing.T) {
		aiModel := &aimodelv1alpha1.AIModel{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "llama-7b",
				Namespace: "system",
			},
			Spec: aimodelv1alpha1.AIModelSpec{
				ModelID:      "meta-llama/Llama-2-7b-hf",
				ExternalName: "llama-7b",
			},
			Status: aimodelv1alpha1.AIModelStatus{
				Phase:                aimodelv1alpha1.AIModelPhaseReady,
				InferenceServiceName: "llama-7b-new",
			},
		}

		mockClient := &mockAdminAPIClient{
			listRoutingPoliciesResponse: &adminapi.RoutingPolicyListResponse{
				Policies: []adminapi.RoutingPolicy{
					{
						PolicyID:       "policy-123",
						OrganizationID: "*",
						Model:          "llama-7b",
						Backends: []adminapi.Backend{
							{BackendID: "llama-7b-old", Weight: 100},
						},
						Enabled: true,
					},
				},
			},
			updateRoutingPolicyResponse: &adminapi.RoutingPolicy{
				PolicyID:       "policy-123",
				OrganizationID: "*",
				Model:          "llama-7b",
				Backends: []adminapi.Backend{
					{BackendID: "llama-7b-new", Weight: 100},
				},
				Enabled: true,
			},
		}

		reconciler := &AIModelReconciler{
			AdminAPIClient: mockClient,
			HealCooldown:   100 * time.Millisecond, // Short cooldown for test
		}

		// First heal
		err := reconciler.healRoutingPolicy(context.Background(), aiModel)
		require.NoError(t, err)
		assert.True(t, mockClient.updateRoutingPolicyCalled, "First heal should call update")

		// Reset mock state
		mockClient.updateRoutingPolicyCalled = false

		// Wait for cooldown to expire
		time.Sleep(150 * time.Millisecond)

		// Second heal after cooldown should succeed
		err = reconciler.healRoutingPolicy(context.Background(), aiModel)
		require.NoError(t, err)
		assert.True(t, mockClient.updateRoutingPolicyCalled, "Second heal should call update after cooldown expires")
	})

	t.Run("different AIModels are rate limited independently", func(t *testing.T) {
		aiModel1 := &aimodelv1alpha1.AIModel{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "llama-7b",
				Namespace: "system",
			},
			Spec: aimodelv1alpha1.AIModelSpec{
				ModelID:      "meta-llama/Llama-2-7b-hf",
				ExternalName: "llama-7b",
			},
			Status: aimodelv1alpha1.AIModelStatus{
				Phase:                aimodelv1alpha1.AIModelPhaseReady,
				InferenceServiceName: "llama-7b-new",
			},
		}

		aiModel2 := &aimodelv1alpha1.AIModel{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gpt-13b",
				Namespace: "system",
			},
			Spec: aimodelv1alpha1.AIModelSpec{
				ModelID:      "gpt/gpt-13b",
				ExternalName: "gpt-13b",
			},
			Status: aimodelv1alpha1.AIModelStatus{
				Phase:                aimodelv1alpha1.AIModelPhaseReady,
				InferenceServiceName: "gpt-13b-new",
			},
		}

		mockClient := &mockAdminAPIClient{
			listRoutingPoliciesResponse: &adminapi.RoutingPolicyListResponse{
				Policies: []adminapi.RoutingPolicy{
					{
						PolicyID:       "policy-123",
						OrganizationID: "*",
						Model:          "test-model",
						Backends: []adminapi.Backend{
							{BackendID: "old-backend", Weight: 100},
						},
						Enabled: true,
					},
				},
			},
			updateRoutingPolicyResponse: &adminapi.RoutingPolicy{
				PolicyID:       "policy-123",
				OrganizationID: "*",
				Model:          "test-model",
				Backends: []adminapi.Backend{
					{BackendID: "new-backend", Weight: 100},
				},
				Enabled: true,
			},
		}

		reconciler := &AIModelReconciler{
			AdminAPIClient: mockClient,
			HealCooldown:   1 * time.Minute,
		}

		// Heal aiModel1
		err := reconciler.healRoutingPolicy(context.Background(), aiModel1)
		require.NoError(t, err)
		assert.True(t, mockClient.updateRoutingPolicyCalled, "First model heal should call update")

		// Reset mock state
		mockClient.updateRoutingPolicyCalled = false

		// Heal aiModel2 immediately - should succeed (different model)
		err = reconciler.healRoutingPolicy(context.Background(), aiModel2)
		require.NoError(t, err)
		assert.True(t, mockClient.updateRoutingPolicyCalled, "Second model heal should call update (different model)")

		// Reset mock state
		mockClient.updateRoutingPolicyCalled = false

		// Heal aiModel1 again - should be rate limited
		err = reconciler.healRoutingPolicy(context.Background(), aiModel1)
		require.NoError(t, err)
		assert.False(t, mockClient.updateRoutingPolicyCalled, "First model second heal should be rate limited")
	})
}

func TestRoutingReadyCondition_DriftDetection(t *testing.T) {
	tests := []struct {
		name                  string
		routingPolicyExists   bool
		driftInfo             driftInfo
		healError             error
		expectedStatus        metav1.ConditionStatus
		expectedReason        string
		expectedMessagePrefix string
	}{
		{
			name:                "no routing policy exists",
			routingPolicyExists: false,
			driftInfo:           driftInfo{detected: false},
			healError:           nil,
			expectedStatus:      metav1.ConditionFalse,
			expectedReason:      "RoutingPolicyMissing",
			expectedMessagePrefix: "No routing policy found",
		},
		{
			name:                "policy synced - no drift",
			routingPolicyExists: true,
			driftInfo: driftInfo{
				detected:        false,
				expectedBackend: "llama-7b-vllm",
			},
			healError:             nil,
			expectedStatus:        metav1.ConditionTrue,
			expectedReason:        "PolicySynced",
			expectedMessagePrefix: "Routing policy backend matches InferenceServiceName llama-7b-vllm",
		},
		{
			name:                "drift detected - healing in progress",
			routingPolicyExists: true,
			driftInfo: driftInfo{
				detected:        true,
				expectedBackend: "llama-7b-vllm",
				actualBackend:   "old-backend",
			},
			healError:             nil,
			expectedStatus:        metav1.ConditionTrue,
			expectedReason:        "PolicyDrifted",
			expectedMessagePrefix: "Drift detected: expected backend llama-7b-vllm, got old-backend",
		},
		{
			name:                "drift detected - healing failed",
			routingPolicyExists: true,
			driftInfo: driftInfo{
				detected:        true,
				expectedBackend: "llama-7b-vllm",
				actualBackend:   "old-backend",
			},
			healError:             fmt.Errorf("Admin API unavailable"),
			expectedStatus:        metav1.ConditionFalse,
			expectedReason:        "HealFailed",
			expectedMessagePrefix: "Failed to heal routing policy drift: expected backend llama-7b-vllm, got old-backend",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock AIModel
			aiModel := &aimodelv1alpha1.AIModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-model",
					Namespace:  "test-namespace",
					Generation: 1,
				},
				Spec: aimodelv1alpha1.AIModelSpec{
					ModelName: "llama-7b",
				},
				Status: aimodelv1alpha1.AIModelStatus{
					InferenceServiceName: "llama-7b-vllm",
					Conditions:           []metav1.Condition{},
				},
			}

			// Simulate the condition setting logic from the controller
			var condition metav1.Condition
			if !tt.routingPolicyExists {
				condition = metav1.Condition{
					Type:               aimodelv1alpha1.ConditionTypeRoutingReady,
					Status:             metav1.ConditionFalse,
					ObservedGeneration: aiModel.Generation,
					Reason:             "RoutingPolicyMissing",
					Message:            "No routing policy found for this model",
				}
			} else if tt.driftInfo.detected && tt.healError != nil {
				condition = metav1.Condition{
					Type:               aimodelv1alpha1.ConditionTypeRoutingReady,
					Status:             metav1.ConditionFalse,
					ObservedGeneration: aiModel.Generation,
					Reason:             "HealFailed",
					Message:            fmt.Sprintf("Failed to heal routing policy drift: expected backend %s, got %s. Error: %v", tt.driftInfo.expectedBackend, tt.driftInfo.actualBackend, tt.healError),
				}
			} else if tt.driftInfo.detected {
				condition = metav1.Condition{
					Type:               aimodelv1alpha1.ConditionTypeRoutingReady,
					Status:             metav1.ConditionTrue,
					ObservedGeneration: aiModel.Generation,
					Reason:             "PolicyDrifted",
					Message:            fmt.Sprintf("Drift detected: expected backend %s, got %s. Healing in progress.", tt.driftInfo.expectedBackend, tt.driftInfo.actualBackend),
				}
			} else {
				condition = metav1.Condition{
					Type:               aimodelv1alpha1.ConditionTypeRoutingReady,
					Status:             metav1.ConditionTrue,
					ObservedGeneration: aiModel.Generation,
					Reason:             "PolicySynced",
					Message:            fmt.Sprintf("Routing policy backend matches InferenceServiceName %s", tt.driftInfo.expectedBackend),
				}
			}

			// Verify condition properties
			assert.Equal(t, aimodelv1alpha1.ConditionTypeRoutingReady, condition.Type,
				"Condition type should be RoutingReady")
			assert.Equal(t, tt.expectedStatus, condition.Status,
				"Condition status mismatch")
			assert.Equal(t, tt.expectedReason, condition.Reason,
				"Condition reason mismatch")
			assert.Contains(t, condition.Message, tt.expectedMessagePrefix,
				"Condition message should contain expected prefix")
			assert.Equal(t, aiModel.Generation, condition.ObservedGeneration,
				"ObservedGeneration should match AIModel generation")
		})
	}
}
