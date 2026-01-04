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
