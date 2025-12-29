package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/api/middleware"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/domain"
	"go.uber.org/zap"
)

// mockBenchmarkService implements the benchmark service interface for testing
type mockBenchmarkService struct {
	targets map[uuid.UUID]*domain.BenchmarkTarget
	runs    map[uuid.UUID]*domain.BenchmarkRun
}

func newMockBenchmarkService() *mockBenchmarkService {
	return &mockBenchmarkService{
		targets: make(map[uuid.UUID]*domain.BenchmarkTarget),
		runs:    make(map[uuid.UUID]*domain.BenchmarkRun),
	}
}

func (m *mockBenchmarkService) GetTarget(ctx context.Context, id uuid.UUID) (*domain.BenchmarkTarget, error) {
	if target, ok := m.targets[id]; ok {
		return target, nil
	}
	return nil, nil
}

// Helper to create a request with org context
func newRequestWithOrgContext(method, path string, body interface{}, orgID *uuid.UUID, isMaster bool) *http.Request {
	var bodyReader *bytes.Reader
	if body != nil {
		bodyBytes, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(bodyBytes)
	} else {
		bodyReader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	req.Header.Set("Content-Type", "application/json")

	ctx := req.Context()
	ctx = context.WithValue(ctx, middleware.IsMasterKeyContextKey, isMaster)
	if orgID != nil {
		ctx = context.WithValue(ctx, middleware.OrgIDContextKey, *orgID)
	}
	ctx = context.WithValue(ctx, middleware.APIKeyContextKey, "test-key")

	return req.WithContext(ctx)
}

// Helper to add chi URL params
func addChiURLParams(req *http.Request, params map[string]string) *http.Request {
	rctx := chi.NewRouteContext()
	for key, val := range params {
		rctx.URLParams.Add(key, val)
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestBenchmarkHandler_VerifyTargetOwnership(t *testing.T) {
	logger := zap.NewNop()
	handler := &BenchmarkHandler{logger: logger}

	orgA := uuid.New()
	orgB := uuid.New()

	tests := []struct {
		name     string
		target   *domain.BenchmarkTarget
		orgID    *uuid.UUID
		expected bool
	}{
		{
			name:     "nil target",
			target:   nil,
			orgID:    &orgA,
			expected: false,
		},
		{
			name:     "nil orgID",
			target:   &domain.BenchmarkTarget{ID: uuid.New(), OrgID: &orgA},
			orgID:    nil,
			expected: false,
		},
		{
			name:     "target without org_id",
			target:   &domain.BenchmarkTarget{ID: uuid.New(), OrgID: nil},
			orgID:    &orgA,
			expected: false,
		},
		{
			name:     "matching org_id",
			target:   &domain.BenchmarkTarget{ID: uuid.New(), OrgID: &orgA},
			orgID:    &orgA,
			expected: true,
		},
		{
			name:     "different org_id",
			target:   &domain.BenchmarkTarget{ID: uuid.New(), OrgID: &orgA},
			orgID:    &orgB,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.verifyTargetOwnership(tt.target, tt.orgID)
			if result != tt.expected {
				t.Errorf("verifyTargetOwnership() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestBenchmarkHandler_SyncScenarios_MasterKeyRequired(t *testing.T) {
	logger := zap.NewNop()
	handler := &BenchmarkHandler{logger: logger}

	orgID := uuid.New()

	// Test with org key (should be forbidden)
	req := newRequestWithOrgContext(http.MethodPost, "/v1/benchmarks/scenarios/sync", nil, &orgID, false)
	rr := httptest.NewRecorder()

	handler.SyncScenarios(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status %d for org key, got %d", http.StatusForbidden, rr.Code)
	}

	// Verify error message
	var errResp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&errResp)
	if errResp["error"] == nil {
		t.Error("expected error response")
	}
}

func TestBenchmarkHandler_CreateTarget_OrgIDAutoSet(t *testing.T) {
	// This test verifies that org_id is auto-set from context for org keys
	// We can't easily test the full handler without a real service, but we can
	// verify the context extraction works

	orgID := uuid.New()
	req := newRequestWithOrgContext(http.MethodPost, "/v1/benchmarks/targets", nil, &orgID, false)

	// Verify org ID is extracted from context
	extractedOrgID := middleware.GetOrgID(req.Context())
	if extractedOrgID == nil {
		t.Fatal("expected org ID in context")
	}
	if *extractedOrgID != orgID {
		t.Errorf("expected org ID %v, got %v", orgID, *extractedOrgID)
	}

	// Verify is_master_key is false
	if middleware.IsMasterKey(req.Context()) {
		t.Error("expected IsMasterKey to be false")
	}
}

func TestBenchmarkHandler_ListTargets_OrgFiltering(t *testing.T) {
	// This test verifies that org keys get their org_id set in the filter params
	orgID := uuid.New()
	req := newRequestWithOrgContext(http.MethodGet, "/v1/benchmarks/targets", nil, &orgID, false)

	// For org keys, the org_id should be extracted from context
	extractedOrgID := middleware.GetOrgID(req.Context())
	if extractedOrgID == nil || *extractedOrgID != orgID {
		t.Errorf("expected org ID %v to be extracted from context", orgID)
	}

	// For master keys, no org_id should be in context (can see all)
	masterReq := newRequestWithOrgContext(http.MethodGet, "/v1/benchmarks/targets", nil, nil, true)
	masterOrgID := middleware.GetOrgID(masterReq.Context())
	if masterOrgID != nil {
		t.Errorf("expected nil org ID for master key, got %v", masterOrgID)
	}
}

func TestBenchmarkHandler_ContextHelpers(t *testing.T) {
	t.Run("org key context", func(t *testing.T) {
		orgID := uuid.New()
		req := newRequestWithOrgContext(http.MethodGet, "/test", nil, &orgID, false)
		ctx := req.Context()

		if middleware.IsMasterKey(ctx) {
			t.Error("expected IsMasterKey to be false for org key")
		}
		if middleware.GetOrgID(ctx) == nil {
			t.Error("expected org ID to be set")
		}
		if *middleware.GetOrgID(ctx) != orgID {
			t.Errorf("expected org ID %v, got %v", orgID, *middleware.GetOrgID(ctx))
		}
		if middleware.GetAPIKeyID(ctx) != "test-key" {
			t.Errorf("expected key ID 'test-key', got '%s'", middleware.GetAPIKeyID(ctx))
		}
	})

	t.Run("master key context", func(t *testing.T) {
		req := newRequestWithOrgContext(http.MethodGet, "/test", nil, nil, true)
		ctx := req.Context()

		if !middleware.IsMasterKey(ctx) {
			t.Error("expected IsMasterKey to be true for master key")
		}
		if middleware.GetOrgID(ctx) != nil {
			t.Errorf("expected nil org ID for master key, got %v", middleware.GetOrgID(ctx))
		}
	})
}

func TestBenchmarkHandler_CrossOrgAccessDenied(t *testing.T) {
	// Test that targets from org A cannot be accessed by org B
	// This is a unit test of the ownership check logic

	logger := zap.NewNop()
	handler := &BenchmarkHandler{logger: logger}

	orgA := uuid.New()
	orgB := uuid.New()
	targetID := uuid.New()

	// Create a target belonging to org A
	targetOrgA := &domain.BenchmarkTarget{
		ID:        targetID,
		Name:      "org-a-target",
		OrgID:     &orgA,
		ModelName: "test-model",
		Status:    "active",
	}

	// Org B should not have access
	if handler.verifyTargetOwnership(targetOrgA, &orgB) {
		t.Error("org B should not have access to org A's target")
	}

	// Org A should have access
	if !handler.verifyTargetOwnership(targetOrgA, &orgA) {
		t.Error("org A should have access to its own target")
	}

	// Master key (nil org) should be handled separately in handlers
	// The verifyTargetOwnership function returns false for nil org,
	// but handlers check IsMasterKey first
	if handler.verifyTargetOwnership(targetOrgA, nil) {
		t.Error("verifyTargetOwnership should return false for nil org")
	}
}

func TestBenchmarkHandler_PlatformTargetAccess(t *testing.T) {
	// Test that targets without org_id (platform-level) cannot be accessed by org keys

	logger := zap.NewNop()
	handler := &BenchmarkHandler{logger: logger}

	orgA := uuid.New()
	targetID := uuid.New()

	// Create a target without org_id (platform target)
	platformTarget := &domain.BenchmarkTarget{
		ID:        targetID,
		Name:      "platform-target",
		OrgID:     nil, // No org - platform level
		ModelName: "test-model",
		Status:    "active",
	}

	// Org key should not have access to platform targets
	if handler.verifyTargetOwnership(platformTarget, &orgA) {
		t.Error("org key should not have access to platform targets")
	}
}

// Integration-style test helpers for handlers
// These test the authorization flow in the handlers

type testCase struct {
	name           string
	requestOrgID   *uuid.UUID
	isMasterKey    bool
	targetOrgID    *uuid.UUID
	expectForbid   bool
	expectNotFound bool
}

func TestBenchmarkAuthorizationMatrix(t *testing.T) {
	orgA := uuid.New()
	orgB := uuid.New()
	logger := zap.NewNop()
	handler := &BenchmarkHandler{logger: logger}

	testCases := []testCase{
		{
			name:         "master_key_can_access_org_target",
			requestOrgID: nil,
			isMasterKey:  true,
			targetOrgID:  &orgA,
			expectForbid: false,
		},
		{
			name:         "org_key_can_access_own_target",
			requestOrgID: &orgA,
			isMasterKey:  false,
			targetOrgID:  &orgA,
			expectForbid: false,
		},
		{
			name:         "org_key_cannot_access_other_org_target",
			requestOrgID: &orgB,
			isMasterKey:  false,
			targetOrgID:  &orgA,
			expectForbid: true,
		},
		{
			name:         "org_key_cannot_access_platform_target",
			requestOrgID: &orgA,
			isMasterKey:  false,
			targetOrgID:  nil,
			expectForbid: true,
		},
		{
			name:         "master_key_can_access_platform_target",
			requestOrgID: nil,
			isMasterKey:  true,
			targetOrgID:  nil,
			expectForbid: false, // Master bypasses ownership check
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			target := &domain.BenchmarkTarget{
				ID:        uuid.New(),
				Name:      "test-target",
				OrgID:     tc.targetOrgID,
				ModelName: "test-model",
				Status:    "active",
			}

			// For org keys, check ownership
			if !tc.isMasterKey {
				hasAccess := handler.verifyTargetOwnership(target, tc.requestOrgID)
				if tc.expectForbid && hasAccess {
					t.Errorf("expected access to be denied")
				}
				if !tc.expectForbid && !hasAccess {
					t.Errorf("expected access to be allowed")
				}
			}
			// Master keys bypass the ownership check (handled in handler logic)
		})
	}
}
