// Package public provides OpenAI-compatible API handlers.
//
// This file implements OpenAI-compatible endpoints (/v1/chat/completions, /v1/completions)
// that proxy to backend inference services while maintaining OpenAI API compatibility.

package public

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/adapter/triton"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/auth"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/routing"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/telemetry"
)

// OpenAIChatCompletionRequest represents an OpenAI chat completions API request.
type OpenAIChatCompletionRequest struct {
	Model       string                 `json:"model"`
	Messages    []OpenAIMessage        `json:"messages"`
	MaxTokens   *int                   `json:"max_tokens,omitempty"`
	Temperature *float64               `json:"temperature,omitempty"`
	Stream      bool                   `json:"stream,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// OpenAIMessage represents a message in an OpenAI chat conversation.
type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAIChatCompletionResponse represents an OpenAI chat completions API response.
type OpenAIChatCompletionResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []OpenAIChoice         `json:"choices"`
	Usage   OpenAIUsage            `json:"usage"`
}

// OpenAIChoice represents a completion choice in an OpenAI response.
type OpenAIChoice struct {
	Index        int             `json:"index"`
	Message      OpenAIMessage   `json:"message"`
	FinishReason string          `json:"finish_reason"`
}

// OpenAIUsage represents token usage information in an OpenAI response.
type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// OpenAICompletionRequest represents an OpenAI text completions API request.
type OpenAICompletionRequest struct {
	Model       string                 `json:"model"`
	Prompt      string                 `json:"prompt"`
	MaxTokens   int                    `json:"max_tokens,omitempty"`
	Temperature float64                `json:"temperature,omitempty"`
	Stream      bool                   `json:"stream,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// OpenAICompletionResponse represents an OpenAI text completions API response.
type OpenAICompletionResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []OpenAICompletionChoice `json:"choices"`
	Usage   OpenAIUsage             `json:"usage"`
}

// OpenAICompletionChoice represents a text completion choice in an OpenAI response.
type OpenAICompletionChoice struct {
	Text         string `json:"text"`
	Index        int    `json:"index"`
	FinishReason string `json:"finish_reason"`
}

// HandleOpenAIChatCompletions handles POST /v1/chat/completions (OpenAI-compatible)
func (h *Handler) HandleOpenAIChatCompletions(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.tracer.Start(r.Context(), "openai.chat_completions")
	defer span.End()

	startTime := time.Now()

	// Get authenticated context from middleware
	authCtxValue := r.Context().Value(AuthContextKey)
	if authCtxValue == nil {
		h.writeError(w, r, fmt.Errorf("authentication required"), api.ErrCodeAuthInvalid)
		return
	}

	authCtx, ok := authCtxValue.(*auth.AuthenticatedContext)
	if !ok {
		h.writeError(w, r, fmt.Errorf("invalid authentication context"), api.ErrCodeAuthInvalid)
		return
	}

	// Parse OpenAI request
	var openAIReq OpenAIChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&openAIReq); err != nil {
		h.writeError(w, r, fmt.Errorf("invalid request body: %w", err), api.ErrCodeInvalidRequest)
		return
	}

	// Validate request
	if openAIReq.Model == "" {
		h.writeError(w, r, fmt.Errorf("model is required"), api.ErrCodeValidationError)
		return
	}
	if len(openAIReq.Messages) == 0 {
		h.writeError(w, r, fmt.Errorf("messages array cannot be empty"), api.ErrCodeValidationError)
		return
	}

	// Get routing policy
	policy, err := h.configLoader.GetPolicy(authCtx.OrganizationID, openAIReq.Model)
	if err != nil {
		h.logger.Warn("no routing policy found",
			zap.String("org_id", authCtx.OrganizationID),
			zap.String("model", openAIReq.Model),
		)
		h.writeError(w, r, fmt.Errorf("no routing policy configured"), api.ErrCodeRoutingError)
		return
	}

	// Validate that at least one backend is configured (PR#16 Issue#1)
	if len(policy.Backends) == 0 {
		h.writeError(w, r, fmt.Errorf("no backends configured for model %q", openAIReq.Model), api.ErrCodeRoutingError)
		return
	}

	// Route based on backend type (spec032 - Triton API Support)
	switch policy.BackendType {
	case "triton":
		h.handleTritonChatCompletion(ctx, w, r, policy, &openAIReq, authCtx, startTime)
		return
	default: // "openai" or empty - use existing vLLM/OpenAI flow
	}

	// Forward OpenAI request directly to backend's OpenAI endpoint
	backendID := policy.Backends[0].BackendID
	backendEndpoint := h.buildBackendEndpointForOpenAI(backendID, openAIReq.Model, "/v1/chat/completions")

	// Rewrite model name to match backend's expected model ID
	// User sends alias (e.g., "gpt-oss-20b"), backend expects HuggingFace ID (e.g., "unsloth/gpt-oss-20b")
	originalModel := openAIReq.Model
	openAIReq.Model = backendID
	h.logger.Debug("rewriting model name for backend",
		zap.String("original_model", originalModel),
		zap.String("backend_model", backendID),
	)

	// Forward the OpenAI request to the backend
	openAIRespInterface, routingDecision, err := h.forwardOpenAIRequest(ctx, backendEndpoint, openAIReq, "chat")
	if err != nil {
		h.writeError(w, r, fmt.Errorf("backend request failed: %w", err), api.ErrCodeBackendError)
		return
	}

	openAIResp, ok := openAIRespInterface.(OpenAIChatCompletionResponse)
	if !ok {
		h.writeError(w, r, fmt.Errorf("invalid response type"), api.ErrCodeBackendError)
		return
	}

	// Add routing headers
	if routingDecision != nil {
		w.Header().Set("X-Routing-Backend", routingDecision.BackendID)
		w.Header().Set("X-Routing-Decision", routingDecision.DecisionType)
	}

	// Emit usage record
	if h.usageHook != nil && routingDecision != nil {
		promptTokens := openAIResp.Usage.PromptTokens
		completionTokens := openAIResp.Usage.CompletionTokens
		_ = h.usageHook.EmitUsage(
			ctx,
			authCtx,
			openAIResp.ID,
			openAIReq.Model,
			routingDecision.BackendID,
			routingDecision.DecisionType,
			promptTokens,
			completionTokens,
			int(time.Since(startTime).Milliseconds()),
			"WITHIN_LIMIT",
			span.SpanContext(),
			routingDecision.AttemptNumber-1,
		)
	}

	// Record token metrics
	if h.tokenMetrics != nil {
		h.tokenMetrics.RecordTokens(
			ctx,
			authCtx.OrganizationID,
			originalModel, // Use original model name (alias)
			"chat",
			openAIResp.Usage.PromptTokens,
			openAIResp.Usage.CompletionTokens,
		)
	}

	// Record per-backend Prometheus metrics for dashboard visibility
	if routingDecision != nil {
		requestLatency := time.Since(startTime)
		telemetry.RecordBackendRequest(
			routingDecision.BackendID,
			authCtx.OrganizationID,
			originalModel,
			true, // success
			requestLatency,
		)
	}

	// Write response
	if err := h.writeJSON(w, http.StatusOK, openAIResp); err != nil {
		h.logger.Error("failed to write OpenAI response", zap.Error(err))
	}
}

// HandleOpenAICompletions handles POST /v1/completions (OpenAI-compatible)
func (h *Handler) HandleOpenAICompletions(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.tracer.Start(r.Context(), "openai.completions")
	defer span.End()

	startTime := time.Now()

	// Get authenticated context from middleware
	authCtxValue := r.Context().Value(AuthContextKey)
	if authCtxValue == nil {
		h.writeError(w, r, fmt.Errorf("authentication required"), api.ErrCodeAuthInvalid)
		return
	}

	authCtx, ok := authCtxValue.(*auth.AuthenticatedContext)
	if !ok {
		h.writeError(w, r, fmt.Errorf("invalid authentication context"), api.ErrCodeAuthInvalid)
		return
	}

	// Parse OpenAI request
	var openAIReq OpenAICompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&openAIReq); err != nil {
		h.writeError(w, r, fmt.Errorf("invalid request body: %w", err), api.ErrCodeInvalidRequest)
		return
	}

	// Validate request
	if openAIReq.Model == "" {
		h.writeError(w, r, fmt.Errorf("model is required"), api.ErrCodeValidationError)
		return
	}
	if openAIReq.Prompt == "" {
		h.writeError(w, r, fmt.Errorf("prompt is required"), api.ErrCodeValidationError)
		return
	}

	// Get routing policy
	policy, err := h.configLoader.GetPolicy(authCtx.OrganizationID, openAIReq.Model)
	if err != nil {
		h.logger.Warn("no routing policy found",
			zap.String("org_id", authCtx.OrganizationID),
			zap.String("model", openAIReq.Model),
		)
		h.writeError(w, r, fmt.Errorf("no routing policy configured"), api.ErrCodeRoutingError)
		return
	}

	// Validate that at least one backend is configured (PR#16 Issue#2)
	if len(policy.Backends) == 0 {
		h.writeError(w, r, fmt.Errorf("no backends configured for model %q", openAIReq.Model), api.ErrCodeRoutingError)
		return
	}

	// Forward OpenAI request directly to backend's OpenAI endpoint
	backendID := policy.Backends[0].BackendID
	backendEndpoint := h.buildBackendEndpointForOpenAI(backendID, openAIReq.Model, "/v1/completions")

	// Rewrite model name to match backend's expected model ID
	// User sends alias (e.g., "gpt-oss-20b"), backend expects HuggingFace ID (e.g., "unsloth/gpt-oss-20b")
	originalModel := openAIReq.Model
	openAIReq.Model = backendID
	h.logger.Debug("rewriting model name for backend",
		zap.String("original_model", originalModel),
		zap.String("backend_model", backendID),
	)

	// Forward the OpenAI request to the backend
	openAIRespInterface, routingDecision, err := h.forwardOpenAIRequest(ctx, backendEndpoint, openAIReq, "completion")
	if err != nil {
		h.writeError(w, r, fmt.Errorf("backend request failed: %w", err), api.ErrCodeBackendError)
		return
	}

	openAIResp, ok := openAIRespInterface.(OpenAICompletionResponse)
	if !ok {
		h.writeError(w, r, fmt.Errorf("invalid response type"), api.ErrCodeBackendError)
		return
	}

	// Add routing headers
	if routingDecision != nil {
		w.Header().Set("X-Routing-Backend", routingDecision.BackendID)
		w.Header().Set("X-Routing-Decision", routingDecision.DecisionType)
	}

	// Emit usage record
	if h.usageHook != nil && routingDecision != nil {
		promptTokens := openAIResp.Usage.PromptTokens
		completionTokens := openAIResp.Usage.CompletionTokens
		_ = h.usageHook.EmitUsage(
			ctx,
			authCtx,
			openAIResp.ID,
			openAIReq.Model,
			routingDecision.BackendID,
			routingDecision.DecisionType,
			promptTokens,
			completionTokens,
			int(time.Since(startTime).Milliseconds()),
			"WITHIN_LIMIT",
			span.SpanContext(),
			routingDecision.AttemptNumber-1,
		)
	}

	// Record token metrics
	if h.tokenMetrics != nil {
		h.tokenMetrics.RecordTokens(
			ctx,
			authCtx.OrganizationID,
			originalModel, // Use original model name (alias)
			"completion",
			openAIResp.Usage.PromptTokens,
			openAIResp.Usage.CompletionTokens,
		)
	}

	// Record per-backend Prometheus metrics for dashboard visibility
	if routingDecision != nil {
		requestLatency := time.Since(startTime)
		telemetry.RecordBackendRequest(
			routingDecision.BackendID,
			authCtx.OrganizationID,
			originalModel,
			true, // success
			requestLatency,
		)
	}

	// Write response
	if err := h.writeJSON(w, http.StatusOK, openAIResp); err != nil {
		h.logger.Error("failed to write OpenAI response", zap.Error(err))
	}
}

// buildBackendEndpointForOpenAI constructs a backend endpoint for OpenAI-compatible requests
// Uses net/url for safe URL manipulation (PR#16 Issue#3)
func (h *Handler) buildBackendEndpointForOpenAI(backendID, model, path string) *routing.BackendEndpoint {
	baseEndpoint := h.buildBackendEndpoint(backendID, model)

	// Parse the backend URI using net/url for safe manipulation
	parsedURI, err := url.Parse(baseEndpoint.URI)
	if err != nil {
		h.logger.Error("failed to parse backend URI",
			zap.String("uri", baseEndpoint.URI),
			zap.Error(err),
		)
		return baseEndpoint // Fallback to original on parse error
	}

	// Replace the path with the OpenAI endpoint path
	parsedURI.Path = path
	baseEndpoint.URI = parsedURI.String()

	return baseEndpoint
}

// forwardOpenAIRequest forwards an OpenAI-format request to the backend and returns OpenAI-format response
func (h *Handler) forwardOpenAIRequest(ctx context.Context, backend *routing.BackendEndpoint, req interface{}, reqType string) (interface{}, *routing.RoutingDecision, error) {
	// Marshal the OpenAI request
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal OpenAI request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", backend.URI, bytes.NewReader(reqBody))
	if err != nil {
		return nil, nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Use shared HTTP client with context-based timeout (PR#16 Issue#4)
	reqCtx, cancel := context.WithTimeout(ctx, backend.Timeout)
	defer cancel()

	resp, err := h.httpClient.Do(httpReq.WithContext(reqCtx))
	if err != nil {
		return nil, nil, fmt.Errorf("backend request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, nil, fmt.Errorf("backend returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse OpenAI response based on type
	var openAIResp interface{}
	if reqType == "chat" {
		var chatResp OpenAIChatCompletionResponse
		if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
			return nil, nil, fmt.Errorf("unmarshal OpenAI chat response: %w", err)
		}
		openAIResp = chatResp
	} else {
		var completionResp OpenAICompletionResponse
		if err := json.NewDecoder(resp.Body).Decode(&completionResp); err != nil {
			return nil, nil, fmt.Errorf("unmarshal OpenAI completion response: %w", err)
		}
		openAIResp = completionResp
	}

	decision := &routing.RoutingDecision{
		BackendID:     backend.ID,
		DecisionType:  "PRIMARY",
		Reason:        "OpenAI endpoint forwarding",
		Timestamp:     time.Now(),
		AttemptNumber: 1,
	}

	return openAIResp, decision, nil
}

// handleTritonChatCompletion handles chat completion requests for Triton backends.
// It translates OpenAI requests to Triton V2 protocol and responses back.
// Implements spec032 - Triton API Support.
func (h *Handler) handleTritonChatCompletion(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
	policy *config.RoutingPolicy,
	req *OpenAIChatCompletionRequest,
	authCtx *auth.AuthenticatedContext,
	startTime time.Time,
) {
	ctx, span := h.tracer.Start(ctx, "triton.chat_completions")
	defer span.End()

	// Check for streaming - not supported for Triton in MVP
	if req.Stream {
		h.writeError(w, r, fmt.Errorf("streaming is not supported for Triton backends"), api.ErrCodeValidationError)
		return
	}

	// Validate tokenizer is configured
	if policy.Tokenizer == "" {
		h.writeError(w, r, fmt.Errorf("tokenizer encoding is required for Triton backends"), api.ErrCodeValidationError)
		return
	}

	// Get or create translator for this tokenizer encoding
	translator, err := h.getOrCreateTranslator(policy.Tokenizer)
	if err != nil {
		h.logger.Error("failed to create triton translator",
			zap.String("tokenizer", policy.Tokenizer),
			zap.Error(err),
		)
		h.writeError(w, r, fmt.Errorf("internal configuration error"), api.ErrCodeBackendError)
		return
	}

	// Convert public types to triton adapter types
	tritonOpenAIReq := h.convertToTritonOpenAIRequest(req)

	// Translate OpenAI request to Triton format
	translateReqStart := time.Now()
	tritonReq, err := translator.TranslateOpenAIToTriton(tritonOpenAIReq)
	translateReqDuration := time.Since(translateReqStart)
	if err != nil {
		h.logger.Error("failed to translate request to triton format",
			zap.Error(err),
		)
		h.writeError(w, r, fmt.Errorf("request translation failed: %w", err), api.ErrCodeValidationError)
		return
	}

	// Build Triton backend endpoint
	backendID := policy.Backends[0].BackendID
	tritonEndpoint := h.buildTritonEndpoint(backendID, policy.Model)

	h.logger.Debug("forwarding to triton backend",
		zap.String("backend_id", backendID),
		zap.String("endpoint", tritonEndpoint),
		zap.String("model", policy.Model),
		zap.String("backend_type", "triton"),
	)

	// Serialize Triton request
	tritonReqBody, err := translator.SerializeTritonRequest(tritonReq)
	if err != nil {
		h.writeError(w, r, fmt.Errorf("request serialization failed: %w", err), api.ErrCodeBackendError)
		return
	}

	// Create HTTP request to Triton
	httpReq, err := http.NewRequestWithContext(ctx, "POST", tritonEndpoint, bytes.NewReader(tritonReqBody))
	if err != nil {
		h.writeError(w, r, fmt.Errorf("failed to create request: %w", err), api.ErrCodeBackendError)
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Get timeout from backend config
	backend := h.buildBackendEndpoint(backendID, policy.Model)
	reqCtx, cancel := context.WithTimeout(ctx, backend.Timeout)
	defer cancel()

	// Send request to Triton
	resp, err := h.httpClient.Do(httpReq.WithContext(reqCtx))
	if err != nil {
		h.logger.Error("triton request failed",
			zap.String("backend_id", backendID),
			zap.Error(err),
		)
		h.writeError(w, r, fmt.Errorf("backend request failed: %w", err), api.ErrCodeBackendError)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		h.writeError(w, r, fmt.Errorf("failed to read response: %w", err), api.ErrCodeBackendError)
		return
	}

	// Handle Triton errors
	if resp.StatusCode != http.StatusOK {
		openAIErr, httpStatus := triton.MapHTTPStatusToTritonError(resp.StatusCode, string(respBody))
		h.logger.Error("triton backend returned error",
			zap.Int("status", resp.StatusCode),
			zap.String("body", string(respBody)),
			zap.String("backend_id", backendID),
		)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(httpStatus)
		_ = json.NewEncoder(w).Encode(triton.OpenAIErrorResponse{Error: *openAIErr})
		return
	}

	// Parse Triton response
	tritonResp, err := translator.ParseTritonResponse(respBody)
	if err != nil {
		h.logger.Error("failed to parse triton response",
			zap.Error(err),
			zap.String("body", string(respBody)),
		)
		h.writeError(w, r, fmt.Errorf("failed to parse backend response: %w", err), api.ErrCodeBackendError)
		return
	}

	// Translate Triton response back to OpenAI format
	translateRespStart := time.Now()
	openAIResp, err := translator.TranslateTritonToOpenAI(tritonResp, tritonOpenAIReq)
	translateRespDuration := time.Since(translateRespStart)
	if err != nil {
		h.logger.Error("failed to translate triton response to openai format",
			zap.Error(err),
		)
		h.writeError(w, r, fmt.Errorf("response translation failed: %w", err), api.ErrCodeBackendError)
		return
	}

	// Create routing decision for metrics
	routingDecision := &routing.RoutingDecision{
		BackendID:     backendID,
		DecisionType:  "PRIMARY",
		Reason:        "Triton backend routing",
		Timestamp:     time.Now(),
		AttemptNumber: 1,
	}

	// Add routing headers
	w.Header().Set("X-Routing-Backend", routingDecision.BackendID)
	w.Header().Set("X-Routing-Decision", routingDecision.DecisionType)
	w.Header().Set("X-Backend-Type", "triton")

	// Emit usage record
	if h.usageHook != nil {
		_ = h.usageHook.EmitUsage(
			ctx,
			authCtx,
			openAIResp.ID,
			req.Model,
			routingDecision.BackendID,
			routingDecision.DecisionType,
			openAIResp.Usage.PromptTokens,
			openAIResp.Usage.CompletionTokens,
			int(time.Since(startTime).Milliseconds()),
			"WITHIN_LIMIT",
			span.SpanContext(),
			0,
		)
	}

	h.logger.Info("triton request completed",
		zap.String("backend_id", backendID),
		zap.String("backend_type", "triton"),
		zap.Int("prompt_tokens", openAIResp.Usage.PromptTokens),
		zap.Int("completion_tokens", openAIResp.Usage.CompletionTokens),
		zap.Duration("latency", time.Since(startTime)),
		zap.Duration("translation_request_ms", translateReqDuration),
		zap.Duration("translation_response_ms", translateRespDuration),
	)

	// Write response
	if err := h.writeJSON(w, http.StatusOK, openAIResp); err != nil {
		h.logger.Error("failed to write triton response", zap.Error(err))
	}
}

// convertToTritonOpenAIRequest converts the public OpenAI types to triton adapter types.
func (h *Handler) convertToTritonOpenAIRequest(req *OpenAIChatCompletionRequest) *triton.OpenAIChatCompletionRequest {
	// Convert messages
	messages := make([]triton.ChatMessage, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = triton.ChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	tritonReq := &triton.OpenAIChatCompletionRequest{
		Model:    req.Model,
		Messages: messages,
		Stream:   req.Stream,
	}

	// Copy optional fields (pointer types preserve explicit zero values)
	if req.MaxTokens != nil {
		maxTokens := *req.MaxTokens
		tritonReq.MaxTokens = &maxTokens
	}
	if req.Temperature != nil {
		temp := *req.Temperature
		tritonReq.Temperature = &temp
	}

	return tritonReq
}

// buildTritonEndpoint constructs the Triton V2 inference endpoint URL.
// Format: http://{backend_host}/v2/models/{model}/infer
func (h *Handler) buildTritonEndpoint(backendID, model string) string {
	backend := h.buildBackendEndpoint(backendID, model)

	// Parse the backend URI
	parsedURI, err := url.Parse(backend.URI)
	if err != nil {
		h.logger.Error("failed to parse backend URI for triton",
			zap.String("uri", backend.URI),
			zap.Error(err),
		)
		return backend.URI // Fallback to original
	}

	// Build Triton V2 inference path: /v2/models/{model}/infer
	// Note: Triton expects the model name with slashes in the path (not URL-encoded)
	// Example: /v2/models/meta-llama/Llama-3.1-8B-Instruct/infer
	parsedURI.Path = "/v2/models/" + model + "/infer"
	// Use Opaque to prevent URL parsing from re-encoding the path
	return parsedURI.Scheme + "://" + parsedURI.Host + parsedURI.Path
}

