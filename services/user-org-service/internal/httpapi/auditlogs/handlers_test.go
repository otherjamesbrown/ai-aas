package auditlogs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/bootstrap"
)

func TestListAuditLogs_InvalidOrgID(t *testing.T) {
	handler := &Handler{
		runtime: &bootstrap.Runtime{},
		logger:  zap.NewNop(),
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/orgs/invalid-uuid/audit-logs", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orgId", "invalid-uuid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler.ListAuditLogs(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "INVALID_ORG_ID", response["code"])
}

func TestListAuditLogs_InvalidLimit(t *testing.T) {
	orgID := uuid.New()
	handler := &Handler{
		runtime: &bootstrap.Runtime{},
		logger:  zap.NewNop(),
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/orgs/"+orgID.String()+"/audit-logs?limit=invalid", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orgId", orgID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler.ListAuditLogs(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "INVALID_LIMIT", response["code"])
}

func TestListAuditLogs_InvalidStartDate(t *testing.T) {
	orgID := uuid.New()
	handler := &Handler{
		runtime: &bootstrap.Runtime{},
		logger:  zap.NewNop(),
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/orgs/"+orgID.String()+"/audit-logs?start_date=not-a-date", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orgId", orgID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler.ListAuditLogs(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "INVALID_START_DATE", response["code"])
}
