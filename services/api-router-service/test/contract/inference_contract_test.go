// Package contract provides contract tests for the API Router Service.
//
// Purpose:
//   These tests validate that request and response payloads conform to the
//   OpenAPI specification defined in specs/006-api-router-service/contracts/api-router.openapi.yaml
//
// Key Responsibilities:
//   - Validate InferenceRequest schema compliance
//   - Validate InferenceResponse schema compliance
//   - Validate ErrorResponse schema compliance
//   - Ensure all required fields are present
//   - Ensure field types match the spec
//
package contract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/xeipuuv/gojsonschema"
)

// getOpenAPISpecPath returns the path to the OpenAPI specification.
func getOpenAPISpecPath(t *testing.T) string {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}

	// Try to find workspace root by going up until we find specs directory
	dir := cwd
	for i := 0; i < 10; i++ {
		specsPath := filepath.Join(dir, "specs", "006-api-router-service", "contracts", "api-router.openapi.yaml")
		if _, err := os.Stat(specsPath); err == nil {
			return specsPath
		}
		
		// Also check for go.work as a marker of workspace root
		goWorkPath := filepath.Join(dir, "go.work")
		if _, err := os.Stat(goWorkPath); err == nil {
			// We're at workspace root, try specs path
			specsPath := filepath.Join(dir, "specs", "006-api-router-service", "contracts", "api-router.openapi.yaml")
			if _, err := os.Stat(specsPath); err == nil {
				return specsPath
			}
		}
		
		parent := filepath.Dir(dir)
		if parent == dir {
			break // Reached root
		}
		dir = parent
	}

	t.Fatalf("OpenAPI spec not found. Searched from cwd: %s", cwd)
	return ""
}

// loadOpenAPISpec loads and validates the OpenAPI specification.
func loadOpenAPISpec(t *testing.T) *openapi3.T {
	specPath := getOpenAPISpecPath(t)
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	spec, err := loader.LoadFromFile(specPath)
	if err != nil {
		t.Fatalf("failed to load OpenAPI spec: %v", err)
	}

	if err := spec.Validate(loader.Context); err != nil {
		t.Fatalf("OpenAPI spec validation failed: %v", err)
	}

	return spec
}

// TestInferenceRequestContract validates InferenceRequest schema compliance.
func TestInferenceRequestContract(t *testing.T) {
	spec := loadOpenAPISpec(t)

	// Get the InferenceRequest schema
	schemaRef := spec.Components.Schemas["InferenceRequest"]
	if schemaRef == nil {
		t.Fatal("InferenceRequest schema not found in OpenAPI spec")
	}

	schema := schemaRef.Value

	// Test valid request
	validRequest := map[string]interface{}{
		"request_id": "550e8400-e29b-41d4-a716-446655440000",
		"model":      "gpt-4",
		"payload":     "Hello, world!",
	}

	requestJSON, err := json.Marshal(validRequest)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	// Validate against schema
	if err := validateJSONAgainstSchema(t, schema, requestJSON); err != nil {
		t.Errorf("valid request failed validation: %v", err)
	}

	// Test request with all optional fields
	fullRequest := map[string]interface{}{
		"request_id":     "550e8400-e29b-41d4-a716-446655440001",
		"model":          "gpt-4",
		"payload":         "Hello, world!",
		"parameters":      map[string]interface{}{"temperature": 0.7},
		"content_type":   "text/plain",
		"metadata":        map[string]string{"source": "test"},
		"hmac_signature": "abc123",
	}

	fullRequestJSON, err := json.Marshal(fullRequest)
	if err != nil {
		t.Fatalf("failed to marshal full request: %v", err)
	}

	if err := validateJSONAgainstSchema(t, schema, fullRequestJSON); err != nil {
		t.Errorf("full request failed validation: %v", err)
	}

	// Test invalid requests (missing required fields)
	testCases := []struct {
		name    string
		request map[string]interface{}
	}{
		{
			name: "missing request_id",
			request: map[string]interface{}{
				"model":  "gpt-4",
				"payload": "Hello",
			},
		},
		{
			name: "missing model",
			request: map[string]interface{}{
				"request_id": "550e8400-e29b-41d4-a716-446655440002",
				"payload":    "Hello",
			},
		},
		{
			name: "missing payload",
			request: map[string]interface{}{
				"request_id": "550e8400-e29b-41d4-a716-446655440003",
				"model":      "gpt-4",
			},
		},
		{
			name: "invalid request_id format (not UUID)",
			request: map[string]interface{}{
				"request_id": "not-a-uuid",
				"model":      "gpt-4",
				"payload":     "Hello",
			},
		},
		{
			name: "invalid content_type",
			request: map[string]interface{}{
				"request_id":  "550e8400-e29b-41d4-a716-446655440004",
				"model":       "gpt-4",
				"payload":     "Hello",
				"content_type": "invalid/type",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			requestJSON, err := json.Marshal(tc.request)
			if err != nil {
				t.Fatalf("failed to marshal request: %v", err)
			}

			// Invalid requests should fail validation
			if err := validateJSONAgainstSchema(t, schema, requestJSON); err == nil {
				t.Errorf("expected validation to fail for invalid request: %s", tc.name)
			}
		})
	}
}

// TestInferenceResponseContract validates InferenceResponse schema compliance.
func TestInferenceResponseContract(t *testing.T) {
	spec := loadOpenAPISpec(t)

	// Get the InferenceResponse schema
	schemaRef := spec.Components.Schemas["InferenceResponse"]
	if schemaRef == nil {
		t.Fatal("InferenceResponse schema not found in OpenAPI spec")
	}

	schema := schemaRef.Value

	// Test valid response
	validResponse := map[string]interface{}{
		"request_id": "550e8400-e29b-41d4-a716-446655440000",
		"output": map[string]interface{}{
			"text": "Generated response",
		},
	}

	responseJSON, err := json.Marshal(validResponse)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	if err := validateJSONAgainstSchema(t, schema, responseJSON); err != nil {
		t.Errorf("valid response failed validation: %v", err)
	}

	// Test response with all optional fields
	fullResponse := map[string]interface{}{
		"request_id": "550e8400-e29b-41d4-a716-446655440001",
		"output": map[string]interface{}{
			"text": "Generated response",
		},
		"usage": map[string]interface{}{
			"tokens_input":  10,
			"tokens_output":  20,
			"latency_ms":    150,
			"cost_usd":      0.001,
			"limit_state":   "WITHIN_LIMIT",
		},
		"trace_id": "trace-123",
		"span_id":  "span-456",
	}

	fullResponseJSON, err := json.Marshal(fullResponse)
	if err != nil {
		t.Fatalf("failed to marshal full response: %v", err)
	}

	if err := validateJSONAgainstSchema(t, schema, fullResponseJSON); err != nil {
		t.Errorf("full response failed validation: %v", err)
	}

	// Test invalid responses (missing required fields)
	testCases := []struct {
		name     string
		response map[string]interface{}
	}{
		{
			name: "missing request_id",
			response: map[string]interface{}{
				"output": map[string]interface{}{"text": "Hello"},
			},
		},
		{
			name: "missing output",
			response: map[string]interface{}{
				"request_id": "550e8400-e29b-41d4-a716-446655440002",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			responseJSON, err := json.Marshal(tc.response)
			if err != nil {
				t.Fatalf("failed to marshal response: %v", err)
			}

			// Invalid responses should fail validation
			if err := validateJSONAgainstSchema(t, schema, responseJSON); err == nil {
				t.Errorf("expected validation to fail for invalid response: %s", tc.name)
			}
		})
	}
}

// TestUsageSummaryContract validates UsageSummary schema compliance.
func TestUsageSummaryContract(t *testing.T) {
	spec := loadOpenAPISpec(t)

	// Get the UsageSummary schema
	schemaRef := spec.Components.Schemas["UsageSummary"]
	if schemaRef == nil {
		t.Fatal("UsageSummary schema not found in OpenAPI spec")
	}

	schema := schemaRef.Value

	// Test valid usage summary
	validUsage := map[string]interface{}{
		"tokens_input":  10,
		"tokens_output": 20,
	}

	usageJSON, err := json.Marshal(validUsage)
	if err != nil {
		t.Fatalf("failed to marshal usage: %v", err)
	}

	if err := validateJSONAgainstSchema(t, schema, usageJSON); err != nil {
		t.Errorf("valid usage summary failed validation: %v", err)
	}

	// Test usage summary with all fields
	fullUsage := map[string]interface{}{
		"tokens_input":  10,
		"tokens_output": 20,
		"latency_ms":    150,
		"cost_usd":      0.001,
		"limit_state":   "WITHIN_LIMIT",
	}

	fullUsageJSON, err := json.Marshal(fullUsage)
	if err != nil {
		t.Fatalf("failed to marshal full usage: %v", err)
	}

	if err := validateJSONAgainstSchema(t, schema, fullUsageJSON); err != nil {
		t.Errorf("full usage summary failed validation: %v", err)
	}

	// Test invalid limit_state enum value
	invalidUsage := map[string]interface{}{
		"tokens_input":  10,
		"tokens_output": 20,
		"limit_state":   "INVALID_STATE",
	}

	invalidUsageJSON, err := json.Marshal(invalidUsage)
	if err != nil {
		t.Fatalf("failed to marshal invalid usage: %v", err)
	}

	if err := validateJSONAgainstSchema(t, schema, invalidUsageJSON); err == nil {
		t.Error("expected validation to fail for invalid limit_state")
	}
}

// TestErrorResponseContract validates ErrorResponse schema compliance.
func TestErrorResponseContract(t *testing.T) {
	spec := loadOpenAPISpec(t)

	// Get the ErrorResponse schema
	schemaRef := spec.Components.Schemas["ErrorResponse"]
	if schemaRef == nil {
		t.Fatal("ErrorResponse schema not found in OpenAPI spec")
	}

	schema := schemaRef.Value

	// Test valid error response
	validError := map[string]interface{}{
		"error": "Authentication failed",
		"code":  "AUTH_INVALID",
	}

	errorJSON, err := json.Marshal(validError)
	if err != nil {
		t.Fatalf("failed to marshal error: %v", err)
	}

	if err := validateJSONAgainstSchema(t, schema, errorJSON); err != nil {
		t.Errorf("valid error response failed validation: %v", err)
	}

	// Test error response with trace_id
	errorWithTrace := map[string]interface{}{
		"error":    "Backend unavailable",
		"code":     "BACKEND_ERROR",
		"trace_id": "trace-123",
	}

	errorWithTraceJSON, err := json.Marshal(errorWithTrace)
	if err != nil {
		t.Fatalf("failed to marshal error with trace: %v", err)
	}

	if err := validateJSONAgainstSchema(t, schema, errorWithTraceJSON); err != nil {
		t.Errorf("error response with trace_id failed validation: %v", err)
	}

	// Test invalid error responses (missing required fields)
	testCases := []struct {
		name  string
		error map[string]interface{}
	}{
		{
			name: "missing error",
			error: map[string]interface{}{
				"code": "AUTH_INVALID",
			},
		},
		{
			name: "missing code",
			error: map[string]interface{}{
				"error": "Authentication failed",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			errorJSON, err := json.Marshal(tc.error)
			if err != nil {
				t.Fatalf("failed to marshal error: %v", err)
			}

			// Invalid error responses should fail validation
			if err := validateJSONAgainstSchema(t, schema, errorJSON); err == nil {
				t.Errorf("expected validation to fail for invalid error response: %s", tc.name)
			}
		})
	}
}

// validateJSONAgainstSchema validates JSON against an OpenAPI schema.
// Uses gojsonschema for validation, handling basic schemas and skipping complex references.
func validateJSONAgainstSchema(t *testing.T, schema *openapi3.Schema, jsonData []byte) error {
	// First, do basic validation: check required fields
	var data map[string]interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return err
	}

	// Check required fields
	if len(schema.Required) > 0 {
		for _, reqField := range schema.Required {
			if _, ok := data[reqField]; !ok {
				return &validationError{message: "missing required field: " + reqField}
			}
		}
	}

	// If schema has no properties (might be a reference), do basic validation only
	if schema.Properties == nil || len(schema.Properties) == 0 {
		// Basic validation passed (required fields checked above)
		return nil
	}

	// Convert OpenAPI schema to JSON Schema for non-reference schemas
	schemaJSON, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("failed to marshal schema: %v", err)
	}

	// Create a JSON Schema document
	jsonSchemaDoc := map[string]interface{}{
		"$schema": "http://json-schema.org/draft-07/schema#",
	}

	// Merge schema properties
	var schemaMap map[string]interface{}
	if err := json.Unmarshal(schemaJSON, &schemaMap); err != nil {
		t.Fatalf("failed to unmarshal schema: %v", err)
	}

	// Copy relevant fields from OpenAPI schema to JSON Schema
	// Filter out properties with $ref (references) as gojsonschema can't resolve them
	if props, ok := schemaMap["properties"].(map[string]interface{}); ok {
		filteredProps := make(map[string]interface{})
		for key, value := range props {
			if propMap, ok := value.(map[string]interface{}); ok {
				// Skip properties that are references
				if _, hasRef := propMap["$ref"]; !hasRef {
					filteredProps[key] = value
				}
			} else {
				filteredProps[key] = value
			}
		}
		if len(filteredProps) > 0 {
			jsonSchemaDoc["properties"] = filteredProps
			jsonSchemaDoc["type"] = "object"
		}
	}
	if required, ok := schemaMap["required"].([]interface{}); ok {
		jsonSchemaDoc["required"] = required
	}
	if enum, ok := schemaMap["enum"].([]interface{}); ok {
		jsonSchemaDoc["enum"] = enum
	}
	if schemaType, ok := schemaMap["type"].(string); ok {
		jsonSchemaDoc["type"] = schemaType
	}
	if format, ok := schemaMap["format"].(string); ok {
		jsonSchemaDoc["format"] = format
	}

	// If no type specified, infer from properties
	if _, hasType := jsonSchemaDoc["type"]; !hasType && jsonSchemaDoc["properties"] != nil {
		jsonSchemaDoc["type"] = "object"
	}

	jsonSchemaBytes, err := json.Marshal(jsonSchemaDoc)
	if err != nil {
		t.Fatalf("failed to marshal JSON schema: %v", err)
	}

	// Validate using gojsonschema
	schemaLoader := gojsonschema.NewBytesLoader(jsonSchemaBytes)
	documentLoader := gojsonschema.NewBytesLoader(jsonData)

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return err
	}

	if !result.Valid() {
		var errMsg string
		for _, desc := range result.Errors() {
			if errMsg != "" {
				errMsg += "; "
			}
			errMsg += desc.String()
		}
		return &validationError{message: errMsg}
	}

	return nil
}

// validationError represents a schema validation error.
type validationError struct {
	message string
}

func (e *validationError) Error() string {
	return e.message
}

// TestInferenceEndpointContract validates the POST /v1/inference endpoint contract.
func TestInferenceEndpointContract(t *testing.T) {
	spec := loadOpenAPISpec(t)

	// Get the POST /v1/inference operation
	pathItem := spec.Paths.Find("/v1/inference")
	if pathItem == nil {
		t.Fatal("/v1/inference path not found in OpenAPI spec")
	}

	postOp := pathItem.Post
	if postOp == nil {
		t.Fatal("POST operation not found for /v1/inference")
	}

	// Validate request body schema
	requestBody := postOp.RequestBody
	if requestBody == nil {
		t.Fatal("request body not defined for POST /v1/inference")
	}

	content := requestBody.Value.Content["application/json"]
	if content == nil {
		t.Fatal("application/json content type not defined for request body")
	}

	if content.Schema == nil {
		t.Fatal("request body schema not defined")
	}

	// Validate response schemas
	responses := postOp.Responses
	if responses == nil {
		t.Fatal("responses not defined for POST /v1/inference")
	}

	// Check for required response codes
	requiredCodes := []string{"200", "401", "402", "429", "503"}
	for _, code := range requiredCodes {
		responseRef := responses.Value(code)
		if responseRef == nil {
			t.Errorf("required response code %s not defined", code)
			continue
		}

		// Validate response has content
		responseContent := responseRef.Value.Content["application/json"]
		if responseContent == nil {
			t.Errorf("application/json content not defined for response %s", code)
			continue
		}

		if responseContent.Schema == nil {
			t.Errorf("schema not defined for response %s", code)
		}
	}

	// Validate security requirements
	// Security is optional at operation level (can inherit from root)
	// But according to the spec, it should be defined
	if postOp.Security == nil {
		t.Error("security requirements not defined for POST /v1/inference")
	}
}

