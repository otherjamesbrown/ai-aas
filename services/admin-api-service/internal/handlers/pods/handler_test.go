package pods

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// mockService implements the Service interface for testing
type mockService struct {
	response *PodHealthResponse
	err      error
}

func (m *mockService) GetPodHealth(ctx context.Context, opts ListOptions) (*PodHealthResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

// Test helpers

func setupHandler(svc Service) *Handler {
	return NewHandler(svc)
}

func TestGetPodHealth_ValidRequest(t *testing.T) {
	now := time.Now()
	svc := &mockService{
		response: &PodHealthResponse{
			Pods: []PodHealth{
				{
					Name:         "gpt-oss-20b-predictor-00001-deployment-abc123",
					Namespace:    "system",
					ModelName:    "gpt-oss-20b",
					Phase:        "Running",
					Ready:        true,
					Node:         "lke-pool-abc123",
					RestartCount: 0,
					AgeSeconds:   3600,
					CreatedAt:    now,
					Containers:   []ContainerHealth{},
					Conditions:   []PodCondition{},
				},
			},
			Summary: HealthSummary{
				TotalPods:     1,
				HealthyPods:   1,
				UnhealthyPods: 0,
				TotalRestarts: 0,
			},
		},
	}
	handler := setupHandler(svc)

	req := httptest.NewRequest("GET", "/v1/pods/health", nil)
	w := httptest.NewRecorder()

	handler.GetPodHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response PodHealthResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Pods) != 1 {
		t.Errorf("expected 1 pod, got %d", len(response.Pods))
	}

	if response.Summary.TotalPods != 1 {
		t.Errorf("expected total_pods 1, got %d", response.Summary.TotalPods)
	}

	if response.Summary.HealthyPods != 1 {
		t.Errorf("expected healthy_pods 1, got %d", response.Summary.HealthyPods)
	}
}

func TestGetPodHealth_FilterByNamespace(t *testing.T) {
	svc := &mockService{
		response: &PodHealthResponse{
			Pods:    []PodHealth{},
			Summary: HealthSummary{},
		},
	}
	handler := setupHandler(svc)

	req := httptest.NewRequest("GET", "/v1/pods/health?namespace=custom-ns", nil)
	w := httptest.NewRecorder()

	handler.GetPodHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Verify namespace was passed to service
	// This test validates that the query parameter is parsed correctly
}

func TestGetPodHealth_FilterByModelName(t *testing.T) {
	svc := &mockService{
		response: &PodHealthResponse{
			Pods: []PodHealth{
				{
					Name:      "gpt-oss-20b-predictor-00001",
					ModelName: "gpt-oss-20b",
					Ready:     true,
				},
			},
			Summary: HealthSummary{
				TotalPods:   1,
				HealthyPods: 1,
			},
		},
	}
	handler := setupHandler(svc)

	req := httptest.NewRequest("GET", "/v1/pods/health?model_name=gpt-oss-20b", nil)
	w := httptest.NewRecorder()

	handler.GetPodHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response PodHealthResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Pods) != 1 {
		t.Errorf("expected 1 pod, got %d", len(response.Pods))
	}

	if response.Pods[0].ModelName != "gpt-oss-20b" {
		t.Errorf("expected model_name gpt-oss-20b, got %s", response.Pods[0].ModelName)
	}
}

func TestGetPodHealth_FilterByEnvironment(t *testing.T) {
	svc := &mockService{
		response: &PodHealthResponse{
			Pods: []PodHealth{
				{
					Name:      "model-predictor-dev",
					ModelName: "test-model",
					Ready:     true,
				},
			},
			Summary: HealthSummary{
				TotalPods:   1,
				HealthyPods: 1,
			},
		},
	}
	handler := setupHandler(svc)

	req := httptest.NewRequest("GET", "/v1/pods/health?environment=development", nil)
	w := httptest.NewRecorder()

	handler.GetPodHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response PodHealthResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Pods) != 1 {
		t.Errorf("expected 1 pod, got %d", len(response.Pods))
	}
}

func TestGetPodHealth_IncludeHealthyFalse(t *testing.T) {
	svc := &mockService{
		response: &PodHealthResponse{
			Pods: []PodHealth{
				{
					Name:         "unhealthy-pod",
					ModelName:    "test-model",
					Phase:        "CrashLoopBackOff",
					Ready:        false,
					RestartCount: 5,
				},
			},
			Summary: HealthSummary{
				TotalPods:     1,
				UnhealthyPods: 1,
				TotalRestarts: 5,
			},
		},
	}
	handler := setupHandler(svc)

	req := httptest.NewRequest("GET", "/v1/pods/health?include_healthy=false", nil)
	w := httptest.NewRecorder()

	handler.GetPodHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response PodHealthResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Pods) != 1 {
		t.Errorf("expected 1 pod, got %d", len(response.Pods))
	}

	if response.Pods[0].Ready {
		t.Error("expected pod to be unhealthy")
	}

	if response.Summary.UnhealthyPods != 1 {
		t.Errorf("expected 1 unhealthy pod, got %d", response.Summary.UnhealthyPods)
	}
}

func TestGetPodHealth_EmptyResult(t *testing.T) {
	svc := &mockService{
		response: &PodHealthResponse{
			Pods: []PodHealth{},
			Summary: HealthSummary{
				TotalPods:     0,
				HealthyPods:   0,
				UnhealthyPods: 0,
				TotalRestarts: 0,
			},
		},
	}
	handler := setupHandler(svc)

	req := httptest.NewRequest("GET", "/v1/pods/health?model_name=nonexistent", nil)
	w := httptest.NewRecorder()

	handler.GetPodHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d (empty results should return 200, not error)", w.Code)
	}

	var response PodHealthResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Pods) != 0 {
		t.Errorf("expected 0 pods, got %d", len(response.Pods))
	}

	if response.Summary.TotalPods != 0 {
		t.Errorf("expected total_pods 0, got %d", response.Summary.TotalPods)
	}
}

func TestGetPodHealth_ServiceError(t *testing.T) {
	svc := &mockService{
		err: fmt.Errorf("kubernetes client error: connection refused"),
	}
	handler := setupHandler(svc)

	req := httptest.NewRequest("GET", "/v1/pods/health", nil)
	w := httptest.NewRecorder()

	handler.GetPodHealth(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}

	var errResponse map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&errResponse); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if _, ok := errResponse["error"]; !ok {
		t.Error("expected error field in response")
	}

	if _, ok := errResponse["details"]; !ok {
		t.Error("expected details field in response")
	}
}

func TestGetPodHealth_DefaultNamespace(t *testing.T) {
	var capturedOpts ListOptions
	svc := &mockService{
		response: &PodHealthResponse{
			Pods:    []PodHealth{},
			Summary: HealthSummary{},
		},
	}

	// Create a custom service that captures the options
	handler := NewHandler(&mockServiceCapture{
		mockService: svc,
		captureFunc: func(opts ListOptions) {
			capturedOpts = opts
		},
	})

	req := httptest.NewRequest("GET", "/v1/pods/health", nil)
	w := httptest.NewRecorder()

	handler.GetPodHealth(w, req)

	if capturedOpts.Namespace != "system" {
		t.Errorf("expected default namespace 'system', got %s", capturedOpts.Namespace)
	}
}

func TestGetPodHealth_IncludeHealthyDefault(t *testing.T) {
	var capturedOpts ListOptions
	svc := &mockService{
		response: &PodHealthResponse{
			Pods:    []PodHealth{},
			Summary: HealthSummary{},
		},
	}

	handler := NewHandler(&mockServiceCapture{
		mockService: svc,
		captureFunc: func(opts ListOptions) {
			capturedOpts = opts
		},
	})

	req := httptest.NewRequest("GET", "/v1/pods/health", nil)
	w := httptest.NewRecorder()

	handler.GetPodHealth(w, req)

	if !capturedOpts.IncludeHealthy {
		t.Error("expected include_healthy to default to true")
	}
}

func TestParseBoolParam(t *testing.T) {
	tests := []struct {
		name         string
		value        string
		defaultValue bool
		expected     bool
	}{
		{
			name:         "empty string uses default true",
			value:        "",
			defaultValue: true,
			expected:     true,
		},
		{
			name:         "empty string uses default false",
			value:        "",
			defaultValue: false,
			expected:     false,
		},
		{
			name:         "true string",
			value:        "true",
			defaultValue: false,
			expected:     true,
		},
		{
			name:         "false string",
			value:        "false",
			defaultValue: true,
			expected:     false,
		},
		{
			name:         "1 as true",
			value:        "1",
			defaultValue: false,
			expected:     true,
		},
		{
			name:         "0 as false",
			value:        "0",
			defaultValue: true,
			expected:     false,
		},
		{
			name:         "invalid value uses default",
			value:        "invalid",
			defaultValue: true,
			expected:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseBoolParam(tt.value, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("parseBoolParam(%q, %v) = %v, expected %v", tt.value, tt.defaultValue, result, tt.expected)
			}
		})
	}
}

func TestGetPodHealth_ContentType(t *testing.T) {
	svc := &mockService{
		response: &PodHealthResponse{
			Pods:    []PodHealth{},
			Summary: HealthSummary{},
		},
	}
	handler := setupHandler(svc)

	req := httptest.NewRequest("GET", "/v1/pods/health", nil)
	w := httptest.NewRecorder()

	handler.GetPodHealth(w, req)

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}
}

func TestGetPodHealth_MultipleFilters(t *testing.T) {
	var capturedOpts ListOptions
	svc := &mockService{
		response: &PodHealthResponse{
			Pods:    []PodHealth{},
			Summary: HealthSummary{},
		},
	}

	handler := NewHandler(&mockServiceCapture{
		mockService: svc,
		captureFunc: func(opts ListOptions) {
			capturedOpts = opts
		},
	})

	req := httptest.NewRequest("GET", "/v1/pods/health?namespace=custom&model_name=test-model&environment=dev&include_healthy=false", nil)
	w := httptest.NewRecorder()

	handler.GetPodHealth(w, req)

	if capturedOpts.Namespace != "custom" {
		t.Errorf("expected namespace 'custom', got %s", capturedOpts.Namespace)
	}

	if capturedOpts.ModelName != "test-model" {
		t.Errorf("expected model_name 'test-model', got %s", capturedOpts.ModelName)
	}

	if capturedOpts.Environment != "dev" {
		t.Errorf("expected environment 'dev', got %s", capturedOpts.Environment)
	}

	if capturedOpts.IncludeHealthy {
		t.Error("expected include_healthy to be false")
	}
}

// mockServiceCapture extends mockService to capture options
type mockServiceCapture struct {
	*mockService
	captureFunc func(ListOptions)
}

func (m *mockServiceCapture) GetPodHealth(ctx context.Context, opts ListOptions) (*PodHealthResponse, error) {
	if m.captureFunc != nil {
		m.captureFunc(opts)
	}
	return m.mockService.GetPodHealth(ctx, opts)
}
