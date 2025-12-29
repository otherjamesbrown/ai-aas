// Package httputil provides HTTP response utilities for API handlers.
//
// This package wraps shared/go/httputil to maintain backward compatibility
// with the legacy error response format used by admin-api-service.
package httputil

import (
	"net/http"

	sharedhttputil "github.com/ai-aas/shared-go/httputil"
)

// Error type constants (legacy - maintained for backward compatibility)
const (
	ErrorTypeValidation = "validation_error"
	ErrorTypeNotFound   = "not_found"
	ErrorTypeConflict   = "conflict"
	ErrorTypeForbidden  = "forbidden"
	ErrorTypeInternal   = "internal_error"
)

// ValidationError represents a field-level validation error.
// This is re-exported from shared/go/httputil for backward compatibility.
type ValidationError = sharedhttputil.ValidationError

// ErrorResponse represents an API error response (legacy format)
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains error details (legacy format)
type ErrorDetail struct {
	Type   string            `json:"type"`
	Title  string            `json:"title"`
	Detail string            `json:"detail,omitempty"`
	Status int               `json:"status"`
	Errors []ValidationError `json:"errors,omitempty"`
}

// WriteJSON writes a JSON response with the given status code.
// This delegates to shared/go/httputil.WriteJSON.
func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	sharedhttputil.WriteJSON(w, status, data)
}

// WriteError writes an error response using the legacy format.
// This maintains backward compatibility with existing callers.
func WriteError(w http.ResponseWriter, r *http.Request, status int, errorType, title, detail string) {
	resp := ErrorResponse{
		Error: ErrorDetail{
			Type:   errorType,
			Title:  title,
			Detail: detail,
			Status: status,
		},
	}
	WriteJSON(w, status, resp)
}

// WriteValidationError writes a validation error response using the legacy format.
// This maintains backward compatibility with existing callers.
func WriteValidationError(w http.ResponseWriter, r *http.Request, errors []ValidationError) {
	resp := ErrorResponse{
		Error: ErrorDetail{
			Type:   ErrorTypeValidation,
			Title:  "Validation Failed",
			Detail: "One or more fields failed validation",
			Status: http.StatusBadRequest,
			Errors: errors,
		},
	}
	WriteJSON(w, http.StatusBadRequest, resp)
}

// WriteInternalError writes a generic internal server error using the legacy format.
func WriteInternalError(w http.ResponseWriter, r *http.Request) {
	WriteError(w, r, http.StatusInternalServerError, ErrorTypeInternal,
		"Internal Server Error", "An unexpected error occurred")
}

// WriteNotFound writes a not found error using the legacy format.
func WriteNotFound(w http.ResponseWriter, r *http.Request, resourceType, id string) {
	WriteError(w, r, http.StatusNotFound, ErrorTypeNotFound,
		resourceType+" Not Found", resourceType+" with ID '"+id+"' was not found")
}

// WriteConflict writes a conflict error using the legacy format.
func WriteConflict(w http.ResponseWriter, r *http.Request, message string) {
	WriteError(w, r, http.StatusConflict, ErrorTypeConflict,
		"Conflict", message)
}

// WriteForbidden writes a forbidden error using the legacy format.
func WriteForbidden(w http.ResponseWriter, r *http.Request, message string) {
	WriteError(w, r, http.StatusForbidden, ErrorTypeForbidden,
		"Forbidden", message)
}
