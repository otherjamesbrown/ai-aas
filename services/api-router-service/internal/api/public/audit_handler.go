// Package public provides audit lookup endpoints for usage records.
//
// Purpose:
//   This package implements audit endpoints for querying usage records by request ID,
//   providing visibility into request history for finance and compliance teams.
//
// Key Responsibilities:
//   - Provide audit lookup endpoint
//   - Query usage records by request ID
//   - Return structured audit information
//
// Requirements Reference:
//   - specs/006-api-router-service/spec.md#US-004 (Accurate, timely usage accounting)
//   - specs/006-api-router-service/spec.md#FR-014 (Audit trail APIs)
//
package public

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/usage"
)

// AuditHandler handles audit lookup requests.
type AuditHandler struct {
	logger      *zap.Logger
	bufferStore *usage.BufferStore
	// TODO: Add database/store for querying published records
	// For now, we'll query the buffer store and Kafka (if available)
}

// NewAuditHandler creates a new audit handler.
func NewAuditHandler(logger *zap.Logger, bufferStore *usage.BufferStore) *AuditHandler {
	return &AuditHandler{
		logger:      logger,
		bufferStore: bufferStore,
	}
}

// RegisterRoutes registers audit routes.
func (h *AuditHandler) RegisterRoutes(r chi.Router) {
	r.Get("/v1/audit/requests/{requestId}", h.GetRequestAudit)
}

// GetRequestAudit returns audit information for a specific request.
func (h *AuditHandler) GetRequestAudit(w http.ResponseWriter, r *http.Request) {
	requestID := chi.URLParam(r, "requestId")
	if requestID == "" {
		h.writeError(w, http.StatusBadRequest, fmt.Errorf("request ID required"), "INVALID_REQUEST")
		return
	}

	// For now, search in buffer store
	// In production, this would query a database or Kafka consumer
	var auditRecord *usage.UsageRecord

	if h.bufferStore != nil {
		records, err := h.bufferStore.Load()
		if err == nil {
			for _, record := range records {
				if record.RequestID == requestID {
					auditRecord = record
					break
				}
			}
		}
	}

	if auditRecord == nil {
		h.writeError(w, http.StatusNotFound, fmt.Errorf("audit record not found for request ID: %s", requestID), "NOT_FOUND")
		return
	}

	// Build audit response
	response := AuditResponse{
		RequestID:      auditRecord.RequestID,
		RecordID:       auditRecord.RecordID,
		OrganizationID: auditRecord.OrganizationID,
		APIKeyID:       auditRecord.APIKeyID,
		Model:          auditRecord.Model,
		BackendID:      auditRecord.BackendID,
		TokensInput:    auditRecord.TokensInput,
		TokensOutput:   auditRecord.TokensOutput,
		LatencyMS:      auditRecord.LatencyMS,
		CostUSD:        auditRecord.CostUSD,
		LimitState:     auditRecord.LimitState,
		DecisionReason: auditRecord.DecisionReason,
		RetryCount:     auditRecord.RetryCount,
		TraceID:        auditRecord.TraceID,
		SpanID:         auditRecord.SpanID,
		Timestamp:      auditRecord.Timestamp,
		Metadata:       auditRecord.Metadata,
	}

	if auditRecord.BudgetSnapshot != nil {
		response.BudgetSnapshot = &BudgetSnapshotResponse{
			Period:            auditRecord.BudgetSnapshot.Period,
			TokensRemaining:   auditRecord.BudgetSnapshot.TokensRemaining,
			CurrencyRemaining: auditRecord.BudgetSnapshot.CurrencyRemaining,
		}
	}

	h.writeJSON(w, http.StatusOK, response)
}

// AuditResponse represents an audit response.
type AuditResponse struct {
	RequestID       string                 `json:"request_id"`
	RecordID        string                 `json:"record_id"`
	OrganizationID  string                 `json:"organization_id"`
	APIKeyID        string                 `json:"api_key_id"`
	Model           string                 `json:"model"`
	BackendID       string                 `json:"backend_id"`
	TokensInput     int                    `json:"tokens_input"`
	TokensOutput    int                    `json:"tokens_output"`
	LatencyMS       int                    `json:"latency_ms"`
	CostUSD         float64                `json:"cost_usd"`
	LimitState      string                 `json:"limit_state"`
	DecisionReason  string                 `json:"decision_reason"`
	BudgetSnapshot  *BudgetSnapshotResponse `json:"budget_snapshot,omitempty"`
	RetryCount      int                    `json:"retry_count,omitempty"`
	TraceID         string                 `json:"trace_id,omitempty"`
	SpanID          string                 `json:"span_id,omitempty"`
	Metadata        map[string]string     `json:"metadata,omitempty"`
	Timestamp       time.Time              `json:"timestamp"`
}

// BudgetSnapshotResponse represents budget snapshot in audit response.
type BudgetSnapshotResponse struct {
	Period            string  `json:"period"`
	TokensRemaining   int     `json:"tokens_remaining"`
	CurrencyRemaining float64 `json:"currency_remaining"`
}

// writeError writes an error response.
func (h *AuditHandler) writeError(w http.ResponseWriter, statusCode int, err error, code string) {
	h.logger.Warn("audit API error",
		zap.Int("status", statusCode),
		zap.String("code", code),
		zap.Error(err),
	)

	response := map[string]interface{}{
		"error": err.Error(),
		"code":  code,
	}

	h.writeJSON(w, statusCode, response)
}

// writeJSON writes a JSON response.
func (h *AuditHandler) writeJSON(w http.ResponseWriter, statusCode int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		h.logger.Error("failed to encode JSON response", zap.Error(err))
	}
}

