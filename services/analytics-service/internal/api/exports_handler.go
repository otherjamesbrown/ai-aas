// Package api provides HTTP handlers for export job management.
package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/shared/go/auth"
	"github.com/otherjamesbrown/ai-aas/services/analytics-service/internal/exports"
)

// ExportsHandler handles export job management API requests.
type ExportsHandler struct {
	repo   *exports.ExportJobRepository
	logger *zap.Logger
}

// NewExportsHandler creates a new exports handler.
func NewExportsHandler(pool *pgxpool.Pool, logger *zap.Logger) *ExportsHandler {
	repo := exports.NewExportJobRepository(pool)
	return &ExportsHandler{
		repo:   repo,
		logger: logger,
	}
}

// CreateExportJob handles POST /analytics/v1/orgs/{orgId}/exports
func (h *ExportsHandler) CreateExportJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse org ID
	orgIDStr := chi.URLParam(r, "orgId")
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid org_id", err)
		return
	}

	// Parse request body
	var req CreateExportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	// Validate time range
	if req.TimeRange.Start.IsZero() || req.TimeRange.End.IsZero() {
		h.respondError(w, http.StatusBadRequest, "timeRange.start and timeRange.end are required", nil)
		return
	}

	if req.TimeRange.End.Before(req.TimeRange.Start) {
		h.respondError(w, http.StatusBadRequest, "timeRange.end must be after timeRange.start", nil)
		return
	}

	// Validate max 31 days
	maxDuration := 31 * 24 * time.Hour
	if req.TimeRange.End.Sub(req.TimeRange.Start) > maxDuration {
		h.respondError(w, http.StatusBadRequest, "time range cannot exceed 31 days", nil)
		return
	}

	// Validate granularity
	granularity := req.Granularity
	if granularity == "" {
		granularity = "daily" // Default per OpenAPI spec
	}
	if granularity != "hourly" && granularity != "daily" && granularity != "monthly" {
		h.respondError(w, http.StatusBadRequest, "granularity must be 'hourly', 'daily', or 'monthly'", nil)
		return
	}

	// Extract requested_by from auth context
	var requestedBy uuid.UUID
	if actor, ok := auth.ActorFromContext(ctx); ok && actor.Subject != "" {
		// Try to parse actor subject as UUID
		if parsedUUID, err := uuid.Parse(actor.Subject); err == nil {
			requestedBy = parsedUUID
		} else {
			// If subject is not a UUID, generate one based on subject (deterministic)
			// In production, you might want to look up user ID from user-org-service
			h.logger.Warn("actor subject is not a UUID, using generated UUID",
				zap.String("subject", actor.Subject),
			)
			requestedBy = uuid.New() // Fallback for non-UUID subjects
		}
	} else {
		// No actor in context - this shouldn't happen if RBAC is working
		h.logger.Warn("no actor found in context, using generated UUID")
		requestedBy = uuid.New()
	}

	// Create export job
	jobID, err := h.repo.CreateExportJob(ctx, exports.CreateExportJobRequest{
		OrgID:          orgID,
		RequestedBy:    requestedBy,
		TimeRangeStart: req.TimeRange.Start,
		TimeRangeEnd:   req.TimeRange.End,
		Granularity:    granularity,
	})
	if err != nil {
		h.logger.Error("failed to create export job", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "failed to create export job", err)
		return
	}

	// Fetch created job to return full details
	job, err := h.repo.GetExportJob(ctx, orgID, jobID)
	if err != nil {
		h.logger.Error("failed to get created export job", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "failed to retrieve export job", err)
		return
	}

	// Build response
	response := convertExportJob(job)
	h.respondJSON(w, http.StatusAccepted, response)
}

// ListExportJobs handles GET /analytics/v1/orgs/{orgId}/exports
func (h *ExportsHandler) ListExportJobs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse org ID
	orgIDStr := chi.URLParam(r, "orgId")
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid org_id", err)
		return
	}

	// Parse optional status filter
	statusFilter := r.URL.Query().Get("status")
	var statusPtr *string
	if statusFilter != "" {
		// Validate status
		validStatuses := map[string]bool{
			"pending":   true,
			"running":   true,
			"succeeded": true,
			"failed":    true,
			"expired":   true,
		}
		if !validStatuses[statusFilter] {
			h.respondError(w, http.StatusBadRequest, "invalid status filter", nil)
			return
		}
		statusPtr = &statusFilter
	}

	// List export jobs
	jobs, err := h.repo.ListExportJobs(ctx, orgID, statusPtr)
	if err != nil {
		h.logger.Error("failed to list export jobs", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "failed to list export jobs", err)
		return
	}

	// Convert to response format
	items := make([]ExportJobResponse, len(jobs))
	for i, job := range jobs {
		items[i] = convertExportJob(&job)
	}

	response := ListExportJobsResponse{
		Items: items,
	}

	h.respondJSON(w, http.StatusOK, response)
}

// GetExportJob handles GET /analytics/v1/orgs/{orgId}/exports/{jobId}
func (h *ExportsHandler) GetExportJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse org ID
	orgIDStr := chi.URLParam(r, "orgId")
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid org_id", err)
		return
	}

	// Parse job ID
	jobIDStr := chi.URLParam(r, "jobId")
	jobID, err := uuid.Parse(jobIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid job_id", err)
		return
	}

	// Get export job
	job, err := h.repo.GetExportJob(ctx, orgID, jobID)
	if err != nil {
		h.logger.Error("failed to get export job", zap.Error(err))
		h.respondError(w, http.StatusNotFound, "export job not found", err)
		return
	}

	// Build response
	response := convertExportJob(job)
	h.respondJSON(w, http.StatusOK, response)
}

// GetExportDownloadUrl handles GET /analytics/v1/orgs/{orgId}/exports/{jobId}/download
func (h *ExportsHandler) GetExportDownloadUrl(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse org ID
	orgIDStr := chi.URLParam(r, "orgId")
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid org_id", err)
		return
	}

	// Parse job ID
	jobIDStr := chi.URLParam(r, "jobId")
	jobID, err := uuid.Parse(jobIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid job_id", err)
		return
	}

	// Get export job
	job, err := h.repo.GetExportJob(ctx, orgID, jobID)
	if err != nil {
		h.logger.Error("failed to get export job", zap.Error(err))
		h.respondError(w, http.StatusNotFound, "export job not found", err)
		return
	}

	// Only allow download for succeeded jobs
	if job.Status != "succeeded" {
		h.respondError(w, http.StatusNotFound, "export job is not ready for download", nil)
		return
	}

	if job.OutputURI == nil {
		h.respondError(w, http.StatusNotFound, "export job output URI not available", nil)
		return
	}

	// Redirect to signed URL
	w.Header().Set("Location", *job.OutputURI)
	w.WriteHeader(http.StatusFound)
}

// Request/Response types matching OpenAPI schema

type CreateExportRequest struct {
	TimeRange   TimeRangeRequest `json:"timeRange"`
	Granularity string           `json:"granularity,omitempty"`
	Models      []uuid.UUID      `json:"models,omitempty"`
	Delivery    *DeliveryRequest `json:"delivery,omitempty"`
}

type TimeRangeRequest struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type DeliveryRequest struct {
	Type   string `json:"type,omitempty"`
	Bucket string `json:"bucket,omitempty"`
}

type ExportJobResponse struct {
	JobID       string          `json:"jobId"`
	OrgID       string          `json:"orgId"`
	Status      string          `json:"status"`
	Granularity string          `json:"granularity"`
	TimeRange   TimeRangeResponse `json:"timeRange"`
	CreatedAt   string          `json:"createdAt"`
	CompletedAt *string          `json:"completedAt,omitempty"`
	OutputURI   *string         `json:"outputUri,omitempty"`
	Checksum    *string         `json:"checksum,omitempty"`
	RowCount    *int64          `json:"rowCount,omitempty"`
	InitiatedBy string          `json:"initiatedBy"`
	Error       *string         `json:"error,omitempty"`
}

type TimeRangeResponse struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type ListExportJobsResponse struct {
	Items []ExportJobResponse `json:"items"`
}

// Helper functions

func convertExportJob(job *exports.ExportJob) ExportJobResponse {
	response := ExportJobResponse{
		JobID:       job.JobID.String(),
		OrgID:       job.OrgID.String(),
		Status:      job.Status,
		Granularity: job.Granularity,
		TimeRange: TimeRangeResponse{
			Start: job.TimeRangeStart.Format(time.RFC3339),
			End:   job.TimeRangeEnd.Format(time.RFC3339),
		},
		CreatedAt:   job.InitiatedAt.Format(time.RFC3339),
		InitiatedBy: job.RequestedBy.String(),
	}

	if job.CompletedAt != nil {
		completedAt := job.CompletedAt.Format(time.RFC3339)
		response.CompletedAt = &completedAt
	}

	if job.OutputURI != nil {
		response.OutputURI = job.OutputURI
	}

	if job.Checksum != nil {
		response.Checksum = job.Checksum
	}

	if job.RowCount != nil {
		response.RowCount = job.RowCount
	}

	if job.ErrorMessage != nil {
		response.Error = job.ErrorMessage
	}

	return response
}

func (h *ExportsHandler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode response", zap.Error(err))
	}
}

func (h *ExportsHandler) respondError(w http.ResponseWriter, status int, message string, err error) {
	if err != nil {
		h.logger.Warn(message, zap.Error(err), zap.Int("status", status))
	} else {
		h.logger.Warn(message, zap.Int("status", status))
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": status,
		"title":  http.StatusText(status),
		"detail": message,
	})
}

