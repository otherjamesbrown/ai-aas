package handlers

import (
	"net/http"
	"time"

	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/domain"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/httputil"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/service"
	"go.uber.org/zap"
)

// AuditHandler handles audit log HTTP requests
type AuditHandler struct {
	svc    *service.AuditService
	logger *zap.Logger
}

// NewAuditHandler creates a new audit handler
func NewAuditHandler(svc *service.AuditService, logger *zap.Logger) *AuditHandler {
	return &AuditHandler{
		svc:    svc,
		logger: logger,
	}
}

// List handles GET /v1/audit-logs
func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
	params := domain.AuditLogListParams{
		Actor:        r.URL.Query().Get("actor"),
		ResourceType: r.URL.Query().Get("resource_type"),
		ResourceID:   r.URL.Query().Get("resource_id"),
		Limit:        parseIntParam(r.URL.Query().Get("limit"), 100),
		Offset:       parseIntParam(r.URL.Query().Get("offset"), 0),
	}

	// Parse time filters
	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			params.From = &t
		} else {
			httputil.WriteValidationError(w, r, []httputil.ValidationError{
				{Field: "from", Message: "Invalid timestamp format. Use RFC3339 (e.g., 2025-11-26T10:00:00Z)"},
			})
			return
		}
	}

	if toStr := r.URL.Query().Get("to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			params.To = &t
		} else {
			httputil.WriteValidationError(w, r, []httputil.ValidationError{
				{Field: "to", Message: "Invalid timestamp format. Use RFC3339 (e.g., 2025-11-26T10:00:00Z)"},
			})
			return
		}
	}

	response, err := h.svc.List(r.Context(), params)
	if err != nil {
		h.logger.Error("failed to list audit logs", zap.Error(err))
		httputil.WriteInternalError(w, r)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, response)
}

