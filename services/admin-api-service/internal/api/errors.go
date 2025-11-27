package api

import (
	"encoding/json"
	"net/http"
)

// ProblemDetail represents an RFC 7807 Problem Details response
type ProblemDetail struct {
	Type     string            `json:"type"`
	Title    string            `json:"title"`
	Status   int               `json:"status"`
	Detail   string            `json:"detail,omitempty"`
	Instance string            `json:"instance,omitempty"`
	Errors   []ValidationError `json:"errors,omitempty"`
}

// ValidationError represents a field-level validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

const (
	ErrorTypeValidation   = "https://docs.ai-aas.local/errors/validation-error"
	ErrorTypeNotFound     = "https://docs.ai-aas.local/errors/not-found"
	ErrorTypeConflict     = "https://docs.ai-aas.local/errors/conflict"
	ErrorTypeUnauthorized = "https://docs.ai-aas.local/errors/unauthorized"
	ErrorTypeForbidden    = "https://docs.ai-aas.local/errors/forbidden"
	ErrorTypeInternal     = "https://docs.ai-aas.local/errors/internal-error"
	ErrorTypeRateLimit    = "https://docs.ai-aas.local/errors/rate-limit"
	ErrorTypeUnavailable  = "https://docs.ai-aas.local/errors/service-unavailable"
)

// WriteError writes an RFC 7807 error response
func WriteError(w http.ResponseWriter, r *http.Request, status int, errType, title, detail string) {
	problem := ProblemDetail{
		Type:     errType,
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: r.URL.Path,
	}
	writeProblem(w, problem)
}

// WriteValidationError writes a validation error response
func WriteValidationError(w http.ResponseWriter, r *http.Request, errors []ValidationError) {
	problem := ProblemDetail{
		Type:     ErrorTypeValidation,
		Title:    "Validation Failed",
		Status:   http.StatusBadRequest,
		Detail:   "One or more validation errors occurred",
		Instance: r.URL.Path,
		Errors:   errors,
	}
	writeProblem(w, problem)
}

// WriteNotFound writes a 404 error response
func WriteNotFound(w http.ResponseWriter, r *http.Request, resourceType, resourceID string) {
	WriteError(w, r, http.StatusNotFound, ErrorTypeNotFound, "Resource Not Found",
		resourceType+" with ID '"+resourceID+"' does not exist")
}

// WriteConflict writes a 409 conflict response
func WriteConflict(w http.ResponseWriter, r *http.Request, detail string) {
	WriteError(w, r, http.StatusConflict, ErrorTypeConflict, "Conflict", detail)
}

// WriteInternalError writes a 500 error response
func WriteInternalError(w http.ResponseWriter, r *http.Request) {
	WriteError(w, r, http.StatusInternalServerError, ErrorTypeInternal, "Internal Server Error",
		"An unexpected error occurred. Please try again later.")
}

// WriteServiceUnavailable writes a 503 error response
func WriteServiceUnavailable(w http.ResponseWriter, r *http.Request, detail string) {
	w.Header().Set("Retry-After", "30")
	WriteError(w, r, http.StatusServiceUnavailable, ErrorTypeUnavailable, "Service Unavailable", detail)
}

func writeProblem(w http.ResponseWriter, problem ProblemDetail) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(problem.Status)
	json.NewEncoder(w).Encode(problem)
}

// WriteJSON writes a JSON response
func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

