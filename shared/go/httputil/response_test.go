package httputil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ai-aas/shared-go/errors"
)

func TestWriteJSON(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		data       interface{}
		wantStatus int
		wantBody   string
	}{
		{
			name:       "simple object",
			status:     http.StatusOK,
			data:       map[string]string{"message": "hello"},
			wantStatus: http.StatusOK,
			wantBody:   `{"message":"hello"}`,
		},
		{
			name:       "nil data",
			status:     http.StatusNoContent,
			data:       nil,
			wantStatus: http.StatusNoContent,
			wantBody:   "",
		},
		{
			name:       "created status",
			status:     http.StatusCreated,
			data:       map[string]int{"id": 123},
			wantStatus: http.StatusCreated,
			wantBody:   `{"id":123}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			WriteJSON(w, tt.status, tt.data)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.data != nil {
				contentType := w.Header().Get("Content-Type")
				if contentType != "application/json" {
					t.Errorf("Content-Type = %q, want application/json", contentType)
				}
			}

			if tt.wantBody != "" {
				var got, want interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
					t.Fatalf("failed to unmarshal response: %v", err)
				}
				if err := json.Unmarshal([]byte(tt.wantBody), &want); err != nil {
					t.Fatalf("failed to unmarshal expected: %v", err)
				}
				gotJSON, _ := json.Marshal(got)
				wantJSON, _ := json.Marshal(want)
				if string(gotJSON) != string(wantJSON) {
					t.Errorf("body = %s, want %s", gotJSON, wantJSON)
				}
			}
		})
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := errors.New(errors.ErrCodeNotFound, "Resource Not Found",
		errors.WithDetail("User with ID '123' was not found"),
	)

	WriteError(w, r, err)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Error != "Resource Not Found" {
		t.Errorf("error = %q, want %q", resp.Error, "Resource Not Found")
	}
	if resp.Code != errors.ErrCodeNotFound {
		t.Errorf("code = %q, want %q", resp.Code, errors.ErrCodeNotFound)
	}
	if resp.Detail != "User with ID '123' was not found" {
		t.Errorf("detail = %q, want %q", resp.Detail, "User with ID '123' was not found")
	}
}

func TestWriteValidationError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/users", nil)

	validationErrors := []ValidationError{
		{Field: "email", Message: "must be a valid email address"},
		{Field: "name", Message: "is required"},
	}

	WriteValidationError(w, r, validationErrors)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Code != errors.ErrCodeValidationError {
		t.Errorf("code = %q, want %q", resp.Code, errors.ErrCodeValidationError)
	}
	if len(resp.Errors) != 2 {
		t.Errorf("errors count = %d, want 2", len(resp.Errors))
	}
	if resp.Errors[0].Field != "email" {
		t.Errorf("first error field = %q, want %q", resp.Errors[0].Field, "email")
	}
}

func TestWriteNotFound(t *testing.T) {
	tests := []struct {
		name         string
		resourceType string
		id           string
		wantDetail   string
	}{
		{
			name:         "with id",
			resourceType: "User",
			id:           "123",
			wantDetail:   "User with ID '123' was not found",
		},
		{
			name:         "without id",
			resourceType: "Organization",
			id:           "",
			wantDetail:   "Organization not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/test", nil)

			WriteNotFound(w, r, tt.resourceType, tt.id)

			if w.Code != http.StatusNotFound {
				t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
			}

			var resp ErrorResponse
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}

			if resp.Detail != tt.wantDetail {
				t.Errorf("detail = %q, want %q", resp.Detail, tt.wantDetail)
			}
		})
	}
}

func TestWriteConflict(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/users", nil)

	WriteConflict(w, r, "User with email already exists")

	if w.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", w.Code, http.StatusConflict)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Code != errors.ErrCodeConflict {
		t.Errorf("code = %q, want %q", resp.Code, errors.ErrCodeConflict)
	}
}

func TestWriteInternalError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	WriteInternalError(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Code != errors.ErrCodeInternalError {
		t.Errorf("code = %q, want %q", resp.Code, errors.ErrCodeInternalError)
	}
}

func TestWriteUnauthorized(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	WriteUnauthorized(w, r, "Invalid API key")

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Code != errors.ErrCodeUnauthorized {
		t.Errorf("code = %q, want %q", resp.Code, errors.ErrCodeUnauthorized)
	}
	if resp.Detail != "Invalid API key" {
		t.Errorf("detail = %q, want %q", resp.Detail, "Invalid API key")
	}
}

func TestWriteForbidden(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	WriteForbidden(w, r, "Model access denied")

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Code != errors.ErrCodeForbidden {
		t.Errorf("code = %q, want %q", resp.Code, errors.ErrCodeForbidden)
	}
}

func TestWriteLimitError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/v1/chat/completions", nil)

	retryAfter := 30
	limitContext := map[string]interface{}{
		"limit":     100,
		"remaining": 0,
		"reset":     "2024-01-01T00:00:00Z",
	}

	WriteLimitError(w, r, errors.ErrCodeRateLimitExceeded, &retryAfter, limitContext)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}

	if w.Header().Get("Retry-After") != "30" {
		t.Errorf("Retry-After = %q, want %q", w.Header().Get("Retry-After"), "30")
	}

	var resp LimitErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Code != errors.ErrCodeRateLimitExceeded {
		t.Errorf("code = %q, want %q", resp.Code, errors.ErrCodeRateLimitExceeded)
	}
	if resp.RetryAfterSeconds == nil || *resp.RetryAfterSeconds != 30 {
		t.Errorf("retry_after_seconds = %v, want 30", resp.RetryAfterSeconds)
	}
}

func TestWriteSuccess(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"status": "ok"}

	WriteSuccess(w, data)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestWriteCreated(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]int{"id": 456}

	WriteCreated(w, data)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}
}

func TestWriteNoContent(t *testing.T) {
	w := httptest.NewRecorder()

	WriteNoContent(w)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNoContent)
	}
}
