// Package httputil provides HTTP response utilities for user-org-service handlers.
//
// This package wraps shared/go/httputil to provide a consistent error response
// format across all user-org-service endpoints.
package httputil

import (
	"net/http"

	sharedhttputil "github.com/ai-aas/shared-go/httputil"
)

// Re-export commonly used types from shared httputil for convenience
type ValidationError = sharedhttputil.ValidationError

// WriteJSON writes a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	sharedhttputil.WriteJSON(w, status, data)
}

// WriteBadRequest writes a 400 bad request response with a custom message.
func WriteBadRequest(w http.ResponseWriter, r *http.Request, message string) {
	sharedhttputil.WriteBadRequest(w, r, message)
}

// WriteUnauthorized writes a 401 unauthorized response.
func WriteUnauthorized(w http.ResponseWriter, r *http.Request, message string) {
	sharedhttputil.WriteUnauthorized(w, r, message)
}

// WriteForbidden writes a 403 forbidden response.
func WriteForbidden(w http.ResponseWriter, r *http.Request, message string) {
	sharedhttputil.WriteForbidden(w, r, message)
}

// WriteNotFound writes a 404 not found response for a missing resource.
func WriteNotFound(w http.ResponseWriter, r *http.Request, resourceType, id string) {
	sharedhttputil.WriteNotFound(w, r, resourceType, id)
}

// WriteConflict writes a 409 conflict response.
func WriteConflict(w http.ResponseWriter, r *http.Request, message string) {
	sharedhttputil.WriteConflict(w, r, message)
}

// WriteInternalError writes a 500 internal server error response.
func WriteInternalError(w http.ResponseWriter, r *http.Request) {
	sharedhttputil.WriteInternalError(w, r)
}

// WriteValidationError writes a validation error with field details.
func WriteValidationError(w http.ResponseWriter, r *http.Request, validationErrors []ValidationError) {
	sharedhttputil.WriteValidationError(w, r, validationErrors)
}

// WriteSuccess writes a successful JSON response with 200 OK.
func WriteSuccess(w http.ResponseWriter, data interface{}) {
	sharedhttputil.WriteSuccess(w, data)
}

// WriteCreated writes a successful JSON response with 201 Created.
func WriteCreated(w http.ResponseWriter, data interface{}) {
	sharedhttputil.WriteCreated(w, data)
}

// WriteNoContent writes a 204 No Content response.
func WriteNoContent(w http.ResponseWriter) {
	sharedhttputil.WriteNoContent(w)
}

// WriteServiceUnavailable writes a 503 service unavailable response.
func WriteServiceUnavailable(w http.ResponseWriter, r *http.Request, message string) {
	sharedhttputil.WriteErrorWithCode(w, r, "SERVICE_UNAVAILABLE", message)
}

// WriteNotImplemented writes a 501 not implemented response.
func WriteNotImplemented(w http.ResponseWriter, r *http.Request, message string) {
	sharedhttputil.WriteJSON(w, http.StatusNotImplemented, map[string]interface{}{
		"error": message,
		"code":  "NOT_IMPLEMENTED",
	})
}
