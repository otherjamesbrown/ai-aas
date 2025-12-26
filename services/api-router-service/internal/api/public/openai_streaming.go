// Package public provides HTTP handlers for the public API.
//
// This file implements streaming inference handlers for gRPC-based backends,
// specifically designed for TensorRT-LLM models on Triton that require
// bidirectional streaming ("decoupled transaction policy").
package public

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/adapter/triton"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/auth"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/config"
)

// gRPCClientManager manages gRPC client connections for streaming backends.
// It is added to the Handler struct in handler.go.
type gRPCClientManager struct {
	clients map[string]*triton.TritonGRPCClient // backendID -> client
	mu      sync.RWMutex
	logger  *zap.Logger
}

// newGRPCClientManager creates a new gRPC client manager.
func newGRPCClientManager(logger *zap.Logger) *gRPCClientManager {
	return &gRPCClientManager{
		clients: make(map[string]*triton.TritonGRPCClient),
		logger:  logger,
	}
}

// getOrCreate returns an existing client or creates a new one for the backend.
func (m *gRPCClientManager) getOrCreate(backendID, endpoint string) (*triton.TritonGRPCClient, error) {
	// Fast path: check with read lock
	m.mu.RLock()
	if client, ok := m.clients[backendID]; ok {
		m.mu.RUnlock()
		return client, nil
	}
	m.mu.RUnlock()

	// Slow path: create with write lock
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if client, ok := m.clients[backendID]; ok {
		return client, nil
	}

	// Create new client
	cfg := triton.DefaultGRPCClientConfig(endpoint)
	client, err := triton.NewTritonGRPCClient(cfg, m.logger)
	if err != nil {
		return nil, fmt.Errorf("create gRPC client for %s: %w", backendID, err)
	}

	m.clients[backendID] = client
	return client, nil
}

// close closes all managed clients.
func (m *gRPCClientManager) close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, client := range m.clients {
		if err := client.Close(); err != nil {
			m.logger.Warn("failed to close gRPC client",
				zap.String("backend_id", id),
				zap.Error(err),
			)
		}
	}
	m.clients = make(map[string]*triton.TritonGRPCClient)
}

// gRPCTranslatorManager manages gRPC translators keyed by tokenizer encoding.
type gRPCTranslatorManager struct {
	translators map[string]*triton.GRPCTranslator
	mu          sync.RWMutex
}

// newGRPCTranslatorManager creates a new translator manager.
func newGRPCTranslatorManager() *gRPCTranslatorManager {
	return &gRPCTranslatorManager{
		translators: make(map[string]*triton.GRPCTranslator),
	}
}

// getOrCreate returns an existing translator or creates a new one.
func (m *gRPCTranslatorManager) getOrCreate(encoding string) (*triton.GRPCTranslator, error) {
	// Fast path
	m.mu.RLock()
	if t, ok := m.translators[encoding]; ok {
		m.mu.RUnlock()
		return t, nil
	}
	m.mu.RUnlock()

	// Slow path
	m.mu.Lock()
	defer m.mu.Unlock()

	if t, ok := m.translators[encoding]; ok {
		return t, nil
	}

	t, err := triton.NewGRPCTranslator(encoding)
	if err != nil {
		return nil, err
	}

	m.translators[encoding] = t
	return t, nil
}

// handleTritonStreamingChatCompletion handles streaming chat completion requests for Triton gRPC backends.
// It uses bidirectional gRPC streaming (ModelStreamInfer) and translates responses to SSE format.
func (h *Handler) handleTritonStreamingChatCompletion(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
	policy *config.RoutingPolicy,
	req *OpenAIChatCompletionRequest,
	authCtx *auth.AuthenticatedContext,
	startTime time.Time,
	grpcClients *gRPCClientManager,
	grpcTranslators *gRPCTranslatorManager,
) {
	ctx, span := h.tracer.Start(ctx, "triton.streaming_chat_completions",
		trace.WithAttributes(
			attribute.String("model", req.Model),
			attribute.Bool("stream", true),
		),
	)
	defer span.End()

	// Get gRPC endpoint for the backend
	backendID := policy.Backends[0].BackendID
	grpcEndpoint := h.buildTritonGRPCEndpoint(backendID, policy.Model)

	h.logger.Debug("starting triton streaming request",
		zap.String("backend_id", backendID),
		zap.String("grpc_endpoint", grpcEndpoint),
		zap.String("model", policy.Model),
	)

	// Get or create gRPC client
	client, err := grpcClients.getOrCreate(backendID, grpcEndpoint)
	if err != nil {
		h.logger.Error("failed to get gRPC client",
			zap.String("backend_id", backendID),
			zap.Error(err),
		)
		h.writeError(w, r, fmt.Errorf("backend connection failed"), api.ErrCodeBackendError)
		return
	}

	// Get or create translator
	translator, err := grpcTranslators.getOrCreate(policy.Tokenizer)
	if err != nil {
		h.logger.Error("failed to get gRPC translator",
			zap.String("tokenizer", policy.Tokenizer),
			zap.Error(err),
		)
		h.writeError(w, r, fmt.Errorf("internal configuration error"), api.ErrCodeBackendError)
		return
	}

	// Translate OpenAI request to gRPC format
	grpcReq, requestID, err := translator.TranslateOpenAIToGRPC(req, policy.Model)
	if err != nil {
		h.logger.Error("failed to translate request",
			zap.Error(err),
		)
		h.writeError(w, r, fmt.Errorf("request translation failed: %w", err), api.ErrCodeValidationError)
		return
	}

	// Create cancellable context tied to client disconnect
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Monitor for client disconnect
	go func() {
		<-r.Context().Done()
		h.logger.Debug("client disconnected, cancelling stream",
			zap.String("request_id", requestID),
		)
		cancel()
	}()

	// Create SSE writer
	sseWriter, err := NewSSEWriter(w, h.logger)
	if err != nil {
		h.writeError(w, r, fmt.Errorf("streaming not supported"), api.ErrCodeBackendError)
		return
	}

	// Start SSE stream
	if err := sseWriter.Start(); err != nil {
		h.logger.Error("failed to start SSE stream", zap.Error(err))
		return
	}

	// Start gRPC stream
	stream, err := client.StreamInfer(streamCtx)
	if err != nil {
		h.logger.Error("failed to start gRPC stream",
			zap.String("backend_id", backendID),
			zap.Error(err),
		)
		sseWriter.WriteError(fmt.Errorf("backend streaming failed"))
		return
	}

	// Send the request
	if err := stream.Send(grpcReq); err != nil {
		h.logger.Error("failed to send request to gRPC stream",
			zap.Error(err),
		)
		sseWriter.WriteError(fmt.Errorf("request sending failed"))
		return
	}

	// Close send side to signal we're done sending
	if err := stream.CloseSend(); err != nil {
		h.logger.Warn("failed to close send side of stream",
			zap.Error(err),
		)
	}

	// Track token counts for usage
	promptTokens := translator.CountPromptTokens(req.Messages)
	var completionTokens int
	var accumulatedText string

	// Receive and translate streaming responses
	created := time.Now().Unix()
	chunkIndex := 0

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			// Stream complete
			break
		}
		if err != nil {
			// Check if this is a context cancellation (client disconnect)
			if streamCtx.Err() != nil {
				h.logger.Debug("stream cancelled due to client disconnect",
					zap.String("request_id", requestID),
				)
				return
			}
			h.logger.Error("error receiving from gRPC stream",
				zap.Error(err),
			)
			sseWriter.WriteError(fmt.Errorf("streaming error: %v", err))
			return
		}

		// Translate response to OpenAI streaming chunk
		chunk, err := translator.TranslateStreamChunk(resp, requestID, req.Model, chunkIndex, created)
		if err != nil {
			h.logger.Error("failed to translate stream chunk",
				zap.Error(err),
			)
			sseWriter.WriteError(err)
			return
		}

		// Accumulate text for token counting
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			accumulatedText += chunk.Choices[0].Delta.Content
		}

		// Write chunk to SSE stream
		if err := sseWriter.WriteChunk(chunk); err != nil {
			h.logger.Error("failed to write SSE chunk",
				zap.Error(err),
			)
			return
		}

		chunkIndex++
	}

	// Calculate final token counts
	completionTokens = translator.CountCompletionTokens(accumulatedText)

	// Write final chunk with finish reason and usage
	finalChunk := translator.TranslateFinalChunk(
		requestID,
		req.Model,
		created,
		"stop",
		promptTokens,
		completionTokens,
	)
	if err := sseWriter.WriteChunk(finalChunk); err != nil {
		h.logger.Error("failed to write final SSE chunk",
			zap.Error(err),
		)
		return
	}

	// Write [DONE] marker
	if err := sseWriter.WriteDone(); err != nil {
		h.logger.Error("failed to write SSE done marker",
			zap.Error(err),
		)
		return
	}

	// Record metrics
	duration := time.Since(startTime)
	h.logger.Info("triton streaming request completed",
		zap.String("request_id", requestID),
		zap.String("model", req.Model),
		zap.Int("prompt_tokens", promptTokens),
		zap.Int("completion_tokens", completionTokens),
		zap.Duration("duration", duration),
		zap.Int("chunks", chunkIndex),
	)

	span.SetAttributes(
		attribute.Int("prompt_tokens", promptTokens),
		attribute.Int("completion_tokens", completionTokens),
		attribute.Int("total_tokens", promptTokens+completionTokens),
		attribute.Int("chunks_sent", chunkIndex),
	)
}

// buildTritonGRPCEndpoint constructs the gRPC endpoint URL for a Triton backend.
// By default, uses port 9000 for gRPC (TRT-LLM's default gRPC port).
func (h *Handler) buildTritonGRPCEndpoint(backendID, model string) string {
	// Default gRPC port for TRT-LLM/Triton
	grpcPort := 9000

	backend, err := h.backendRegistry.GetBackend(backendID)
	if err == nil && backend != nil {
		// Extract host from HTTP URI and use gRPC port
		return fmt.Sprintf("%s:%d", extractHost(backend.URI), grpcPort)
	}

	// Fallback: use model name as service name with Kubernetes DNS
	return fmt.Sprintf("%s.system.svc.cluster.local:%d", model, grpcPort)
}

// extractHost extracts the host from a URL (removes scheme and path).
func extractHost(uri string) string {
	// Simple extraction - can be enhanced with url.Parse for robustness
	// Remove scheme
	if len(uri) > 7 && uri[:7] == "http://" {
		uri = uri[7:]
	} else if len(uri) > 8 && uri[:8] == "https://" {
		uri = uri[8:]
	}
	// Remove path
	for i, c := range uri {
		if c == '/' || c == ':' {
			return uri[:i]
		}
	}
	return uri
}
