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

	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/auth"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/routing"
)

// OpenAI Chat Completions Request (OpenAI-compatible format)
type OpenAIChatCompletionRequest struct {
	Model       string                 `json:"model"`
	Messages    []OpenAIMessage        `json:"messages"`
	MaxTokens   int                    `json:"max_tokens,omitempty"`
	Temperature float64                `json:"temperature,omitempty"`
	Stream      bool                   `json:"stream,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAI Chat Completions Response (OpenAI-compatible format)
type OpenAIChatCompletionResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []OpenAIChoice         `json:"choices"`
	Usage   OpenAIUsage            `json:"usage"`
}

type OpenAIChoice struct {
	Index        int             `json:"index"`
	Message      OpenAIMessage   `json:"message"`
	FinishReason string          `json:"finish_reason"`
}

type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// OpenAI Completions Request (OpenAI-compatible format)
type OpenAICompletionRequest struct {
	Model       string                 `json:"model"`
	Prompt      string                 `json:"prompt"`
	MaxTokens   int                    `json:"max_tokens,omitempty"`
	Temperature float64                `json:"temperature,omitempty"`
	Stream      bool                   `json:"stream,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// OpenAI Completions Response (OpenAI-compatible format)
type OpenAICompletionResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []OpenAICompletionChoice `json:"choices"`
	Usage   OpenAIUsage             `json:"usage"`
}

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
	authCtxValue := r.Context().Value("auth_context")
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

	// Forward OpenAI request directly to backend's OpenAI endpoint
	backendEndpoint := h.buildBackendEndpointForOpenAI(policy.Backends[0].BackendID, openAIReq.Model, "/v1/chat/completions")
	
	// Forward the OpenAI request as-is to the backend
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
	authCtxValue := r.Context().Value("auth_context")
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
	backendEndpoint := h.buildBackendEndpointForOpenAI(policy.Backends[0].BackendID, openAIReq.Model, "/v1/completions")
	
	// Forward the OpenAI request as-is to the backend
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
	defer resp.Body.Close()

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

