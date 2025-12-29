// Package httputil provides HTTP response utilities for ai-aas services.
//
// This package provides consistent JSON response formatting with automatic
// trace ID extraction from OpenTelemetry context.
package httputil

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/ai-aas/shared-go/errors"
	"go.opentelemetry.io/otel/trace"
)

// ErrorResponse represents a standard JSON error response.
type ErrorResponse struct {
	Error   string            `json:"error"`
	Code    string            `json:"code"`
	TraceID string            `json:"trace_id,omitempty"`
	Detail  string            `json:"detail,omitempty"`
	Errors  []ValidationError `json:"errors,omitempty"`
}

// LimitErrorResponse extends ErrorResponse with rate limit context.
type LimitErrorResponse struct {
	Error             string                 `json:"error"`
	Code              string                 `json:"code"`
	TraceID           string                 `json:"trace_id,omitempty"`
	RetryAfterSeconds *int                   `json:"retry_after_seconds,omitempty"`
	LimitContext      map[string]interface{} `json:"limit_context,omitempty"`
}

// extractTraceID extracts the trace ID from the request context.
func extractTraceID(r *http.Request) string {
	span := trace.SpanFromContext(r.Context())
	if span.SpanContext().IsValid() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}

// WriteJSON writes a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Printf("httputil: failed to encode JSON response: %v", err)
		}
	}
}

// WriteError writes an error response using a shared Error.
func WriteError(w http.ResponseWriter, r *http.Request, err *errors.Error) {
	traceID := extractTraceID(r)
	if traceID == "" {
		traceID = err.TraceID
	}

	resp := ErrorResponse{
		Error:   err.Message,
		Code:    err.Code,
		TraceID: traceID,
		Detail:  err.Detail,
	}

	WriteJSON(w, err.HTTPStatus(), resp)
}

// WriteErrorWithCode writes an error response with the given code and message.
func WriteErrorWithCode(w http.ResponseWriter, r *http.Request, code, message string) {
	err := errors.New(code, message,
		errors.WithTraceID(extractTraceID(r)),
	)
	WriteError(w, r, err)
}

// WriteValidationError writes a validation error with field details.
func WriteValidationError(w http.ResponseWriter, r *http.Request, validationErrors []ValidationError) {
	traceID := extractTraceID(r)

	resp := ErrorResponse{
		Error:   "Validation Failed",
		Code:    errors.ErrCodeValidationError,
		TraceID: traceID,
		Detail:  "One or more fields failed validation",
		Errors:  validationErrors,
	}

	WriteJSON(w, http.StatusBadRequest, resp)
}

// WriteNotFound writes a 404 response for a missing resource.
func WriteNotFound(w http.ResponseWriter, r *http.Request, resourceType, id string) {
	detail := resourceType + " not found"
	if id != "" {
		detail = resourceType + " with ID '" + id + "' was not found"
	}

	err := errors.New(errors.ErrCodeNotFound, resourceType+" Not Found",
		errors.WithDetail(detail),
		errors.WithTraceID(extractTraceID(r)),
	)
	WriteError(w, r, err)
}

// WriteConflict writes a 409 conflict response.
func WriteConflict(w http.ResponseWriter, r *http.Request, message string) {
	err := errors.New(errors.ErrCodeConflict, "Conflict",
		errors.WithDetail(message),
		errors.WithTraceID(extractTraceID(r)),
	)
	WriteError(w, r, err)
}

// WriteInternalError writes a generic 500 internal server error.
func WriteInternalError(w http.ResponseWriter, r *http.Request) {
	err := errors.New(errors.ErrCodeInternalError, "Internal Server Error",
		errors.WithDetail("An unexpected error occurred"),
		errors.WithTraceID(extractTraceID(r)),
	)
	WriteError(w, r, err)
}

// WriteUnauthorized writes a 401 unauthorized response.
func WriteUnauthorized(w http.ResponseWriter, r *http.Request, message string) {
	if message == "" {
		message = "Authentication required"
	}
	err := errors.New(errors.ErrCodeUnauthorized, "Unauthorized",
		errors.WithDetail(message),
		errors.WithTraceID(extractTraceID(r)),
	)
	WriteError(w, r, err)
}

// WriteForbidden writes a 403 forbidden response.
func WriteForbidden(w http.ResponseWriter, r *http.Request, message string) {
	if message == "" {
		message = "Access denied"
	}
	err := errors.New(errors.ErrCodeForbidden, "Forbidden",
		errors.WithDetail(message),
		errors.WithTraceID(extractTraceID(r)),
	)
	WriteError(w, r, err)
}

// WriteBadRequest writes a 400 bad request response.
func WriteBadRequest(w http.ResponseWriter, r *http.Request, message string) {
	err := errors.New(errors.ErrCodeInvalidRequest, "Bad Request",
		errors.WithDetail(message),
		errors.WithTraceID(extractTraceID(r)),
	)
	WriteError(w, r, err)
}

// WriteLimitError writes a rate limit error with retry context.
func WriteLimitError(w http.ResponseWriter, r *http.Request, code string, retryAfter *int, limitContext map[string]interface{}) {
	traceID := extractTraceID(r)

	resp := LimitErrorResponse{
		Error:             "Rate limit exceeded",
		Code:              code,
		TraceID:           traceID,
		RetryAfterSeconds: retryAfter,
		LimitContext:      limitContext,
	}

	if retryAfter != nil {
		w.Header().Set("Retry-After", strconv.Itoa(*retryAfter))
	}

	WriteJSON(w, errors.GetHTTPStatus(code), resp)
}

// WriteSuccess writes a successful JSON response with 200 OK.
func WriteSuccess(w http.ResponseWriter, data interface{}) {
	WriteJSON(w, http.StatusOK, data)
}

// WriteCreated writes a successful JSON response with 201 Created.
func WriteCreated(w http.ResponseWriter, data interface{}) {
	WriteJSON(w, http.StatusCreated, data)
}

// WriteNoContent writes a 204 No Content response.
func WriteNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// ProblemDetails represents an RFC 7807 Problem Details response.
// Use this format for services that prefer the RFC 7807 standard.
type ProblemDetails struct {
	// Type is a URI reference that identifies the problem type (optional).
	Type string `json:"type,omitempty"`
	// Title is a short, human-readable summary of the problem type.
	Title string `json:"title"`
	// Status is the HTTP status code.
	Status int `json:"status"`
	// Detail is a human-readable explanation specific to this occurrence.
	Detail string `json:"detail,omitempty"`
	// Instance is a URI reference that identifies the specific occurrence (optional).
	Instance string `json:"instance,omitempty"`
}

// WriteProblemDetails writes an RFC 7807 Problem Details response.
// Uses Content-Type: application/problem+json as per the RFC.
func WriteProblemDetails(w http.ResponseWriter, status int, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	resp := ProblemDetails{
		Status: status,
		Title:  title,
		Detail: detail,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("httputil: failed to encode problem details response: %v", err)
	}
}

// WriteProblemBadRequest writes a 400 Bad Request using RFC 7807 format.
func WriteProblemBadRequest(w http.ResponseWriter, detail string) {
	WriteProblemDetails(w, http.StatusBadRequest, http.StatusText(http.StatusBadRequest), detail)
}

// WriteProblemNotFound writes a 404 Not Found using RFC 7807 format.
func WriteProblemNotFound(w http.ResponseWriter, detail string) {
	WriteProblemDetails(w, http.StatusNotFound, http.StatusText(http.StatusNotFound), detail)
}

// WriteProblemInternalError writes a 500 Internal Server Error using RFC 7807 format.
func WriteProblemInternalError(w http.ResponseWriter, detail string) {
	WriteProblemDetails(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), detail)
}

// WriteProblemUnauthorized writes a 401 Unauthorized using RFC 7807 format.
func WriteProblemUnauthorized(w http.ResponseWriter, detail string) {
	WriteProblemDetails(w, http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized), detail)
}

// WriteProblemForbidden writes a 403 Forbidden using RFC 7807 format.
func WriteProblemForbidden(w http.ResponseWriter, detail string) {
	WriteProblemDetails(w, http.StatusForbidden, http.StatusText(http.StatusForbidden), detail)
}
