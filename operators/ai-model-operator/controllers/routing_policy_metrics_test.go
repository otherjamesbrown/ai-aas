package controllers

import (
	"context"
	"testing"

	"github.com/ai-aas/ai-model-operator/internal/adminapi"
	aimodelv1alpha1 "github.com/ai-aas/ai-model-operator/api/v1alpha1"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// getCounterValue retrieves the current value of a counter metric
func getCounterValue(counter *prometheus.CounterVec, labels prometheus.Labels) (float64, error) {
	metric := &dto.Metric{}
	if err := counter.With(labels).Write(metric); err != nil {
		return 0, err
	}
	return metric.Counter.GetValue(), nil
}

func TestRoutingPolicyDriftMetrics(t *testing.T) {
	// Reset metrics before test
	routingPolicyDriftDetectedTotal.Reset()

	tests := []struct {
		name             string
		aiModel          *aimodelv1alpha1.AIModel
		mockListResponse *adminapi.RoutingPolicyListResponse
		expectedDrift    bool
		expectMetricInc  bool
	}{
		{
			name: "increments drift_detected_total when drift is detected",
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
			expectedDrift:   true,
			expectMetricInc: true,
		},
		{
			name: "does not increment metric when no drift",
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
					InferenceServiceName: "llama-7b", // Matches backend ID
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
			expectedDrift:   false,
			expectMetricInc: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock Admin API client
			mockClient := &mockAdminAPIClient{
				listRoutingPoliciesResponse: tt.mockListResponse,
			}
			reconciler := &AIModelReconciler{
				AdminAPIClient: mockClient,
			}

			// Get initial counter value
			initialValue, err := getCounterValue(
				routingPolicyDriftDetectedTotal,
				prometheus.Labels{
					"model_name": "llama-7b",
					"namespace":  "system",
				},
			)
			require.NoError(t, err)

			// Execute
			driftDetected := reconciler.detectRoutingPolicyDrift(context.Background(), tt.aiModel)

			// Verify drift detection result
			assert.Equal(t, tt.expectedDrift, driftDetected,
				"Drift detection result mismatch")

			// Get final counter value
			finalValue, err := getCounterValue(
				routingPolicyDriftDetectedTotal,
				prometheus.Labels{
					"model_name": "llama-7b",
					"namespace":  "system",
				},
			)
			require.NoError(t, err)

			// Verify metric increment
			if tt.expectMetricInc {
				assert.Greater(t, finalValue, initialValue,
					"Expected drift_detected_total to increment")
				assert.Equal(t, initialValue+1, finalValue,
					"Expected drift_detected_total to increment by 1")
			} else {
				assert.Equal(t, initialValue, finalValue,
					"Expected drift_detected_total to NOT increment")
			}
		})
	}
}

func TestRoutingPolicyMetricsRegistration(t *testing.T) {
	// Test that all routing policy metrics are registered
	// This is a smoke test to ensure the metrics are properly initialized
	tests := []struct {
		name   string
		metric prometheus.Collector
	}{
		{
			name:   "routing_policy_drift_detected_total",
			metric: routingPolicyDriftDetectedTotal,
		},
		{
			name:   "routing_policy_drift_duration_seconds",
			metric: routingPolicyDriftDuration,
		},
		{
			name:   "routing_policy_heal_attempts_total",
			metric: routingPolicyHealAttemptsTotal,
		},
		{
			name:   "routing_policy_heal_duration_seconds",
			metric: routingPolicyHealDuration,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify the metric is not nil
			assert.NotNil(t, tt.metric, "Metric should be initialized")

			// Verify the metric can be described (proves registration worked)
			descCh := make(chan *prometheus.Desc, 1)
			tt.metric.Describe(descCh)
			close(descCh)

			// Should have at least one description
			desc := <-descCh
			assert.NotNil(t, desc, "Metric should have a description")
		})
	}
}
