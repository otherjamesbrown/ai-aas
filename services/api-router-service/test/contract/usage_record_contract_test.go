// Package contract provides contract tests for usage record schema validation.
//
// Purpose:
//   These tests validate that usage records conform to the JSON schema defined in
//   specs/006-api-router-service/contracts/usage-record.schema.yaml
//
// Key Responsibilities:
//   - Validate UsageRecord schema compliance
//   - Ensure all required fields are present
//   - Ensure field types and formats match the spec
//   - Validate enum values
//
package contract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/xeipuuv/gojsonschema"
)

// getUsageRecordSchemaPath returns the path to the usage record JSON schema.
func getUsageRecordSchemaPath(t *testing.T) string {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}

	// Try to find workspace root by going up until we find specs directory
	dir := cwd
	for i := 0; i < 10; i++ {
		schemaPath := filepath.Join(dir, "specs", "006-api-router-service", "contracts", "usage-record.schema.yaml")
		if _, err := os.Stat(schemaPath); err == nil {
			return schemaPath
		}

		// Also check for go.work as a marker of workspace root
		goWorkPath := filepath.Join(dir, "go.work")
		if _, err := os.Stat(goWorkPath); err == nil {
			schemaPath := filepath.Join(dir, "specs", "006-api-router-service", "contracts", "usage-record.schema.yaml")
			if _, err := os.Stat(schemaPath); err == nil {
				return schemaPath
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // Reached root
		}
		dir = parent
	}

	t.Fatalf("usage record schema not found. Searched from cwd: %s", cwd)
	return ""
}

// loadUsageRecordSchema loads the usage record JSON schema.
func loadUsageRecordSchema(t *testing.T) *gojsonschema.Schema {
	schemaPath := getUsageRecordSchemaPath(t)

	schemaLoader := gojsonschema.NewReferenceLoader("file://" + schemaPath)
	schema, err := gojsonschema.NewSchema(schemaLoader)
	if err != nil {
		t.Fatalf("failed to load usage record schema: %v", err)
	}

	return schema
}

// TestUsageRecordContract validates UsageRecord schema compliance.
func TestUsageRecordContract(t *testing.T) {
	schema := loadUsageRecordSchema(t)

	// Test valid usage record with all required fields
	recordID := uuid.New()
	requestID := uuid.New()
	orgID := uuid.New()
	apiKeyID := uuid.New()

	validRecord := map[string]interface{}{
		"record_id":      recordID.String(),
		"request_id":     requestID.String(),
		"organization_id": orgID.String(),
		"api_key_id":     apiKeyID.String(),
		"model":          "gpt-4o",
		"backend_id":     "backend-1",
		"tokens_input":   100,
		"tokens_output":  200,
		"latency_ms":     150,
		"cost_usd":       0.001,
		"limit_state":    "WITHIN_LIMIT",
		"decision_reason": "PRIMARY",
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
	}

	recordJSON, err := json.Marshal(validRecord)
	if err != nil {
		t.Fatalf("failed to marshal record: %v", err)
	}

	documentLoader := gojsonschema.NewBytesLoader(recordJSON)
	result, err := schema.Validate(documentLoader)
	if err != nil {
		t.Fatalf("failed to validate record: %v", err)
	}

	if !result.Valid() {
		var errMsg string
		for _, desc := range result.Errors() {
			if errMsg != "" {
				errMsg += "; "
			}
			errMsg += desc.String()
		}
		t.Errorf("valid record failed validation: %s", errMsg)
	}
}

// TestUsageRecordWithOptionalFields validates usage record with all optional fields.
func TestUsageRecordWithOptionalFields(t *testing.T) {
	schema := loadUsageRecordSchema(t)

	recordID := uuid.New()
	requestID := uuid.New()
	orgID := uuid.New()
	apiKeyID := uuid.New()

	fullRecord := map[string]interface{}{
		"record_id":      recordID.String(),
		"request_id":     requestID.String(),
		"organization_id": orgID.String(),
		"api_key_id":     apiKeyID.String(),
		"model":          "gpt-4o",
		"backend_id":     "backend-1",
		"tokens_input":   100,
		"tokens_output":  200,
		"latency_ms":     150,
		"cost_usd":       0.001,
		"limit_state":    "WITHIN_LIMIT",
		"decision_reason": "PRIMARY",
		"budget_snapshot": map[string]interface{}{
			"period":            "MONTHLY",
			"tokens_remaining":  10000,
			"currency_remaining": 50.0,
		},
		"retry_count": 0,
		"trace_id":    "trace-123",
		"span_id":     "span-456",
		"metadata": map[string]string{
			"source": "test",
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	recordJSON, err := json.Marshal(fullRecord)
	if err != nil {
		t.Fatalf("failed to marshal record: %v", err)
	}

	documentLoader := gojsonschema.NewBytesLoader(recordJSON)
	result, err := schema.Validate(documentLoader)
	if err != nil {
		t.Fatalf("failed to validate record: %v", err)
	}

	if !result.Valid() {
		var errMsg string
		for _, desc := range result.Errors() {
			if errMsg != "" {
				errMsg += "; "
			}
			errMsg += desc.String()
		}
		t.Errorf("full record failed validation: %s", errMsg)
	}
}

// TestUsageRecordMissingRequiredFields validates that missing required fields fail validation.
func TestUsageRecordMissingRequiredFields(t *testing.T) {
	schema := loadUsageRecordSchema(t)

	testCases := []struct {
		name   string
		record map[string]interface{}
	}{
		{
			name: "missing record_id",
			record: map[string]interface{}{
				"request_id":      uuid.New().String(),
				"organization_id": uuid.New().String(),
				"api_key_id":      uuid.New().String(),
				"model":           "gpt-4o",
				"backend_id":      "backend-1",
				"tokens_input":    100,
				"tokens_output":   200,
				"latency_ms":      150,
				"cost_usd":        0.001,
				"limit_state":     "WITHIN_LIMIT",
				"decision_reason": "PRIMARY",
				"timestamp":       time.Now().UTC().Format(time.RFC3339),
			},
		},
		{
			name: "missing request_id",
			record: map[string]interface{}{
				"record_id":       uuid.New().String(),
				"organization_id": uuid.New().String(),
				"api_key_id":      uuid.New().String(),
				"model":           "gpt-4o",
				"backend_id":      "backend-1",
				"tokens_input":    100,
				"tokens_output":   200,
				"latency_ms":      150,
				"cost_usd":        0.001,
				"limit_state":     "WITHIN_LIMIT",
				"decision_reason": "PRIMARY",
				"timestamp":       time.Now().UTC().Format(time.RFC3339),
			},
		},
		{
			name: "missing organization_id",
			record: map[string]interface{}{
				"record_id":      uuid.New().String(),
				"request_id":     uuid.New().String(),
				"api_key_id":     uuid.New().String(),
				"model":          "gpt-4o",
				"backend_id":     "backend-1",
				"tokens_input":   100,
				"tokens_output":  200,
				"latency_ms":     150,
				"cost_usd":       0.001,
				"limit_state":    "WITHIN_LIMIT",
				"decision_reason": "PRIMARY",
				"timestamp":      time.Now().UTC().Format(time.RFC3339),
			},
		},
		{
			name: "missing tokens_input",
			record: map[string]interface{}{
				"record_id":       uuid.New().String(),
				"request_id":      uuid.New().String(),
				"organization_id": uuid.New().String(),
				"api_key_id":      uuid.New().String(),
				"model":           "gpt-4o",
				"backend_id":      "backend-1",
				"tokens_output":   200,
				"latency_ms":      150,
				"cost_usd":        0.001,
				"limit_state":     "WITHIN_LIMIT",
				"decision_reason": "PRIMARY",
				"timestamp":       time.Now().UTC().Format(time.RFC3339),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			recordJSON, err := json.Marshal(tc.record)
			if err != nil {
				t.Fatalf("failed to marshal record: %v", err)
			}

			documentLoader := gojsonschema.NewBytesLoader(recordJSON)
			result, err := schema.Validate(documentLoader)
			if err != nil {
				t.Fatalf("failed to validate record: %v", err)
			}

			if result.Valid() {
				t.Errorf("expected validation to fail for record missing %s", tc.name)
			}
		})
	}
}

// TestUsageRecordEnumValues validates enum value constraints.
func TestUsageRecordEnumValues(t *testing.T) {
	schema := loadUsageRecordSchema(t)

	recordID := uuid.New()
	requestID := uuid.New()
	orgID := uuid.New()
	apiKeyID := uuid.New()

	baseRecord := map[string]interface{}{
		"record_id":       recordID.String(),
		"request_id":      requestID.String(),
		"organization_id": orgID.String(),
		"api_key_id":      apiKeyID.String(),
		"model":           "gpt-4o",
		"backend_id":      "backend-1",
		"tokens_input":    100,
		"tokens_output":  200,
		"latency_ms":      150,
		"cost_usd":        0.001,
		"timestamp":       time.Now().UTC().Format(time.RFC3339),
	}

	testCases := []struct {
		name           string
		limitState     string
		decisionReason string
		shouldPass     bool
	}{
		{
			name:           "valid limit_state WITHIN_LIMIT",
			limitState:     "WITHIN_LIMIT",
			decisionReason: "PRIMARY",
			shouldPass:     true,
		},
		{
			name:           "valid limit_state RATE_LIMITED",
			limitState:     "RATE_LIMITED",
			decisionReason: "PRIMARY",
			shouldPass:     true,
		},
		{
			name:           "valid limit_state BUDGET_EXCEEDED",
			limitState:     "BUDGET_EXCEEDED",
			decisionReason: "PRIMARY",
			shouldPass:     true,
		},
		{
			name:           "invalid limit_state",
			limitState:     "INVALID_STATE",
			decisionReason: "PRIMARY",
			shouldPass:     false,
		},
		{
			name:           "valid decision_reason FAILOVER",
			limitState:     "WITHIN_LIMIT",
			decisionReason: "FAILOVER",
			shouldPass:     true,
		},
		{
			name:           "invalid decision_reason",
			limitState:     "WITHIN_LIMIT",
			decisionReason: "INVALID_REASON",
			shouldPass:     false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			record := make(map[string]interface{})
			for k, v := range baseRecord {
				record[k] = v
			}
			record["limit_state"] = tc.limitState
			record["decision_reason"] = tc.decisionReason

			recordJSON, err := json.Marshal(record)
			if err != nil {
				t.Fatalf("failed to marshal record: %v", err)
			}

			documentLoader := gojsonschema.NewBytesLoader(recordJSON)
			result, err := schema.Validate(documentLoader)
			if err != nil {
				t.Fatalf("failed to validate record: %v", err)
			}

			if result.Valid() != tc.shouldPass {
				if tc.shouldPass {
					var errMsg string
					for _, desc := range result.Errors() {
						if errMsg != "" {
							errMsg += "; "
						}
						errMsg += desc.String()
					}
					t.Errorf("expected validation to pass but it failed: %s", errMsg)
				} else {
					t.Errorf("expected validation to fail but it passed")
				}
			}
		})
	}
}

// TestUsageRecordFieldTypes validates field type constraints.
func TestUsageRecordFieldTypes(t *testing.T) {
	schema := loadUsageRecordSchema(t)

	recordID := uuid.New()
	requestID := uuid.New()
	orgID := uuid.New()
	apiKeyID := uuid.New()

	baseRecord := map[string]interface{}{
		"record_id":       recordID.String(),
		"request_id":      requestID.String(),
		"organization_id": orgID.String(),
		"api_key_id":      apiKeyID.String(),
		"model":           "gpt-4o",
		"backend_id":      "backend-1",
		"tokens_input":    100,
		"tokens_output":  200,
		"latency_ms":      150,
		"cost_usd":        0.001,
		"limit_state":     "WITHIN_LIMIT",
		"decision_reason": "PRIMARY",
		"timestamp":       time.Now().UTC().Format(time.RFC3339),
	}

	testCases := []struct {
		name   string
		field  string
		value  interface{}
		shouldPass bool
	}{
		{
			name:        "tokens_input as string (invalid)",
			field:       "tokens_input",
			value:       "100",
			shouldPass:  false,
		},
		{
			name:        "tokens_input as negative (invalid)",
			field:       "tokens_input",
			value:       -1,
			shouldPass:  false,
		},
		{
			name:        "cost_usd as string (invalid)",
			field:       "cost_usd",
			value:       "0.001",
			shouldPass:  false,
		},
		{
			name:        "cost_usd as negative (invalid)",
			field:       "cost_usd",
			value:       -0.001,
			shouldPass:  false,
		},
		{
			name:        "latency_ms as string (invalid)",
			field:       "latency_ms",
			value:       "150",
			shouldPass:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			record := make(map[string]interface{})
			for k, v := range baseRecord {
				record[k] = v
			}
			record[tc.field] = tc.value

			recordJSON, err := json.Marshal(record)
			if err != nil {
				t.Fatalf("failed to marshal record: %v", err)
			}

			documentLoader := gojsonschema.NewBytesLoader(recordJSON)
			result, err := schema.Validate(documentLoader)
			if err != nil {
				t.Fatalf("failed to validate record: %v", err)
			}

			if result.Valid() != tc.shouldPass {
				if tc.shouldPass {
					var errMsg string
					for _, desc := range result.Errors() {
						if errMsg != "" {
							errMsg += "; "
						}
						errMsg += desc.String()
					}
					t.Errorf("expected validation to pass but it failed: %s", errMsg)
				} else {
					t.Errorf("expected validation to fail but it passed")
				}
			}
		})
	}
}

