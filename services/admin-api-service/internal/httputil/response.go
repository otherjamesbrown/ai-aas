// Package httputil provides HTTP response utilities for API handlers.
package httputil

import (
	"encoding/json"
	"net/http"
)

// Error type constants
const (
	ErrorTypeValidation = "validation_error"
	ErrorTypeNotFound   = "not_found"
	ErrorTypeConflict   = "conflict"
	ErrorTypeInternal   = "internal_error"
)

// ValidationError represents a field-level validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains error details
type ErrorDetail struct {
	Type    string            `json:"type"`
	Title   string            `json:"title"`
	Detail  string            `json:"detail,omitempty"`
	Status  int               `json:"status"`
	Errors  []ValidationError `json:"errors,omitempty"`
}

// WriteJSON writes a JSON response with the given status code
func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

// WriteError writes an error response
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

// WriteValidationError writes a validation error response
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

// WriteInternalError writes a generic internal server error
func WriteInternalError(w http.ResponseWriter, r *http.Request) {
	WriteError(w, r, http.StatusInternalServerError, ErrorTypeInternal,
		"Internal Server Error", "An unexpected error occurred")
}

// WriteNotFound writes a not found error
func WriteNotFound(w http.ResponseWriter, r *http.Request, resourceType, id string) {
	WriteError(w, r, http.StatusNotFound, ErrorTypeNotFound,
		resourceType+" Not Found", resourceType+" with ID '"+id+"' was not found")
}

// WriteConflict writes a conflict error
func WriteConflict(w http.ResponseWriter, r *http.Request, message string) {
	WriteError(w, r, http.StatusConflict, ErrorTypeConflict,
		"Conflict", message)
}
