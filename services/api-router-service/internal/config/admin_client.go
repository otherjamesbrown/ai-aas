package config

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// HTTPAdminAPIClient implements AdminAPIClient using HTTP requests to Admin API.
type HTTPAdminAPIClient struct {
	endpoint   string
	apiKey     string
	httpClient *http.Client
	logger     *zap.Logger
}

// NewHTTPAdminAPIClient creates a new HTTP-based Admin API client.
func NewHTTPAdminAPIClient(endpoint, apiKey string, logger *zap.Logger) *HTTPAdminAPIClient {
	return &HTTPAdminAPIClient{
		endpoint: endpoint,
		apiKey:   apiKey,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		logger: logger,
	}
}

// adminAPIModel represents the model structure returned by Admin API /v1/models.
type adminAPIModel struct {
	Name             string  `json:"name"`
	ExternalName     string  `json:"external_name,omitempty"`
	HFModelID        string  `json:"hf_model_id"`
	CacheStatus      *string `json:"cache_status"`
	DeploymentStatus *string `json:"deployment_status"`
	InferenceService string  `json:"inference_service,omitempty"`
	ModelType        string  `json:"model_type,omitempty"`
}

// GetModelDeployment retrieves deployment information for a specific model.
// Returns nil if the model is not found or Admin API is unavailable.
func (c *HTTPAdminAPIClient) GetModelDeployment(ctx context.Context, modelName string) (*ModelDeploymentInfo, error) {
	if c.endpoint == "" {
		return nil, fmt.Errorf("admin API endpoint not configured")
	}

	// Fetch all models from Admin API
	// Note: Admin API doesn't have a single-model GET endpoint, so we fetch all and filter
	url := fmt.Sprintf("%s/v1/models", c.endpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Add admin API key for authentication
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("admin api returned %d: %s", resp.StatusCode, string(body))
	}

	// Admin API returns an array directly, not wrapped in an object
	var models []adminAPIModel
	if err := json.NewDecoder(resp.Body).Decode(&models); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Find the requested model
	for _, model := range models {
		if model.Name == modelName || model.ExternalName == modelName {
			inferenceService := model.InferenceService
			if inferenceService == "" {
				// Fallback: construct service name from model name
				// KServe creates services as: <model-name>-predictor.<namespace>.svc.cluster.local
				inferenceService = model.Name + "-predictor"
			}

			return &ModelDeploymentInfo{
				Name:             model.Name,
				DeploymentStatus: model.DeploymentStatus,
				InferenceService: inferenceService,
			}, nil
		}
	}

	// Model not found
	return nil, fmt.Errorf("model %q not found in registry", modelName)
}
