package public

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"
)

func TestNewSSEWriter(t *testing.T) {
	t.Run("valid response writer", func(t *testing.T) {
		rec := httptest.NewRecorder()
		writer, err := NewSSEWriter(rec, nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if writer == nil {
			t.Error("expected writer, got nil")
		}
	})

	t.Run("with logger", func(t *testing.T) {
		rec := httptest.NewRecorder()
		logger := zap.NewNop()
		writer, err := NewSSEWriter(rec, logger)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if writer == nil {
			t.Error("expected writer, got nil")
		}
	})
}

func TestSSEWriter_Start(t *testing.T) {
	t.Run("sets proper headers", func(t *testing.T) {
		rec := httptest.NewRecorder()
		writer, _ := NewSSEWriter(rec, nil)

		err := writer.Start()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Check headers
		if rec.Header().Get("Content-Type") != "text/event-stream" {
			t.Errorf("expected Content-Type 'text/event-stream', got %q", rec.Header().Get("Content-Type"))
		}
		if rec.Header().Get("Cache-Control") != "no-cache" {
			t.Errorf("expected Cache-Control 'no-cache', got %q", rec.Header().Get("Cache-Control"))
		}
		if rec.Header().Get("Connection") != "keep-alive" {
			t.Errorf("expected Connection 'keep-alive', got %q", rec.Header().Get("Connection"))
		}
		if rec.Header().Get("X-Accel-Buffering") != "no" {
			t.Errorf("expected X-Accel-Buffering 'no', got %q", rec.Header().Get("X-Accel-Buffering"))
		}
	})

	t.Run("idempotent", func(t *testing.T) {
		rec := httptest.NewRecorder()
		writer, _ := NewSSEWriter(rec, nil)

		writer.Start()
		err := writer.Start()
		if err != nil {
			t.Errorf("second Start should not error: %v", err)
		}
	})

	t.Run("fails after close", func(t *testing.T) {
		rec := httptest.NewRecorder()
		writer, _ := NewSSEWriter(rec, nil)

		writer.Close()
		err := writer.Start()
		if err == nil {
			t.Error("expected error after close")
		}
	})
}

func TestSSEWriter_WriteChunk(t *testing.T) {
	t.Run("writes JSON chunk", func(t *testing.T) {
		rec := httptest.NewRecorder()
		writer, _ := NewSSEWriter(rec, nil)
		writer.Start()

		chunk := map[string]string{"message": "hello"}
		err := writer.WriteChunk(chunk)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "data: ") {
			t.Error("expected 'data: ' prefix")
		}
		if !strings.Contains(body, `"message":"hello"`) {
			t.Error("expected JSON content")
		}
		if !strings.HasSuffix(body, "\n\n") {
			t.Error("expected double newline suffix")
		}
	})

	t.Run("fails without start", func(t *testing.T) {
		rec := httptest.NewRecorder()
		writer, _ := NewSSEWriter(rec, nil)

		err := writer.WriteChunk(map[string]string{"test": "data"})
		if err == nil {
			t.Error("expected error without Start")
		}
	})

	t.Run("fails after close", func(t *testing.T) {
		rec := httptest.NewRecorder()
		writer, _ := NewSSEWriter(rec, nil)
		writer.Start()
		writer.Close()

		err := writer.WriteChunk(map[string]string{"test": "data"})
		if err == nil {
			t.Error("expected error after close")
		}
	})
}

func TestSSEWriter_WriteRawData(t *testing.T) {
	t.Run("writes raw data", func(t *testing.T) {
		rec := httptest.NewRecorder()
		writer, _ := NewSSEWriter(rec, nil)
		writer.Start()

		err := writer.WriteRawData(`{"custom":"data"}`)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		body := rec.Body.String()
		if !strings.Contains(body, `data: {"custom":"data"}`) {
			t.Errorf("expected raw data in output, got: %s", body)
		}
	})
}

func TestSSEWriter_WriteDone(t *testing.T) {
	t.Run("writes [DONE] marker", func(t *testing.T) {
		rec := httptest.NewRecorder()
		writer, _ := NewSSEWriter(rec, nil)
		writer.Start()

		err := writer.WriteDone()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "data: [DONE]") {
			t.Errorf("expected [DONE] marker, got: %s", body)
		}
	})

	t.Run("closes writer", func(t *testing.T) {
		rec := httptest.NewRecorder()
		writer, _ := NewSSEWriter(rec, nil)
		writer.Start()

		writer.WriteDone()

		if !writer.IsClosed() {
			t.Error("expected writer to be closed after WriteDone")
		}
	})

	t.Run("fails without start", func(t *testing.T) {
		rec := httptest.NewRecorder()
		writer, _ := NewSSEWriter(rec, nil)

		err := writer.WriteDone()
		if err == nil {
			t.Error("expected error without Start")
		}
	})
}

func TestSSEWriter_WriteError(t *testing.T) {
	t.Run("writes error in OpenAI format", func(t *testing.T) {
		rec := httptest.NewRecorder()
		writer, _ := NewSSEWriter(rec, nil)
		writer.Start()

		err := writer.WriteError(errors.New("test error"))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "data: ") {
			t.Error("expected 'data: ' prefix")
		}
		if !strings.Contains(body, `"error"`) {
			t.Error("expected error field")
		}
		if !strings.Contains(body, `"message":"test error"`) {
			t.Error("expected error message")
		}
		if !strings.Contains(body, `"type":"server_error"`) {
			t.Error("expected error type")
		}
	})

	t.Run("works without start - auto-starts", func(t *testing.T) {
		rec := httptest.NewRecorder()
		writer, _ := NewSSEWriter(rec, nil)

		err := writer.WriteError(errors.New("test error"))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Should have set headers
		if rec.Header().Get("Content-Type") != "text/event-stream" {
			t.Error("expected SSE headers to be set")
		}
	})
}

func TestSSEWriter_Close(t *testing.T) {
	t.Run("idempotent", func(t *testing.T) {
		rec := httptest.NewRecorder()
		writer, _ := NewSSEWriter(rec, nil)
		writer.Start()

		writer.Close()
		writer.Close() // Should not panic
	})

	t.Run("sets closed state", func(t *testing.T) {
		rec := httptest.NewRecorder()
		writer, _ := NewSSEWriter(rec, nil)

		if writer.IsClosed() {
			t.Error("expected not closed initially")
		}

		writer.Close()

		if !writer.IsClosed() {
			t.Error("expected closed after Close")
		}
	})
}

func TestSSEWriter_States(t *testing.T) {
	t.Run("IsStarted", func(t *testing.T) {
		rec := httptest.NewRecorder()
		writer, _ := NewSSEWriter(rec, nil)

		if writer.IsStarted() {
			t.Error("expected not started initially")
		}

		writer.Start()

		if !writer.IsStarted() {
			t.Error("expected started after Start")
		}
	})
}

func TestSSEWriter_FullStreamingFlow(t *testing.T) {
	rec := httptest.NewRecorder()
	writer, _ := NewSSEWriter(rec, nil)

	// Start stream
	err := writer.Start()
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	// Write multiple chunks
	chunks := []map[string]interface{}{
		{"id": "1", "content": "Hello"},
		{"id": "2", "content": " "},
		{"id": "3", "content": "World"},
	}

	for _, chunk := range chunks {
		if err := writer.WriteChunk(chunk); err != nil {
			t.Fatalf("failed to write chunk: %v", err)
		}
	}

	// Write done
	if err := writer.WriteDone(); err != nil {
		t.Fatalf("failed to write done: %v", err)
	}

	// Verify output
	body := rec.Body.String()
	lines := strings.Split(body, "\n\n")

	// Should have 4 data events (3 chunks + 1 done) plus empty trailing
	dataCount := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			dataCount++
		}
	}

	if dataCount != 4 {
		t.Errorf("expected 4 data events, got %d", dataCount)
	}

	// Verify [DONE] is last data event
	lastDataIdx := strings.LastIndex(body, "data: ")
	if lastDataIdx == -1 {
		t.Fatal("no data events found")
	}
	lastData := body[lastDataIdx:]
	if !strings.HasPrefix(lastData, "data: [DONE]") {
		t.Errorf("expected last data to be [DONE], got: %s", lastData)
	}
}

// mockNonFlushableWriter is a response writer that doesn't implement http.Flusher
type mockNonFlushableWriter struct {
	header http.Header
}

func (m *mockNonFlushableWriter) Header() http.Header {
	if m.header == nil {
		m.header = make(http.Header)
	}
	return m.header
}

func (m *mockNonFlushableWriter) Write([]byte) (int, error) {
	return 0, nil
}

func (m *mockNonFlushableWriter) WriteHeader(int) {}

func TestNewSSEWriter_NonFlushable(t *testing.T) {
	w := &mockNonFlushableWriter{}
	_, err := NewSSEWriter(w, nil)
	if err == nil {
		t.Error("expected error for non-flushable writer")
	}
}
