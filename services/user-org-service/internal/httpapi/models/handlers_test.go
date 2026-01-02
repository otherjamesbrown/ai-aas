package models

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_OpenAIModelsResponse_Parsing tests that we can parse OpenAI models response
func Test_OpenAIModelsResponse_Parsing(t *testing.T) {
	jsonResp := `{
		"object": "list",
		"data": [
			{"id": "gpt-4", "object": "model", "owned_by": "openai", "created": 1234567890},
			{"id": "llama-7b", "object": "model", "owned_by": "meta", "created": 1234567890}
		]
	}`

	var resp OpenAIModelsResponse
	err := json.Unmarshal([]byte(jsonResp), &resp)
	require.NoError(t, err)

	assert.Equal(t, "list", resp.Object)
	assert.Len(t, resp.Data, 2)
	assert.Equal(t, "gpt-4", resp.Data[0].ID)
	assert.Equal(t, "openai", resp.Data[0].OwnedBy)
}

// Test_ModelResponse_Serialization tests that Model struct serializes correctly
func Test_ModelResponse_Serialization(t *testing.T) {
	model := Model{
		ID:          "gpt-4",
		Name:        "gpt-4",
		Provider:    "openai",
		Description: "GPT-4 model",
		Status:      "available",
		ContextSize: 8192,
	}

	data, err := json.Marshal(model)
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, "gpt-4", parsed["id"])
	assert.Equal(t, "gpt-4", parsed["name"])
	assert.Equal(t, "openai", parsed["provider"])
	assert.Equal(t, "available", parsed["status"])
	assert.Equal(t, float64(8192), parsed["context_size"])
}

// Test_ErrorResponse_Format tests error response structure
func Test_ErrorResponse_Format(t *testing.T) {
	errResp := ErrorResponse{
		Type:   "https://api.ai-aas.io/errors/not_found",
		Title:  "Model Not Found",
		Status: 404,
		Detail: "model not found or not accessible",
	}

	data, err := json.Marshal(errResp)
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, "Model Not Found", parsed["title"])
	assert.Equal(t, float64(404), parsed["status"])
}

// Test_ConvertOpenAIModelsToModels tests the conversion logic
func Test_ConvertOpenAIModelsToModels(t *testing.T) {
	openAIModels := []OpenAIModel{
		{ID: "gpt-4", Object: "model", OwnedBy: "openai"},
		{ID: "llama-7b", Object: "model", OwnedBy: "meta"},
		{ID: "claude-3", Object: "model", OwnedBy: ""},
	}

	models := make([]Model, 0, len(openAIModels))
	for _, m := range openAIModels {
		provider := m.OwnedBy
		if provider == "" {
			provider = "platform"
		}

		models = append(models, Model{
			ID:          m.ID,
			Name:        m.ID,
			Provider:    provider,
			Description: "",
			Status:      "available",
			ContextSize: 4096,
		})
	}

	assert.Len(t, models, 3)
	assert.Equal(t, "gpt-4", models[0].Name)
	assert.Equal(t, "openai", models[0].Provider)
	assert.Equal(t, "platform", models[2].Provider) // Empty owned_by becomes "platform"
}

// Test_Handler_writeJSON tests JSON response writing
func Test_Handler_writeJSON(t *testing.T) {
	handler := &Handler{
		runtime: nil,
		logger:  nil,
	}

	w := httptest.NewRecorder()
	data := map[string]string{"status": "ok"}

	handler.writeJSON(w, http.StatusOK, data)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp map[string]string
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp["status"])
}

// Test_Handler_writeError tests error response writing
func Test_Handler_writeError(t *testing.T) {
	handler := &Handler{
		runtime: nil,
		logger:  nil,
	}

	w := httptest.NewRecorder()

	handler.writeError(w, http.StatusNotFound, "not_found", "Not Found", "resource not found")

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var errResp ErrorResponse
	err := json.NewDecoder(w.Body).Decode(&errResp)
	require.NoError(t, err)

	assert.Equal(t, "https://api.ai-aas.io/errors/not_found", errResp.Type)
	assert.Equal(t, "Not Found", errResp.Title)
	assert.Equal(t, 404, errResp.Status)
	assert.Equal(t, "resource not found", errResp.Detail)
}
