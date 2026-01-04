// Package model provides CLI commands for model management.
package model

import (
	"testing"

	"github.com/fatih/color"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/admin"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/output"
	"github.com/stretchr/testify/assert"
)

func TestNewPipelineCommand(t *testing.T) {
	cmd := NewPipelineCommand()

	assert.NotNil(t, cmd)
	assert.Equal(t, "pipeline <model-name>", cmd.Use)
	assert.Contains(t, cmd.Short, "deployment pipeline status")
	assert.NotNil(t, cmd.RunE)

	// Check flags
	environmentFlag := cmd.Flags().Lookup("environment")
	assert.NotNil(t, environmentFlag)
	assert.Equal(t, "e", environmentFlag.Shorthand)

	formatFlag := cmd.Flags().Lookup("format")
	assert.NotNil(t, formatFlag)
	assert.Equal(t, "f", formatFlag.Shorthand)
	assert.Equal(t, "table", formatFlag.DefValue)
}

func TestGetStatusIcon(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		expected string
	}{
		{
			name:     "ok status",
			status:   "ok",
			expected: "✓",
		},
		{
			name:     "success status",
			status:   "success",
			expected: "✓",
		},
		{
			name:     "ready status",
			status:   "ready",
			expected: "✓",
		},
		{
			name:     "warning status",
			status:   "warning",
			expected: "⚠",
		},
		{
			name:     "pending status",
			status:   "pending",
			expected: "⚠",
		},
		{
			name:     "error status",
			status:   "error",
			expected: "✗",
		},
		{
			name:     "missing status",
			status:   "missing",
			expected: "✗",
		},
		{
			name:     "failed status",
			status:   "failed",
			expected: "✗",
		},
		{
			name:     "unknown status",
			status:   "unknown",
			expected: "-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getStatusIcon(tt.status)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetStatusColor(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		expected *color.Color
	}{
		{
			name:     "ok status",
			status:   "ok",
			expected: output.Success,
		},
		{
			name:     "success status",
			status:   "success",
			expected: output.Success,
		},
		{
			name:     "ready status",
			status:   "ready",
			expected: output.Success,
		},
		{
			name:     "warning status",
			status:   "warning",
			expected: output.Warning,
		},
		{
			name:     "pending status",
			status:   "pending",
			expected: output.Warning,
		},
		{
			name:     "error status",
			status:   "error",
			expected: output.Error,
		},
		{
			name:     "missing status",
			status:   "missing",
			expected: output.Error,
		},
		{
			name:     "failed status",
			status:   "failed",
			expected: output.Error,
		},
		{
			name:     "unknown status",
			status:   "unknown",
			expected: output.Muted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getStatusColor(tt.status)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDisplayPipelineStatus(t *testing.T) {
	// This is a display function - just ensure it doesn't panic
	status := &admin.PipelineStatus{
		ModelName:          "test-model",
		Environment:        "development",
		IsFullyOperational: true,
		OverallStatus:      "Model is fully operational",
		GitOps: admin.PipelineStage{
			Name:   "GitOps",
			Status: "ok",
			Checks: []admin.StageCheck{
				{
					Name:   "AIModel CR",
					Status: "ok",
					Value:  "Found",
				},
			},
		},
		Kubernetes: admin.PipelineStage{
			Name:   "Kubernetes",
			Status: "ok",
			Checks: []admin.StageCheck{
				{
					Name:   "AIModel",
					Status: "ok",
					Value:  "Ready",
				},
			},
		},
		AdminAPI: admin.PipelineStage{
			Name:   "Admin API",
			Status: "ok",
			Checks: []admin.StageCheck{
				{
					Name:   "Registry",
					Status: "ok",
					Value:  "Registered",
				},
			},
		},
		RoutingPolicy: admin.PipelineStage{
			Name:   "Routing Policy",
			Status: "ok",
			Checks: []admin.StageCheck{
				{
					Name:   "Global Policy",
					Status: "ok",
					Value:  "Exists",
				},
			},
		},
		InferenceEndpoint: admin.PipelineStage{
			Name:   "Inference Endpoint",
			Status: "ok",
			Checks: []admin.StageCheck{
				{
					Name:   "Health",
					Status: "ok",
					Value:  "Responding",
				},
			},
		},
		SuggestedActions: []string{},
	}

	// Just ensure it doesn't panic
	assert.NotPanics(t, func() {
		displayPipelineStatus(status)
	})
}

func TestDisplayStageStatus(t *testing.T) {
	stage := admin.PipelineStage{
		Name:   "Test Stage",
		Status: "ok",
		Checks: []admin.StageCheck{
			{
				Name:    "Check 1",
				Status:  "ok",
				Value:   "Success",
				Details: "Additional details",
			},
			{
				Name:   "Check 2",
				Status: "error",
				Value:  "Failed",
				Error:  "Something went wrong",
			},
		},
		Message: "Stage completed",
	}

	// Just ensure it doesn't panic
	assert.NotPanics(t, func() {
		displayStageStatus(stage)
	})
}
