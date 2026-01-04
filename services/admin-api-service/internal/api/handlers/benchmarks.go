package handlers

import (
	"encoding/json"
	"net/http"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/api/middleware"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/domain"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/httputil"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/service"
	"go.uber.org/zap"
)

var benchmarkNameRegex = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// BenchmarkHandler handles benchmark HTTP requests
type BenchmarkHandler struct {
	svc    *service.BenchmarkService
	logger *zap.Logger
}

// NewBenchmarkHandler creates a new benchmark handler
func NewBenchmarkHandler(svc *service.BenchmarkService, logger *zap.Logger) *BenchmarkHandler {
	return &BenchmarkHandler{
		svc:    svc,
		logger: logger,
	}
}

// ============================================================================
// Scenario Handlers
// ============================================================================

// ListScenarios handles GET /v1/benchmarks/scenarios
func (h *BenchmarkHandler) ListScenarios(w http.ResponseWriter, r *http.Request) {
	params := domain.BenchmarkScenarioListParams{
		Limit:  parseIntParam(r.URL.Query().Get("limit"), 100),
		Offset: parseIntParam(r.URL.Query().Get("offset"), 0),
	}

	response, err := h.svc.ListScenarios(r.Context(), params)
	if err != nil {
		h.logger.Error("failed to list scenarios", zap.Error(err))
		httputil.WriteInternalError(w, r)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, response)
}

// GetScenario handles GET /v1/benchmarks/scenarios/{name}
func (h *BenchmarkHandler) GetScenario(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		httputil.WriteValidationError(w, r, []httputil.ValidationError{
			{Field: "name", Message: "scenario name is required"},
		})
		return
	}

	scenario, err := h.svc.GetScenario(r.Context(), name)
	if err != nil {
		h.logger.Error("failed to get scenario", zap.Error(err))
		httputil.WriteInternalError(w, r)
		return
	}

	if scenario == nil {
		httputil.WriteNotFound(w, r, "Scenario", name)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, scenario)
}

// SyncScenariosRequest represents the request body for syncing scenarios
type SyncScenariosRequest struct {
	Scenarios     []domain.BenchmarkScenarioUpsert `json:"scenarios"`
	DeleteOrphans bool                             `json:"delete_orphans,omitempty"`
}

// SyncScenarios handles POST /v1/benchmarks/scenarios/sync
// This endpoint is master key only - org API keys cannot sync scenarios
func (h *BenchmarkHandler) SyncScenarios(w http.ResponseWriter, r *http.Request) {
	// Require master key for scenario sync (platform operation)
	if !middleware.IsMasterKey(r.Context()) {
		httputil.WriteForbidden(w, r, "Only master API key can sync scenarios")
		return
	}

	var req SyncScenariosRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, r, http.StatusBadRequest, httputil.ErrorTypeValidation,
			"Invalid Request Body", "Failed to parse JSON: "+err.Error())
		return
	}

	// Validate scenarios
	var validationErrors []httputil.ValidationError
	for i, scenario := range req.Scenarios {
		if scenario.Name == "" {
			validationErrors = append(validationErrors, httputil.ValidationError{
				Field:   "scenarios[" + string(rune('0'+i)) + "].name",
				Message: "scenario name is required",
			})
		}
	}

	if len(validationErrors) > 0 {
		httputil.WriteValidationError(w, r, validationErrors)
		return
	}

	result, err := h.svc.SyncScenarios(r.Context(), req.Scenarios, req.DeleteOrphans)
	if err != nil {
		h.logger.Error("failed to sync scenarios", zap.Error(err))
		httputil.WriteInternalError(w, r)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result)
}

// ============================================================================
// Target Handlers
// ============================================================================

// CreateTarget handles POST /v1/benchmarks/targets
func (h *BenchmarkHandler) CreateTarget(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var create domain.BenchmarkTargetCreate
	if err := json.NewDecoder(r.Body).Decode(&create); err != nil {
		httputil.WriteError(w, r, http.StatusBadRequest, httputil.ErrorTypeValidation,
			"Invalid Request Body", "Failed to parse JSON: "+err.Error())
		return
	}

	// For org API keys, auto-set org_id from context (org keys cannot set arbitrary org_id)
	if orgID := middleware.GetOrgID(ctx); orgID != nil {
		create.OrgID = orgID
	}

	// Validate
	errors := h.validateTargetCreate(&create)
	if len(errors) > 0 {
		httputil.WriteValidationError(w, r, errors)
		return
	}

	target, err := h.svc.CreateTarget(ctx, &create)
	if err != nil {
		if ce, ok := err.(*service.ConflictError); ok {
			httputil.WriteConflict(w, r, ce.Message)
			return
		}
		if mae, ok := err.(*service.ModelAccessError); ok {
			httputil.WriteForbidden(w, r, mae.Message)
			return
		}
		if ve, ok := err.(*service.BenchmarkValidationError); ok {
			httputil.WriteValidationError(w, r, []httputil.ValidationError{
				{Field: "scenario_name", Message: ve.Message},
			})
			return
		}
		h.logger.Error("failed to create target", zap.Error(err))
		httputil.WriteInternalError(w, r)
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, target)
}

// ListTargets handles GET /v1/benchmarks/targets
func (h *BenchmarkHandler) ListTargets(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	params := domain.BenchmarkTargetListParams{
		Environment:  r.URL.Query().Get("environment"),
		ModelName:    r.URL.Query().Get("model_name"),
		ScenarioName: r.URL.Query().Get("scenario_name"),
		Status:       r.URL.Query().Get("status"),
		Limit:        parseIntParam(r.URL.Query().Get("limit"), 100),
		Offset:       parseIntParam(r.URL.Query().Get("offset"), 0),
	}

	// For org API keys, always filter by the org_id from context
	// Master keys can optionally filter by org_id via query param
	if orgID := middleware.GetOrgID(ctx); orgID != nil {
		// Org API key - must filter by their org
		params.OrgID = orgID
	} else if orgIDStr := r.URL.Query().Get("org_id"); orgIDStr != "" {
		// Master key with explicit org_id filter
		orgID, err := uuid.Parse(orgIDStr)
		if err != nil {
			httputil.WriteValidationError(w, r, []httputil.ValidationError{
				{Field: "org_id", Message: "Invalid organization ID format"},
			})
			return
		}
		params.OrgID = &orgID
	}

	// Validate status if provided
	if params.Status != "" && !domain.IsValidBenchmarkTargetStatus(params.Status) {
		httputil.WriteValidationError(w, r, []httputil.ValidationError{
			{Field: "status", Message: "Invalid status. Must be one of: active, paused, disabled"},
		})
		return
	}

	response, err := h.svc.ListTargets(ctx, params)
	if err != nil {
		h.logger.Error("failed to list targets", zap.Error(err))
		httputil.WriteInternalError(w, r)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, response)
}

// GetTarget handles GET /v1/benchmarks/targets/{id}
func (h *BenchmarkHandler) GetTarget(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.WriteValidationError(w, r, []httputil.ValidationError{
			{Field: "id", Message: "Invalid target ID format"},
		})
		return
	}

	target, err := h.svc.GetTarget(ctx, id)
	if err != nil {
		h.logger.Error("failed to get target", zap.Error(err))
		httputil.WriteInternalError(w, r)
		return
	}

	if target == nil {
		httputil.WriteNotFound(w, r, "Target", idStr)
		return
	}

	// Verify org ownership for org API keys
	if orgID := middleware.GetOrgID(ctx); orgID != nil {
		if !h.verifyTargetOwnership(target, orgID) {
			httputil.WriteForbidden(w, r, "You do not have access to this target")
			return
		}
	}

	httputil.WriteJSON(w, http.StatusOK, target)
}

// UpdateTarget handles PATCH /v1/benchmarks/targets/{id}
func (h *BenchmarkHandler) UpdateTarget(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.WriteValidationError(w, r, []httputil.ValidationError{
			{Field: "id", Message: "Invalid target ID format"},
		})
		return
	}

	// Verify org ownership for org API keys before any update
	if orgID := middleware.GetOrgID(ctx); orgID != nil {
		existingTarget, err := h.svc.GetTarget(ctx, id)
		if err != nil {
			h.logger.Error("failed to get target for ownership check", zap.Error(err))
			httputil.WriteInternalError(w, r)
			return
		}
		if existingTarget == nil {
			httputil.WriteNotFound(w, r, "Target", idStr)
			return
		}
		if !h.verifyTargetOwnership(existingTarget, orgID) {
			httputil.WriteForbidden(w, r, "You do not have access to this target")
			return
		}
	}

	var update domain.BenchmarkTargetUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		httputil.WriteError(w, r, http.StatusBadRequest, httputil.ErrorTypeValidation,
			"Invalid Request Body", "Failed to parse JSON: "+err.Error())
		return
	}

	// Org keys cannot change the org_id of a target
	if !middleware.IsMasterKey(ctx) {
		update.OrgID = nil // Strip any org_id from the update
	}

	// Validate
	errors := h.validateTargetUpdate(&update)
	if len(errors) > 0 {
		httputil.WriteValidationError(w, r, errors)
		return
	}

	target, err := h.svc.UpdateTarget(ctx, id, &update)
	if err != nil {
		if ve, ok := err.(*service.BenchmarkValidationError); ok {
			httputil.WriteValidationError(w, r, []httputil.ValidationError{
				{Field: "status", Message: ve.Message},
			})
			return
		}
		h.logger.Error("failed to update target", zap.Error(err))
		httputil.WriteInternalError(w, r)
		return
	}

	if target == nil {
		httputil.WriteNotFound(w, r, "Target", idStr)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, target)
}

// DeleteTarget handles DELETE /v1/benchmarks/targets/{id}
func (h *BenchmarkHandler) DeleteTarget(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.WriteValidationError(w, r, []httputil.ValidationError{
			{Field: "id", Message: "Invalid target ID format"},
		})
		return
	}

	// Check if target exists
	target, err := h.svc.GetTarget(ctx, id)
	if err != nil {
		h.logger.Error("failed to get target", zap.Error(err))
		httputil.WriteInternalError(w, r)
		return
	}

	if target == nil {
		httputil.WriteNotFound(w, r, "Target", idStr)
		return
	}

	// Verify org ownership for org API keys
	if orgID := middleware.GetOrgID(ctx); orgID != nil {
		if !h.verifyTargetOwnership(target, orgID) {
			httputil.WriteForbidden(w, r, "You do not have access to this target")
			return
		}
	}

	if err := h.svc.DeleteTarget(ctx, id); err != nil {
		h.logger.Error("failed to delete target", zap.Error(err))
		httputil.WriteInternalError(w, r)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// StartTarget handles POST /v1/benchmarks/targets/{id}/start
func (h *BenchmarkHandler) StartTarget(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.WriteValidationError(w, r, []httputil.ValidationError{
			{Field: "id", Message: "Invalid target ID format"},
		})
		return
	}

	// Verify org ownership for org API keys before starting
	if orgID := middleware.GetOrgID(ctx); orgID != nil {
		existingTarget, err := h.svc.GetTarget(ctx, id)
		if err != nil {
			h.logger.Error("failed to get target for ownership check", zap.Error(err))
			httputil.WriteInternalError(w, r)
			return
		}
		if existingTarget == nil {
			httputil.WriteNotFound(w, r, "Target", idStr)
			return
		}
		if !h.verifyTargetOwnership(existingTarget, orgID) {
			httputil.WriteForbidden(w, r, "You do not have access to this target")
			return
		}
	}

	target, err := h.svc.StartTarget(ctx, id)
	if err != nil {
		h.logger.Error("failed to start target", zap.Error(err))
		httputil.WriteError(w, r, http.StatusBadGateway, "runner_error",
			"Failed to Start Target", err.Error())
		return
	}

	if target == nil {
		httputil.WriteNotFound(w, r, "Target", idStr)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, target)
}

// StopTarget handles POST /v1/benchmarks/targets/{id}/stop
func (h *BenchmarkHandler) StopTarget(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.WriteValidationError(w, r, []httputil.ValidationError{
			{Field: "id", Message: "Invalid target ID format"},
		})
		return
	}

	// Verify org ownership for org API keys before stopping
	if orgID := middleware.GetOrgID(ctx); orgID != nil {
		existingTarget, err := h.svc.GetTarget(ctx, id)
		if err != nil {
			h.logger.Error("failed to get target for ownership check", zap.Error(err))
			httputil.WriteInternalError(w, r)
			return
		}
		if existingTarget == nil {
			httputil.WriteNotFound(w, r, "Target", idStr)
			return
		}
		if !h.verifyTargetOwnership(existingTarget, orgID) {
			httputil.WriteForbidden(w, r, "You do not have access to this target")
			return
		}
	}

	target, err := h.svc.StopTarget(ctx, id)
	if err != nil {
		h.logger.Error("failed to stop target", zap.Error(err))
		httputil.WriteError(w, r, http.StatusBadGateway, "runner_error",
			"Failed to Stop Target", err.Error())
		return
	}

	if target == nil {
		httputil.WriteNotFound(w, r, "Target", idStr)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, target)
}

// ============================================================================
// Run Handlers
// ============================================================================

// ListRuns handles GET /v1/benchmarks/runs
func (h *BenchmarkHandler) ListRuns(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	params := domain.BenchmarkRunListParams{
		ScenarioName: r.URL.Query().Get("scenario_name"),
		Status:       r.URL.Query().Get("status"),
		TriggeredBy:  r.URL.Query().Get("triggered_by"),
		Limit:        parseIntParam(r.URL.Query().Get("limit"), 100),
		Offset:       parseIntParam(r.URL.Query().Get("offset"), 0),
	}

	// For org API keys, always filter runs by the org's targets
	if orgID := middleware.GetOrgID(ctx); orgID != nil {
		params.OrgID = orgID
	}

	// Parse target_id if provided
	if targetIDStr := r.URL.Query().Get("target_id"); targetIDStr != "" {
		targetID, err := uuid.Parse(targetIDStr)
		if err != nil {
			httputil.WriteValidationError(w, r, []httputil.ValidationError{
				{Field: "target_id", Message: "Invalid target ID format"},
			})
			return
		}
		params.TargetID = &targetID
	}

	// Parse started_after if provided
	if startedAfterStr := r.URL.Query().Get("started_after"); startedAfterStr != "" {
		startedAfter, err := time.Parse(time.RFC3339, startedAfterStr)
		if err != nil {
			httputil.WriteValidationError(w, r, []httputil.ValidationError{
				{Field: "started_after", Message: "Invalid date format. Use RFC3339 format (e.g., 2024-01-01T00:00:00Z)"},
			})
			return
		}
		params.StartedAfter = &startedAfter
	}

	// Validate status if provided
	if params.Status != "" && !domain.IsValidBenchmarkRunStatus(params.Status) {
		httputil.WriteValidationError(w, r, []httputil.ValidationError{
			{Field: "status", Message: "Invalid status. Must be one of: pending, running, completed, failed, cancelled"},
		})
		return
	}

	// Validate triggered_by if provided
	if params.TriggeredBy != "" && !domain.IsValidBenchmarkTriggerType(params.TriggeredBy) {
		httputil.WriteValidationError(w, r, []httputil.ValidationError{
			{Field: "triggered_by", Message: "Invalid triggered_by. Must be one of: schedule, manual, api, ci"},
		})
		return
	}

	response, err := h.svc.ListRuns(ctx, params)
	if err != nil {
		h.logger.Error("failed to list runs", zap.Error(err))
		httputil.WriteInternalError(w, r)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, response)
}

// GetRun handles GET /v1/benchmarks/runs/{id}
func (h *BenchmarkHandler) GetRun(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.WriteValidationError(w, r, []httputil.ValidationError{
			{Field: "id", Message: "Invalid run ID format"},
		})
		return
	}

	run, err := h.svc.GetRun(ctx, id)
	if err != nil {
		h.logger.Error("failed to get run", zap.Error(err))
		httputil.WriteInternalError(w, r)
		return
	}

	if run == nil {
		httputil.WriteNotFound(w, r, "Run", idStr)
		return
	}

	// Verify org ownership for org API keys (via target)
	if orgID := middleware.GetOrgID(ctx); orgID != nil {
		target, err := h.svc.GetTarget(ctx, run.TargetID)
		if err != nil {
			h.logger.Error("failed to get target for ownership check", zap.Error(err))
			httputil.WriteInternalError(w, r)
			return
		}
		if target == nil || !h.verifyTargetOwnership(target, orgID) {
			httputil.WriteForbidden(w, r, "You do not have access to this run")
			return
		}
	}

	httputil.WriteJSON(w, http.StatusOK, run)
}

// CancelRun handles POST /v1/benchmarks/runs/{id}/cancel (UC-BM-006)
func (h *BenchmarkHandler) CancelRun(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.WriteValidationError(w, r, []httputil.ValidationError{
			{Field: "id", Message: "Invalid run ID format"},
		})
		return
	}

	// Verify org ownership for org API keys (via target)
	if orgID := middleware.GetOrgID(ctx); orgID != nil {
		run, err := h.svc.GetRun(ctx, id)
		if err != nil {
			h.logger.Error("failed to get run for ownership check", zap.Error(err))
			httputil.WriteInternalError(w, r)
			return
		}
		if run == nil {
			httputil.WriteNotFound(w, r, "Run", idStr)
			return
		}

		target, err := h.svc.GetTarget(ctx, run.TargetID)
		if err != nil {
			h.logger.Error("failed to get target for ownership check", zap.Error(err))
			httputil.WriteInternalError(w, r)
			return
		}
		if target == nil || !h.verifyTargetOwnership(target, orgID) {
			httputil.WriteForbidden(w, r, "You do not have access to this run")
			return
		}
	}

	run, err := h.svc.CancelRun(ctx, id)
	if err != nil {
		// Check if it's a validation error (UC-BM-006/AC-03: completed/failed runs)
		if validationErr, ok := err.(*service.BenchmarkValidationError); ok {
			httputil.WriteError(w, r, http.StatusBadRequest, httputil.ErrorTypeValidation,
				"Invalid Operation", validationErr.Message)
			return
		}

		h.logger.Error("failed to cancel run", zap.Error(err))
		httputil.WriteInternalError(w, r)
		return
	}

	if run == nil {
		httputil.WriteNotFound(w, r, "Run", idStr)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, run)
}

// TriggerRunRequest represents the request body for triggering a run
type TriggerRunRequest struct {
	TriggeredByUser *string `json:"triggered_by_user,omitempty"`
}

// TriggerRun handles POST /v1/benchmarks/targets/{id}/trigger
func (h *BenchmarkHandler) TriggerRun(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.WriteValidationError(w, r, []httputil.ValidationError{
			{Field: "id", Message: "Invalid target ID format"},
		})
		return
	}

	// Verify org ownership for org API keys before triggering
	if orgID := middleware.GetOrgID(ctx); orgID != nil {
		existingTarget, err := h.svc.GetTarget(ctx, id)
		if err != nil {
			h.logger.Error("failed to get target for ownership check", zap.Error(err))
			httputil.WriteInternalError(w, r)
			return
		}
		if existingTarget == nil {
			httputil.WriteNotFound(w, r, "Target", idStr)
			return
		}
		if !h.verifyTargetOwnership(existingTarget, orgID) {
			httputil.WriteForbidden(w, r, "You do not have access to this target")
			return
		}
	}

	// Parse optional request body
	var req TriggerRunRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httputil.WriteError(w, r, http.StatusBadRequest, httputil.ErrorTypeValidation,
				"Invalid Request Body", "Failed to parse JSON: "+err.Error())
			return
		}
	}

	run, err := h.svc.TriggerRun(ctx, id, req.TriggeredByUser)
	if err != nil {
		if ve, ok := err.(*service.BenchmarkValidationError); ok {
			httputil.WriteError(w, r, http.StatusBadRequest, httputil.ErrorTypeValidation,
				"Cannot Trigger Run", ve.Message)
			return
		}
		h.logger.Error("failed to trigger run", zap.Error(err))
		httputil.WriteError(w, r, http.StatusBadGateway, "runner_error",
			"Failed to Trigger Run", err.Error())
		return
	}

	if run == nil {
		httputil.WriteNotFound(w, r, "Target", idStr)
		return
	}

	httputil.WriteJSON(w, http.StatusAccepted, run)
}

// ============================================================================
// Validation Helpers
// ============================================================================

func (h *BenchmarkHandler) validateTargetCreate(create *domain.BenchmarkTargetCreate) []httputil.ValidationError {
	var errors []httputil.ValidationError

	if create.Name == "" {
		errors = append(errors, httputil.ValidationError{Field: "name", Message: "name is required"})
	} else if len(create.Name) > 100 {
		errors = append(errors, httputil.ValidationError{Field: "name", Message: "name must be 100 characters or less"})
	} else if !benchmarkNameRegex.MatchString(create.Name) {
		errors = append(errors, httputil.ValidationError{Field: "name", Message: "name must start with a letter and contain only lowercase letters, numbers, and hyphens"})
	}

	if create.ModelName == "" {
		errors = append(errors, httputil.ValidationError{Field: "model_name", Message: "model_name is required"})
	} else if len(create.ModelName) > 255 {
		errors = append(errors, httputil.ValidationError{Field: "model_name", Message: "model_name must be 255 characters or less"})
	}

	if create.ScenarioName == "" {
		errors = append(errors, httputil.ValidationError{Field: "scenario_name", Message: "scenario_name is required"})
	}

	if create.Environment == "" {
		errors = append(errors, httputil.ValidationError{Field: "environment", Message: "environment is required"})
	} else if !isValidEnvironment(create.Environment) {
		errors = append(errors, httputil.ValidationError{Field: "environment", Message: "Invalid environment. Must be one of: development, staging, production"})
	}

	if create.IntervalSeconds != nil && *create.IntervalSeconds < 60 {
		errors = append(errors, httputil.ValidationError{Field: "interval_seconds", Message: "interval_seconds must be at least 60"})
	}

	return errors
}

func (h *BenchmarkHandler) validateTargetUpdate(update *domain.BenchmarkTargetUpdate) []httputil.ValidationError {
	var errors []httputil.ValidationError

	if update.ModelName != nil && len(*update.ModelName) > 255 {
		errors = append(errors, httputil.ValidationError{Field: "model_name", Message: "model_name must be 255 characters or less"})
	}

	if update.Status != nil && !domain.IsValidBenchmarkTargetStatus(*update.Status) {
		errors = append(errors, httputil.ValidationError{Field: "status", Message: "Invalid status. Must be one of: active, paused, disabled"})
	}

	if update.IntervalSeconds != nil && *update.IntervalSeconds < 60 {
		errors = append(errors, httputil.ValidationError{Field: "interval_seconds", Message: "interval_seconds must be at least 60"})
	}

	return errors
}

func isValidEnvironment(env string) bool {
	validEnvs := []string{"development", "staging", "production"}
	for _, v := range validEnvs {
		if v == env {
			return true
		}
	}
	return false
}

// verifyTargetOwnership checks if the target belongs to the given organization
func (h *BenchmarkHandler) verifyTargetOwnership(target *domain.BenchmarkTarget, orgID *uuid.UUID) bool {
	if target == nil || orgID == nil {
		return false
	}
	// Target must have an org_id that matches the requester's org
	if target.OrgID == nil {
		return false
	}
	return *target.OrgID == *orgID
}
