package middleware

import (
	"bytes"
	"context"
	"io"
	"net/http"

	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/domain"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/service"
)

// AuditLogger interface for audit logging
type AuditLogger interface {
	Log(ctx context.Context, create *domain.AuditLogCreate)
}

// Audit creates an audit logging middleware for mutating operations
func Audit(auditSvc *service.AuditService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only audit mutating operations
			if r.Method != http.MethodPost && r.Method != http.MethodPatch && r.Method != http.MethodDelete {
				next.ServeHTTP(w, r)
				return
			}

			// Read and restore request body for logging
			var bodyBytes []byte
			if r.Body != nil {
				bodyBytes, _ = io.ReadAll(r.Body)
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}

			// Wrap response writer to capture status
			wrapped := &auditResponseWriter{ResponseWriter: w, status: http.StatusOK}

			// Process request
			next.ServeHTTP(wrapped, r)

			// Determine action and resource from path
			action, resourceType, resourceID := parseAuditInfo(r.Method, r.URL.Path)

			// Determine outcome
			outcome := domain.OutcomeSuccess
			var errorDetail *string
			if wrapped.status >= 400 {
				outcome = domain.OutcomeFailure
				// Could parse error from response body if needed
			}

			// Get client IP (respecting X-Forwarded-For)
			clientIP := r.RemoteAddr
			if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
				clientIP = xff
			}

			// Create audit log entry
			auditSvc.Log(r.Context(), &domain.AuditLogCreate{
				Actor:        GetAPIKeyID(r.Context()),
				Action:       action,
				ResourceType: resourceType,
				ResourceID:   resourceID,
				Outcome:      outcome,
				ErrorDetail:  errorDetail,
				ClientIP:     clientIP,
				UserAgent:    r.UserAgent(),
				RequestID:    r.Header.Get(RequestIDHeader),
			})
		})
	}
}

type auditResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *auditResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func parseAuditInfo(method, path string) (action, resourceType, resourceID string) {
	// Simple path parsing - in production would use more sophisticated routing info
	switch {
	case contains(path, "/registry/models"):
		resourceType = domain.ResourceTypeModel
		switch method {
		case http.MethodPost:
			action = domain.ActionModelCreate
		case http.MethodPatch:
			action = domain.ActionModelUpdate
		case http.MethodDelete:
			action = domain.ActionModelDelete
		}
	case contains(path, "/organizations"):
		resourceType = domain.ResourceTypeOrganization
		switch method {
		case http.MethodPost:
			action = domain.ActionOrgCreate
		case http.MethodPatch:
			action = domain.ActionOrgUpdate
		}
	case contains(path, "/routing/policies"):
		resourceType = domain.ResourceTypePolicy
		switch method {
		case http.MethodPost:
			if contains(path, "/activate") {
				action = domain.ActionPolicyActivate
			} else if contains(path, "/deactivate") {
				action = domain.ActionPolicyDeactivate
			} else {
				action = domain.ActionPolicyCreate
			}
		case http.MethodPatch:
			action = domain.ActionPolicyUpdate
		case http.MethodDelete:
			action = domain.ActionPolicyDelete
		}
	}

	// Extract resource ID from path (last segment after base)
	// This is simplified - real implementation would use chi's URL params
	resourceID = extractResourceID(path)

	return action, resourceType, resourceID
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func extractResourceID(path string) string {
	// Simple extraction - get last path segment
	lastSlash := -1
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			lastSlash = i
			break
		}
	}
	if lastSlash >= 0 && lastSlash < len(path)-1 {
		return path[lastSlash+1:]
	}
	return ""
}

