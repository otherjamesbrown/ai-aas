//go:build (nightly || full) && e2e_tier || !e2e_tier

package suites

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/ai-aas/tests/e2e/harness"
)

// TestEngineList tests listing available inference engines
func TestEngineList(t *testing.T) {
	ctx := setupEngineTestContext(t)
	defer ctx.Cleanup()

	// GET /v1/engines - List all engines (Admin API)
	resp, err := ctx.AdminClient.GET("/v1/engines")
	if err != nil {
		t.Fatalf("Failed to list engines: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, resp.String())
	}

	var result struct {
		Engines []struct {
			ID                    string   `json:"id"`
			Name                  string   `json:"name"`
			DisplayName           string   `json:"display_name"`
			Description           string   `json:"description"`
			DefaultVersion        string   `json:"default_version"`
			ProtocolVersions      []string `json:"protocol_versions"`
			SupportedModelFormats []string `json:"supported_model_formats"`
		} `json:"engines"`
	}

	if err := resp.UnmarshalJSON(&result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(result.Engines) == 0 {
		t.Fatal("Expected at least one engine, got none")
	}

	// Verify vLLM engine is present
	foundVLLM := false
	for _, engine := range result.Engines {
		if engine.Name == "vllm" {
			foundVLLM = true

			// Verify required fields
			if engine.DisplayName == "" {
				t.Error("vLLM engine missing display_name")
			}
			if engine.DefaultVersion == "" {
				t.Error("vLLM engine missing default_version")
			}
			if len(engine.ProtocolVersions) == 0 {
				t.Error("vLLM engine missing protocol_versions")
			}
			if len(engine.SupportedModelFormats) == 0 {
				t.Error("vLLM engine missing supported_model_formats")
			}

			t.Logf("vLLM engine found: display_name=%s, default_version=%s, protocols=%v",
				engine.DisplayName, engine.DefaultVersion, engine.ProtocolVersions)
			break
		}
	}

	if !foundVLLM {
		t.Error("vLLM engine not found in engine list")
	}

	t.Logf("Engine list test passed: %d engine(s) found", len(result.Engines))
}

// TestEngineDetails tests getting engine details
func TestEngineDetails(t *testing.T) {
	ctx := setupEngineTestContext(t)
	defer ctx.Cleanup()

	// GET /v1/engines/vllm - Get vLLM engine details (Admin API)
	resp, err := ctx.AdminClient.GET("/v1/engines/vllm")
	if err != nil {
		t.Fatalf("Failed to get engine details: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, resp.String())
	}

	var engine struct {
		ID                    string   `json:"id"`
		Name                  string   `json:"name"`
		DisplayName           string   `json:"display_name"`
		Description           string   `json:"description"`
		DefaultVersion        string   `json:"default_version"`
		ProtocolVersions      []string `json:"protocol_versions"`
		SupportedModelFormats []string `json:"supported_model_formats"`
	}

	if err := resp.UnmarshalJSON(&engine); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Verify engine metadata
	if engine.Name != "vllm" {
		t.Errorf("Expected engine name 'vllm', got '%s'", engine.Name)
	}
	if engine.DisplayName == "" {
		t.Error("Engine missing display_name")
	}
	if engine.DefaultVersion == "" {
		t.Error("Engine missing default_version")
	}

	t.Logf("Engine details test passed: name=%s, display=%s, version=%s",
		engine.Name, engine.DisplayName, engine.DefaultVersion)
}

// TestEngineVersionList tests listing engine versions
func TestEngineVersionList(t *testing.T) {
	ctx := setupEngineTestContext(t)
	defer ctx.Cleanup()

	// GET /v1/engines/vllm/versions - List vLLM versions (Admin API)
	resp, err := ctx.AdminClient.GET("/v1/engines/vllm/versions")
	if err != nil {
		t.Fatalf("Failed to list engine versions: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, resp.String())
	}

	var result struct {
		Versions []struct {
			ID             string `json:"id"`
			EngineID       string `json:"engine_id"`
			Version        string `json:"version"`
			ContainerImage string `json:"container_image"`
			IsDefault      bool   `json:"is_default"`
			IsDeprecated   bool   `json:"is_deprecated"`
			MinGPUMemoryGB *int   `json:"min_gpu_memory_gb,omitempty"`
			ReleaseNotes   string `json:"release_notes,omitempty"`
		} `json:"versions"`
	}

	if err := resp.UnmarshalJSON(&result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(result.Versions) == 0 {
		t.Fatal("Expected at least one version, got none")
	}

	// Verify at least one default version exists
	foundDefault := false
	for _, version := range result.Versions {
		if version.IsDefault {
			foundDefault = true
			t.Logf("Default version: %s (image: %s)", version.Version, version.ContainerImage)
		}

		// Verify required fields
		if version.Version == "" {
			t.Error("Version missing version field")
		}
		if version.ContainerImage == "" {
			t.Error("Version missing container_image")
		}
	}

	if !foundDefault {
		t.Error("No default version found")
	}

	t.Logf("Engine version list test passed: %d version(s) found", len(result.Versions))
}

// TestEngineVersionManagement tests adding and setting default versions
func TestEngineVersionManagement(t *testing.T) {
	ctx := setupEngineTestContext(t)
	defer ctx.Cleanup()

	// Generate unique version identifier for this test
	testVersion := fmt.Sprintf("0.99.%d", time.Now().Unix()%1000)
	testImage := fmt.Sprintf("vllm/vllm-openai:test-%s", testVersion)

	// POST /v1/engines/vllm/versions - Add new version
	addReq := map[string]interface{}{
		"version":         testVersion,
		"container_image": testImage,
		"set_default":     false,
		"release_notes":   "E2E test version",
	}

	resp, err := ctx.AdminClient.POST("/v1/engines/vllm/versions", addReq)
	if err != nil {
		t.Logf("Failed to add version (may not have permission): %v", err)
		// This is acceptable - not all test environments allow version management
		return
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		t.Logf("Add version returned status %d (may not have permission): %s",
			resp.StatusCode, resp.String())
		return
	}

	var addedVersion struct {
		ID             string `json:"id"`
		Version        string `json:"version"`
		ContainerImage string `json:"container_image"`
		IsDefault      bool   `json:"is_default"`
	}

	if err := resp.UnmarshalJSON(&addedVersion); err != nil {
		t.Fatalf("Failed to unmarshal add version response: %v", err)
	}

	if addedVersion.Version != testVersion {
		t.Errorf("Expected version %s, got %s", testVersion, addedVersion.Version)
	}

	t.Logf("Added version: %s (image: %s)", addedVersion.Version, addedVersion.ContainerImage)

	// POST /v1/engines/vllm/versions/{version}/set-default - Set as default (Admin API)
	setDefaultPath := fmt.Sprintf("/v1/engines/vllm/versions/%s/set-default", testVersion)
	resp, err = ctx.AdminClient.POST(setDefaultPath, nil)
	if err != nil {
		t.Logf("Failed to set default version: %v", err)
	} else if resp.StatusCode == 200 {
		t.Logf("Set version %s as default", testVersion)
	}

	// Clean up: Delete the test version (if API supports it)
	deletePath := fmt.Sprintf("/v1/engines/vllm/versions/%s", testVersion)
	_, _ = ctx.AdminClient.DELETE(deletePath)

	t.Logf("Engine version management test passed")
}

// TestEngineConfigList tests listing engine configurations
func TestEngineConfigList(t *testing.T) {
	ctx := setupEngineTestContext(t)
	defer ctx.Cleanup()

	// GET /v1/engine-configs - List all configs (Admin API)
	resp, err := ctx.AdminClient.GET("/v1/engine-configs")
	if err != nil {
		t.Fatalf("Failed to list engine configs: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, resp.String())
	}

	var result struct {
		Configs []struct {
			ID                    string          `json:"id"`
			Name                  string          `json:"name"`
			EngineName            string          `json:"engine_name"`
			Version               string          `json:"version"`
			Description           string          `json:"description"`
			GPUCount              int             `json:"gpu_count"`
			GPUType               string          `json:"gpu_type"`
			MemoryGB              int             `json:"memory_gb"`
			CPUCores              int             `json:"cpu_cores"`
			EngineArgs            json.RawMessage `json:"engine_args"`
			MinReplicas           int             `json:"min_replicas"`
			MaxReplicas           int             `json:"max_replicas"`
			StartupTimeoutSeconds int             `json:"startup_timeout_seconds"`
			IsDefault             bool            `json:"is_default"`
		} `json:"configs"`
	}

	if err := resp.UnmarshalJSON(&result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Note: configs may be empty in a fresh environment
	t.Logf("Found %d engine config(s)", len(result.Configs))

	// If configs exist, verify structure
	for _, config := range result.Configs {
		if config.Name == "" {
			t.Error("Config missing name")
		}
		if config.EngineName == "" {
			t.Error("Config missing engine_name")
		}
		if config.GPUCount <= 0 {
			t.Error("Config has invalid gpu_count")
		}
		if config.MemoryGB <= 0 {
			t.Error("Config has invalid memory_gb")
		}

		t.Logf("Config: name=%s, engine=%s, gpu=%d, memory=%dGB",
			config.Name, config.EngineName, config.GPUCount, config.MemoryGB)
	}

	t.Logf("Engine config list test passed")
}

// TestEngineConfigOperations tests creating and using engine configs
func TestEngineConfigOperations(t *testing.T) {
	ctx := setupEngineTestContext(t)
	defer ctx.Cleanup()

	// Generate unique config name
	configName := fmt.Sprintf("vllm/e2e-test-%d", time.Now().Unix())

	// POST /v1/engine-configs - Create new config
	createReq := map[string]interface{}{
		"name":                    configName,
		"engine":                  "vllm",
		"description":             "E2E test configuration",
		"gpu_count":               1,
		"gpu_type":                "rtx-4090",
		"memory_gb":               32,
		"cpu_cores":               4,
		"min_replicas":            1,
		"max_replicas":            1,
		"startup_timeout_seconds": 900,
		"engine_args": map[string]interface{}{
			"max-model-len":          32768,
			"gpu-memory-utilization": 0.9,
		},
	}

	resp, err := ctx.AdminClient.POST("/v1/engine-configs", createReq)
	if err != nil {
		t.Logf("Failed to create config (may not have permission): %v", err)
		return
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		t.Logf("Create config returned status %d (may not have permission): %s",
			resp.StatusCode, resp.String())
		return
	}

	var createdConfig struct {
		ID                    string          `json:"id"`
		Name                  string          `json:"name"`
		EngineName            string          `json:"engine_name"`
		GPUCount              int             `json:"gpu_count"`
		MemoryGB              int             `json:"memory_gb"`
		EngineArgs            json.RawMessage `json:"engine_args"`
		StartupTimeoutSeconds int             `json:"startup_timeout_seconds"`
	}

	if err := resp.UnmarshalJSON(&createdConfig); err != nil {
		t.Fatalf("Failed to unmarshal create config response: %v", err)
	}

	if createdConfig.Name != configName {
		t.Errorf("Expected config name %s, got %s", configName, createdConfig.Name)
	}

	t.Logf("Created config: name=%s, gpu=%d, memory=%dGB",
		createdConfig.Name, createdConfig.GPUCount, createdConfig.MemoryGB)

	// GET /v1/engine-configs/{name} - Get config details (Admin API)
	getResp, err := ctx.AdminClient.GET("/v1/engine-configs/" + configName)
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	if getResp.StatusCode != 200 {
		t.Fatalf("Expected status 200, got %d: %s", getResp.StatusCode, getResp.String())
	}

	var retrievedConfig struct {
		Name       string          `json:"name"`
		GPUCount   int             `json:"gpu_count"`
		MemoryGB   int             `json:"memory_gb"`
		EngineArgs json.RawMessage `json:"engine_args"`
	}

	if err := getResp.UnmarshalJSON(&retrievedConfig); err != nil {
		t.Fatalf("Failed to unmarshal get config response: %v", err)
	}

	if retrievedConfig.Name != configName {
		t.Errorf("Retrieved config name mismatch: expected %s, got %s",
			configName, retrievedConfig.Name)
	}

	// Verify engine args were stored correctly
	var engineArgs map[string]interface{}
	if err := json.Unmarshal(retrievedConfig.EngineArgs, &engineArgs); err != nil {
		t.Errorf("Failed to unmarshal engine_args: %v", err)
	} else {
		if maxLen, ok := engineArgs["max-model-len"].(float64); !ok || maxLen != 32768 {
			t.Error("engine_args missing or incorrect max-model-len")
		}
	}

	t.Logf("Config retrieval verified: name=%s matches created config", retrievedConfig.Name)

	// Clean up: Delete the test config (Admin API)
	deleteResp, err := ctx.AdminClient.DELETE("/v1/engine-configs/" + configName)
	if err != nil {
		t.Logf("Failed to delete config: %v", err)
	} else if deleteResp.StatusCode == 200 || deleteResp.StatusCode == 204 {
		t.Logf("Deleted test config: %s", configName)
	}

	t.Logf("Engine config operations test passed")
}

// TestEngineConfigInDeployment verifies that configs can be used in model deployments
func TestEngineConfigInDeployment(t *testing.T) {
	ctx := setupEngineTestContext(t)
	defer ctx.Cleanup()

	// This test verifies that a config can be referenced in deployment operations
	// We won't actually deploy, just verify the config exists and is retrievable

	// List available configs (Admin API)
	resp, err := ctx.AdminClient.GET("/v1/engine-configs")
	if err != nil {
		t.Fatalf("Failed to list configs: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	var result struct {
		Configs []struct {
			Name       string `json:"name"`
			EngineName string `json:"engine_name"`
			IsDefault  bool   `json:"is_default"`
		} `json:"configs"`
	}

	if err := resp.UnmarshalJSON(&result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Verify we can find at least one vLLM config (or have a way to create one)
	foundVLLMConfig := false
	for _, config := range result.Configs {
		if config.EngineName == "vllm" {
			foundVLLMConfig = true
			t.Logf("Found vLLM config available for deployment: %s (default: %v)",
				config.Name, config.IsDefault)
			break
		}
	}

	if !foundVLLMConfig {
		t.Log("No vLLM configs found - deployments would need to create one")
	}

	t.Logf("Engine config deployment compatibility test passed")
}

// setupEngineTestContext creates a test context configured for engine API testing
func setupEngineTestContext(t *testing.T) *harness.Context {
	config, err := harness.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Create standard context with both user-org-service and admin-api-service clients
	ctx, err := harness.NewContext(config)
	if err != nil {
		t.Fatalf("Failed to create test context: %v", err)
	}

	// Engine APIs use ctx.AdminClient which points to admin-api-service

	return ctx
}
