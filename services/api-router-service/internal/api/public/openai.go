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

	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/adapter/preprocessor"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/adapter/triton"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/auth"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/routing"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/telemetry"
)

// Type aliases for OpenAI chat completion types.
// These reuse the types defined in the triton adapter to eliminate duplication.
// The triton package defines comprehensive OpenAI-compatible types used for
// protocol translation, which are now shared with the public API layer.
type (
	// OpenAIChatCompletionRequest represents an OpenAI chat completions API request.
	OpenAIChatCompletionRequest = triton.OpenAIChatCompletionRequest

	// OpenAIMessage represents a message in an OpenAI chat conversation.
	OpenAIMessage = triton.ChatMessage

	// OpenAIChatCompletionResponse represents an OpenAI chat completions API response.
	OpenAIChatCompletionResponse = triton.OpenAIChatCompletionResponse

	// OpenAIChoice represents a completion choice in an OpenAI response.
	OpenAIChoice = triton.ChatCompletionChoice

	// OpenAIUsage represents token usage information in an OpenAI response.
	OpenAIUsage = triton.UsageInfo
)

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

	// Apply preprocessing if configured for this model
	if policy.PreprocessorConfig != nil && policy.PreprocessorConfig.Enabled {
		preprocessedReq, err := h.preprocessRequest(ctx, policy, &openAIReq)
		if err != nil {
			if policy.PreprocessorConfig.BypassOnFailure {
				h.logger.Warn("preprocessor failed, bypassing",
					zap.String("model", openAIReq.Model),
					zap.String("endpoint", policy.PreprocessorConfig.Endpoint),
					zap.Error(err),
				)
				// Continue with original request
			} else {
				h.writeError(w, r, fmt.Errorf("preprocessing failed: %w", err), api.ErrCodeBackendError)
				return
			}
		} else {
			openAIReq = *preprocessedReq
		}
	}

	// Route based on backend type (spec032 - Triton API Support)
	switch policy.BackendType {
	case "triton", "triton-grpc":
		// Both "triton" (HTTP) and "triton-grpc" are handled by the same entry point,
		// which internally routes to HTTP or gRPC based on BackendType
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
		// Normalize content: extract from reasoning_content if content is empty (for reasoning models)
		for i := range chatResp.Choices {
			normalized := chatResp.Choices[i].Message.NormalizeContent()
			chatResp.Choices[i].Message.Content = normalized
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
//
// TRT-LLM Note: TRT-LLM models require gRPC due to "decoupled transaction policy".
// When backend_type is "triton-grpc", ALL requests (streaming and non-streaming)
// are routed through gRPC. HTTP is only used for standard Triton backends.
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

	// Check if gRPC is required (TRT-LLM models require gRPC for all requests)
	useGRPC := policy.BackendType == "triton-grpc" || (policy.TritonConfig != nil && policy.TritonConfig.IsGRPC())

	if req.Stream {
		// Streaming always requires gRPC
		if useGRPC {
			h.handleTritonStreamingChatCompletion(ctx, w, r, policy, req, authCtx, startTime, h.grpcClients, h.grpcTranslators)
			return
		}
		// Standard Triton HTTP does not support streaming
		h.writeError(w, r, fmt.Errorf("streaming requires gRPC backend (set backend_type: triton-grpc or triton_config.protocol: grpc)"), api.ErrCodeValidationError)
		return
	}

	// For non-streaming with gRPC backend, use gRPC (required for TRT-LLM)
	if useGRPC {
		h.handleTritonNonStreamingGRPC(ctx, w, r, policy, req, authCtx, startTime)
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

	// Translate OpenAI request to Triton format
	// Note: OpenAIChatCompletionRequest is now a type alias for triton.OpenAIChatCompletionRequest,
	// so we can pass the request directly without conversion.
	translateReqStart := time.Now()
	tritonReq, err := translator.TranslateOpenAIToTriton(req)
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
	openAIResp, err := translator.TranslateTritonToOpenAI(tritonResp, req)
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
	// TRT-LLM always uses "ensemble" as the model name (standard pipeline structure)
	// Each deployment is a separate service, so routing to the correct service handles model selection
	parsedURI.Path = "/v2/models/ensemble/infer"
	// Use Opaque to prevent URL parsing from re-encoding the path
	return parsedURI.Scheme + "://" + parsedURI.Host + parsedURI.Path
}

// handleTritonNonStreamingGRPC handles non-streaming chat completion requests using gRPC.
// This is required for TRT-LLM models that use "decoupled transaction policy" and cannot
// use HTTP for inference. The function uses gRPC streaming to collect all response chunks
// and returns a single OpenAI-compatible JSON response.
func (h *Handler) handleTritonNonStreamingGRPC(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
	policy *config.RoutingPolicy,
	req *OpenAIChatCompletionRequest,
	authCtx *auth.AuthenticatedContext,
	startTime time.Time,
) {
	ctx, span := h.tracer.Start(ctx, "triton.grpc_chat_completions")
	defer span.End()

	// Validate tokenizer is configured
	if policy.Tokenizer == "" {
		h.writeError(w, r, fmt.Errorf("tokenizer encoding is required for Triton backends"), api.ErrCodeValidationError)
		return
	}

	// Get gRPC endpoint for the backend
	backendID := policy.Backends[0].BackendID
	grpcEndpoint := h.buildTritonGRPCEndpoint(backendID, policy.Model)

	h.logger.Debug("starting triton gRPC non-streaming request",
		zap.String("backend_id", backendID),
		zap.String("grpc_endpoint", grpcEndpoint),
		zap.String("model", policy.Model),
	)

	// Get or create gRPC client
	client, err := h.grpcClients.getOrCreate(backendID, grpcEndpoint)
	if err != nil {
		h.logger.Error("failed to get gRPC client",
			zap.String("backend_id", backendID),
			zap.Error(err),
		)
		h.writeError(w, r, fmt.Errorf("backend connection failed"), api.ErrCodeBackendError)
		return
	}

	// Get or create translator
	translator, err := h.grpcTranslators.getOrCreate(policy.Tokenizer)
	if err != nil {
		h.logger.Error("failed to get gRPC translator",
			zap.String("tokenizer", policy.Tokenizer),
			zap.Error(err),
		)
		h.writeError(w, r, fmt.Errorf("internal configuration error"), api.ErrCodeBackendError)
		return
	}

	// Translate OpenAI request to gRPC format
	// TRT-LLM always uses "ensemble" as the model name (standard pipeline structure)
	grpcReq, requestID, err := translator.TranslateOpenAIToGRPC(req, "ensemble")
	if err != nil {
		h.logger.Error("failed to translate request",
			zap.Error(err),
		)
		h.writeError(w, r, fmt.Errorf("request translation failed: %w", err), api.ErrCodeValidationError)
		return
	}

	// DEBUG: Log the gRPC request structure
	h.logger.Debug("translated gRPC request",
		zap.String("request_id", requestID),
		zap.String("model_name", grpcReq.ModelName),
		zap.Int("num_inputs", len(grpcReq.Inputs)),
		zap.Int("num_outputs", len(grpcReq.Outputs)),
	)

	// Log each input tensor
	for i, input := range grpcReq.Inputs {
		var valuePreview string
		if input.Contents != nil && len(input.Contents.BytesContents) > 0 {
			valuePreview = string(input.Contents.BytesContents[0])
			if len(valuePreview) > 200 {
				valuePreview = valuePreview[:200] + "..."
			}
		} else if input.Contents != nil && len(input.Contents.IntContents) > 0 {
			valuePreview = fmt.Sprintf("%v", input.Contents.IntContents[0])
		} else if input.Contents != nil && len(input.Contents.Fp32Contents) > 0 {
			valuePreview = fmt.Sprintf("%v", input.Contents.Fp32Contents[0])
		} else if input.Contents != nil && len(input.Contents.BoolContents) > 0 {
			valuePreview = fmt.Sprintf("%v", input.Contents.BoolContents[0])
		}

		h.logger.Debug("gRPC input tensor",
			zap.String("request_id", requestID),
			zap.Int("index", i),
			zap.String("name", input.Name),
			zap.String("datatype", input.Datatype),
			zap.Any("shape", input.Shape),
			zap.String("value_preview", valuePreview),
		)
	}

	// Start gRPC stream
	stream, err := client.StreamInfer(ctx)
	if err != nil {
		h.logger.Error("failed to start gRPC stream",
			zap.String("backend_id", backendID),
			zap.Error(err),
		)
		h.writeError(w, r, fmt.Errorf("backend streaming failed"), api.ErrCodeBackendError)
		return
	}

	// Send the request
	if err := stream.Send(grpcReq); err != nil {
		h.logger.Error("failed to send request to gRPC stream",
			zap.Error(err),
		)
		h.writeError(w, r, fmt.Errorf("request sending failed"), api.ErrCodeBackendError)
		return
	}

	// Close send side to signal we're done sending
	if err := stream.CloseSend(); err != nil {
		h.logger.Warn("failed to close send side of stream",
			zap.Error(err),
		)
	}

	// Collect all streaming responses into a single response
	var accumulatedText string
	promptTokens := translator.CountPromptTokens(req.Messages)
	responseCount := 0

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			h.logger.Error("error receiving from gRPC stream",
				zap.Error(err),
			)
			h.writeError(w, r, fmt.Errorf("backend error: %v", err), api.ErrCodeBackendError)
			return
		}

		responseCount++

		// DEBUG: Log raw gRPC response structure
		h.logger.Debug("received gRPC response",
			zap.Int("response_count", responseCount),
			zap.String("request_id", requestID),
			zap.Bool("has_infer_response", resp.InferResponse != nil),
			zap.String("error_message", resp.ErrorMessage),
		)

		// Check for backend error in response - fail fast instead of returning empty response
		if resp.ErrorMessage != "" {
			h.logger.Error("backend returned error in gRPC response",
				zap.String("request_id", requestID),
				zap.String("backend_id", backendID),
				zap.String("error_message", resp.ErrorMessage),
			)
			h.writeError(w, r, fmt.Errorf("backend error: %s", resp.ErrorMessage), api.ErrCodeBackendError)
			return
		}

		if resp.InferResponse != nil {
			h.logger.Debug("gRPC response details",
				zap.String("request_id", requestID),
				zap.Int("num_outputs", len(resp.InferResponse.Outputs)),
				zap.Int("num_raw_outputs", len(resp.InferResponse.RawOutputContents)),
			)

			// Log details of each output tensor
			for i, output := range resp.InferResponse.Outputs {
				hasContents := output.Contents != nil
				var numBytesContents int
				if hasContents {
					numBytesContents = len(output.Contents.BytesContents)
				}
				var rawSize int
				if i < len(resp.InferResponse.RawOutputContents) {
					rawSize = len(resp.InferResponse.RawOutputContents[i])
				}

				h.logger.Debug("output tensor details",
					zap.String("request_id", requestID),
					zap.Int("output_index", i),
					zap.String("name", output.Name),
					zap.String("datatype", output.Datatype),
					zap.Any("shape", output.Shape),
					zap.Bool("has_contents", hasContents),
					zap.Int("num_bytes_contents", numBytesContents),
					zap.Int("raw_output_size", rawSize),
				)

				// If we have raw output, log a sample
				if rawSize > 0 {
					sample := string(resp.InferResponse.RawOutputContents[i])
					if len(sample) > 100 {
						sample = sample[:100] + "..."
					}
					h.logger.Debug("raw output sample",
						zap.String("request_id", requestID),
						zap.Int("output_index", i),
						zap.String("sample", sample),
					)
				}
			}
		}

		// Extract text from response
		text, err := translator.ExtractTextFromGRPCResponse(resp)
		if err != nil {
			h.logger.Warn("failed to extract text from gRPC response",
				zap.Error(err),
			)
			continue
		}

		h.logger.Debug("extracted text from response",
			zap.String("request_id", requestID),
			zap.Int("response_count", responseCount),
			zap.Int("text_length", len(text)),
		)

		accumulatedText += text
	}

	// Calculate completion tokens
	completionTokens := translator.CountCompletionTokens(accumulatedText)

	// Build OpenAI response
	// Normalize accumulated text to prevent nil content
	content := interface{}(accumulatedText)
	if content == nil || content == "" {
		content = ""
	}

	openAIResp := triton.OpenAIChatCompletionResponse{
		ID:      requestID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []triton.ChatCompletionChoice{
			{
				Index: 0,
				Message: triton.ChatMessage{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: "stop",
			},
		},
		Usage: triton.UsageInfo{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		},
	}

	// Create routing decision for metrics
	routingDecision := &routing.RoutingDecision{
		BackendID:     backendID,
		DecisionType:  "PRIMARY",
		Reason:        "Triton gRPC backend routing",
		Timestamp:     time.Now(),
		AttemptNumber: 1,
	}

	// Add routing headers
	w.Header().Set("X-Routing-Backend", routingDecision.BackendID)
	w.Header().Set("X-Routing-Decision", routingDecision.DecisionType)
	w.Header().Set("X-Backend-Type", "triton-grpc")

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

	// Record token metrics
	if h.tokenMetrics != nil {
		h.tokenMetrics.RecordTokens(
			ctx,
			authCtx.OrganizationID,
			req.Model,
			"chat",
			openAIResp.Usage.PromptTokens,
			openAIResp.Usage.CompletionTokens,
		)
	}

	// Record per-backend Prometheus metrics
	requestLatency := time.Since(startTime)
	telemetry.RecordBackendRequest(
		routingDecision.BackendID,
		authCtx.OrganizationID,
		req.Model,
		true, // success
		requestLatency,
	)

	h.logger.Info("triton gRPC request completed",
		zap.String("request_id", requestID),
		zap.String("backend_id", backendID),
		zap.String("backend_type", "triton-grpc"),
		zap.Int("prompt_tokens", openAIResp.Usage.PromptTokens),
		zap.Int("completion_tokens", openAIResp.Usage.CompletionTokens),
		zap.Duration("latency", requestLatency),
	)

	// Write response
	if err := h.writeJSON(w, http.StatusOK, openAIResp); err != nil {
		h.logger.Error("failed to write triton gRPC response", zap.Error(err))
	}
}

// preprocessRequest applies model-specific chat template preprocessing.
// It calls the preprocessor service to format the prompt according to the model's
// chat template (e.g., Llama-3 format, Harmony format, etc.).
func (h *Handler) preprocessRequest(
	ctx context.Context,
	policy *config.RoutingPolicy,
	req *OpenAIChatCompletionRequest,
) (*OpenAIChatCompletionRequest, error) {
	if policy.PreprocessorConfig == nil {
		return req, nil
	}

	// Get or create preprocessor client
	client, err := h.preprocessorClients.GetOrCreate(policy.PreprocessorConfig.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("get preprocessor client: %w", err)
	}

	// Apply timeout from config
	timeout := policy.PreprocessorConfig.GetTimeout()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Convert messages to preprocessor format
	// Extract text content from multimodal messages for preprocessing
	messages := make([]preprocessor.Message, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = preprocessor.Message{
			Role:    msg.Role,
			Content: msg.GetTextContent(),
		}
	}

	// Determine engine type based on backend type
	engineType := "vllm" // default
	switch policy.BackendType {
	case "triton":
		engineType = "triton_string"
	case "triton-grpc":
		engineType = "triton_tensor"
	}

	// Build preprocessor request
	prepReq := &preprocessor.PreprocessRequest{
		ModelID:    policy.PreprocessorConfig.ModelID,
		EngineType: engineType,
		Messages:   messages,
	}
	if req.MaxTokens != nil {
		prepReq.MaxTokens = int32(*req.MaxTokens)
	}
	if req.Temperature != nil {
		prepReq.Temperature = float32(*req.Temperature)
	}
	if req.TopP != nil {
		prepReq.TopP = float32(*req.TopP)
	}
	prepReq.Stream = req.Stream

	h.logger.Debug("calling preprocessor",
		zap.String("model_id", prepReq.ModelID),
		zap.String("engine_type", engineType),
		zap.Int("num_messages", len(messages)),
	)

	// Call preprocessor
	resp, err := client.Preprocess(ctx, prepReq)
	if err != nil {
		return nil, fmt.Errorf("preprocess RPC: %w", err)
	}

	h.logger.Debug("preprocessor response",
		zap.Int("prompt_length", len(resp.FormattedPrompt)),
		zap.Int32("token_count", resp.PromptTokenCount),
	)

	// Create new request with preprocessed prompt
	// For string modes (vllm, triton_string), we embed the formatted prompt
	// in the first message content. The backend will use this directly.
	newReq := *req
	if resp.FormattedPrompt != "" {
		// Replace messages with a single message containing the formatted prompt
		// This works for backends that accept a pre-formatted prompt
		newReq.Messages = []OpenAIMessage{
			{
				Role:    "user",
				Content: resp.FormattedPrompt,
			},
		}
	}

	// For tensor mode, the formatted prompt is also available but the
	// input_ids would be used directly by the gRPC translator.
	// TODO: Store input_ids in request context for gRPC translator to use

	return &newReq, nil
}

