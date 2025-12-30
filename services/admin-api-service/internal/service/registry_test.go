package service

import (
	"strings"
	"testing"

	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/domain"
)

func TestValidateRegistration_InvalidKServeName(t *testing.T) {
	service := &ModelRegistryService{} // minimal setup

	tests := []struct {
		name      string
		modelName string
		wantErr   bool
		errText   string
	}{
		{
			name:      "invalid - contains period",
			modelName: "model-v0.3",
			wantErr:   true,
			errText:   "invalid model name",
		},
		{
			name:      "invalid - uppercase letters",
			modelName: "GPT-4",
			wantErr:   true,
			errText:   "invalid model name",
		},
		{
			name:      "invalid - contains underscore",
			modelName: "llama_3",
			wantErr:   true,
			errText:   "invalid model name",
		},
		{
			name:      "invalid - starts with number",
			modelName: "123-model",
			wantErr:   true,
			errText:   "invalid model name",
		},
		{
			name:      "valid - lowercase with hyphens",
			modelName: "llama-3-8b",
			wantErr:   false,
		},
		{
			name:      "valid - complex name without periods",
			modelName: "mistral-7b-instruct-v03",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := &domain.ModelRegistration{
				ModelName:             tt.modelName,
				DeploymentEndpoint:    "http://test",
				DeploymentEnvironment: "development",
				DeploymentNamespace:   "default",
			}

			err := service.validateRegistration(reg)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRegistration() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && !strings.Contains(err.Error(), tt.errText) {
				t.Errorf("expected error to contain %q, got: %v", tt.errText, err)
			}
		})
	}
}
