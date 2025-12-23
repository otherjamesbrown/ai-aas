// Package public provides HTTP handlers for the public API.
//
// This file implements SSEWriter for Server-Sent Events streaming responses
// in the OpenAI streaming format.
package public

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"go.uber.org/zap"
)

// SSEWriter writes Server-Sent Events to an HTTP response.
// It handles the OpenAI streaming format with proper headers, flushing, and error handling.
type SSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
	logger  *zap.Logger
	mu      sync.Mutex
	started bool
	closed  bool
}

// NewSSEWriter creates a new SSE writer from an HTTP response writer.
// Returns an error if the response writer does not support flushing.
func NewSSEWriter(w http.ResponseWriter, logger *zap.Logger) (*SSEWriter, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("response writer does not support flushing")
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	return &SSEWriter{
		w:       w,
		flusher: flusher,
		logger:  logger,
	}, nil
}

// Start writes the SSE headers and prepares for streaming.
// This must be called before any chunks are written.
func (sw *SSEWriter) Start() error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if sw.started {
		return nil // Already started
	}

	if sw.closed {
		return fmt.Errorf("writer is closed")
	}

	// Set SSE headers
	sw.w.Header().Set("Content-Type", "text/event-stream")
	sw.w.Header().Set("Cache-Control", "no-cache")
	sw.w.Header().Set("Connection", "keep-alive")
	sw.w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	sw.started = true
	sw.flusher.Flush()

	return nil
}

// WriteChunk writes a data chunk to the SSE stream.
// The chunk will be serialized to JSON and formatted as an SSE event.
func (sw *SSEWriter) WriteChunk(chunk interface{}) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if sw.closed {
		return fmt.Errorf("writer is closed")
	}

	if !sw.started {
		return fmt.Errorf("writer not started, call Start() first")
	}

	// Serialize chunk to JSON
	data, err := json.Marshal(chunk)
	if err != nil {
		return fmt.Errorf("failed to serialize chunk: %w", err)
	}

	// Write SSE event format: "data: {...}\n\n"
	if _, err := fmt.Fprintf(sw.w, "data: %s\n\n", data); err != nil {
		return fmt.Errorf("failed to write chunk: %w", err)
	}

	sw.flusher.Flush()
	return nil
}

// WriteRawData writes raw data to the SSE stream without JSON serialization.
// Useful for pre-serialized data or special markers.
func (sw *SSEWriter) WriteRawData(data string) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if sw.closed {
		return fmt.Errorf("writer is closed")
	}

	if !sw.started {
		return fmt.Errorf("writer not started, call Start() first")
	}

	// Write SSE event format: "data: ...\n\n"
	if _, err := fmt.Fprintf(sw.w, "data: %s\n\n", data); err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	sw.flusher.Flush()
	return nil
}

// WriteDone writes the [DONE] marker to signal the end of the stream.
// This follows the OpenAI streaming protocol.
func (sw *SSEWriter) WriteDone() error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if sw.closed {
		return fmt.Errorf("writer is closed")
	}

	if !sw.started {
		return fmt.Errorf("writer not started, call Start() first")
	}

	// Write OpenAI's [DONE] marker
	if _, err := fmt.Fprintf(sw.w, "data: [DONE]\n\n"); err != nil {
		return fmt.Errorf("failed to write done marker: %w", err)
	}

	sw.flusher.Flush()
	sw.closed = true
	return nil
}

// WriteError writes an error as an SSE event.
// This allows errors to be streamed to the client in a structured format.
func (sw *SSEWriter) WriteError(err error) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if sw.closed {
		return fmt.Errorf("writer is closed")
	}

	// If not started, start the stream first
	if !sw.started {
		sw.w.Header().Set("Content-Type", "text/event-stream")
		sw.w.Header().Set("Cache-Control", "no-cache")
		sw.w.Header().Set("Connection", "keep-alive")
		sw.w.Header().Set("X-Accel-Buffering", "no")
		sw.started = true
	}

	// Create error response in OpenAI format
	errorResp := map[string]interface{}{
		"error": map[string]interface{}{
			"message": err.Error(),
			"type":    "server_error",
		},
	}

	data, marshalErr := json.Marshal(errorResp)
	if marshalErr != nil {
		sw.logger.Error("failed to marshal error response", zap.Error(marshalErr))
		// Fall back to plain text error
		fmt.Fprintf(sw.w, "data: {\"error\":{\"message\":\"internal error\",\"type\":\"server_error\"}}\n\n")
		sw.flusher.Flush()
		return marshalErr
	}

	if _, writeErr := fmt.Fprintf(sw.w, "data: %s\n\n", data); writeErr != nil {
		return fmt.Errorf("failed to write error: %w", writeErr)
	}

	sw.flusher.Flush()
	return nil
}

// Close closes the SSE writer.
// After Close is called, no more data can be written.
func (sw *SSEWriter) Close() {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	sw.closed = true
}

// IsClosed returns whether the writer has been closed.
func (sw *SSEWriter) IsClosed() bool {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.closed
}

// IsStarted returns whether the writer has been started.
func (sw *SSEWriter) IsStarted() bool {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.started
}
