// Package auditlogs provides HTTP handlers for audit log viewing.
//
// Purpose:
//
//	This package implements REST API handlers for listing and filtering audit
//	events. Audit logs are read-only and provide compliance visibility into
//	all state-changing operations within an organization.
//
// Dependencies:
//   - github.com/go-chi/chi/v5: HTTP router for route parameters
//   - github.com/google/uuid: UUID parsing and validation
//   - internal/bootstrap: Runtime dependencies (Postgres store, config)
//   - internal/storage/postgres: Data access layer
//
// Key Responsibilities:
//   - ListAuditLogs: GET /v1/orgs/{orgId}/audit-logs - List audit events with filters
//
// Requirements Reference:
//   - specs/005-user-org-service/spec.md#US-004 (Audit & Compliance)
//   - usecases/audit.yaml (UC-AUD-001)
//
// Debugging Notes:
//   - Only organization members can view audit logs for their org
//   - Results are paginated (default 25, max 100)
//   - Filters: action, resource_type, actor_id, start_date, end_date, limit
//   - Events are sorted by timestamp descending (newest first)
//
// Thread Safety:
//   - Handler methods are safe for concurrent use (stateless, uses runtime dependencies)
//
// Error Handling:
//   - Invalid UUID returns 400 Bad Request
//   - Unauthorized access returns 403 Forbidden
//   - Database errors return 500 Internal Server Error
package auditlogs

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/bootstrap"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/httpapi/middleware"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/httputil"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/storage/postgres"
)

// RegisterRoutes mounts audit log routes beneath /v1/orgs/{orgId}.
func RegisterRoutes(router chi.Router, rt *bootstrap.Runtime, logger *zap.Logger) {
	if rt == nil || rt.Postgres == nil {
		return
	}
	handler := &Handler{
		runtime: rt,
		logger:  logger,
	}
	// Audit logs require org:read or org:admin scope
	router.With(middleware.RequireAdminScope(logger, "org:read", "org:admin")).Get("/v1/orgs/{orgId}/audit-logs", handler.ListAuditLogs)
}

// Handler serves audit log endpoints.
type Handler struct {
	runtime *bootstrap.Runtime
	logger  *zap.Logger
}

// AuditEvent represents an audit event in API responses.
type AuditEvent struct {
	ID           string         `json:"id"`
	Timestamp    time.Time      `json:"timestamp"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type"`
	ResourceID   string         `json:"resource_id"`
	ActorType    string         `json:"actor_type"`
	ActorID      string         `json:"actor_id"`
	ActorEmail   string         `json:"actor_email"`
	IPAddress    string         `json:"ip_address,omitempty"`
	UserAgent    string         `json:"user_agent,omitempty"`
	Details      string         `json:"details,omitempty"`
	Status       string         `json:"status"`
	Resource     string         `json:"resource,omitempty"`
}

// AuditEventsResponse is the response structure for listing audit events.
type AuditEventsResponse struct {
	Events     []AuditEvent `json:"events"`
	TotalCount int          `json:"total_count"`
	HasMore    bool         `json:"has_more"`
}

// ListAuditLogs handles GET /v1/orgs/{orgId}/audit-logs
// Query parameters:
//   - action: Filter by action type (e.g., "user.created")
//   - resource_type: Filter by resource type (e.g., "apikey")
//   - actor_id: Filter by actor (UUID or email)
//   - start_date: Filter events after this date (RFC3339)
//   - end_date: Filter events before this date (RFC3339)
//   - limit: Max number of results (default 25, max 100)
//   - offset: Pagination offset (default 0)
func (h *Handler) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse org ID from path
	orgIDStr := chi.URLParam(r, "orgId")
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "INVALID_ORG_ID", "Invalid organization ID", nil)
		return
	}

	// Build filters from query parameters
	filters := postgres.AuditLogFilters{
		OrgID: orgID,
	}

	if action := r.URL.Query().Get("action"); action != "" {
		filters.Action = &action
	}

	if resourceType := r.URL.Query().Get("resource_type"); resourceType != "" {
		filters.ResourceType = &resourceType
	}

	if actorID := r.URL.Query().Get("actor_id"); actorID != "" {
		filters.ActorID = &actorID
	}

	if startDate := r.URL.Query().Get("start_date"); startDate != "" {
		t, err := time.Parse(time.RFC3339, startDate)
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "INVALID_START_DATE", "Invalid start_date format (use RFC3339)", nil)
			return
		}
		filters.StartDate = &t
	}

	if endDate := r.URL.Query().Get("end_date"); endDate != "" {
		t, err := time.Parse(time.RFC3339, endDate)
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "INVALID_END_DATE", "Invalid end_date format (use RFC3339)", nil)
			return
		}
		filters.EndDate = &t
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit <= 0 {
			httputil.WriteError(w, http.StatusBadRequest, "INVALID_LIMIT", "Invalid limit parameter", nil)
			return
		}
		filters.Limit = limit
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		offset, err := strconv.Atoi(offsetStr)
		if err != nil || offset < 0 {
			httputil.WriteError(w, http.StatusBadRequest, "INVALID_OFFSET", "Invalid offset parameter", nil)
			return
		}
		filters.Offset = offset
	}

	// Query database
	logs, totalCount, err := h.runtime.Postgres.ListAuditLogs(ctx, filters)
	if err != nil {
		h.logger.Error("failed to list audit logs",
			zap.String("org_id", orgID.String()),
			zap.Error(err))
		httputil.WriteError(w, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to retrieve audit logs", nil)
		return
	}

	// Convert to API response format
	events := make([]AuditEvent, 0, len(logs))
	for _, log := range logs {
		event := AuditEvent{
			ID:         log.EventID.String(),
			Timestamp:  log.CreatedAt,
			Action:     log.Action,
			ActorType:  log.ActorType,
			ActorID:    log.ActorID.String(),
			ActorEmail: log.ActorEmail,
			Status:     "success", // Default status
		}

		if log.TargetType != nil {
			event.ResourceType = *log.TargetType
		}

		if log.TargetID != nil {
			event.ResourceID = log.TargetID.String()
		}

		if log.Resource != nil {
			event.Resource = *log.Resource
		}

		if log.IPAddress != nil {
			event.IPAddress = *log.IPAddress
		}

		if log.UserAgent != nil {
			event.UserAgent = *log.UserAgent
		}

		// Serialize metadata to details JSON string
		if log.Metadata != nil && len(log.Metadata) > 0 {
			detailsJSON, err := json.Marshal(log.Metadata)
			if err == nil {
				event.Details = string(detailsJSON)
			}
		}

		events = append(events, event)
	}

	// Calculate if there are more results
	hasMore := false
	if filters.Limit > 0 {
		hasMore = (filters.Offset + len(events)) < totalCount
	}

	response := AuditEventsResponse{
		Events:     events,
		TotalCount: totalCount,
		HasMore:    hasMore,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("failed to encode response", zap.Error(err))
	}
}
