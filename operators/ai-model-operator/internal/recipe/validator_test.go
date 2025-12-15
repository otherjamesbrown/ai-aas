/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package recipe

import (
	"testing"

	aimodelv1alpha1 "github.com/ai-aas/ai-model-operator/api/v1alpha1"
)

func TestValidator_Validate(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name    string
		spec    *aimodelv1alpha1.ModelRecipeSpec
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid vllm recipe",
			spec: &aimodelv1alpha1.ModelRecipeSpec{
				ModelID: "meta-llama/Llama-2-7b-hf",
				Runtime: "vllm",
				Resources: aimodelv1alpha1.RecipeResources{
					GPU: aimodelv1alpha1.GPUResources{
						Vendor: "nvidia",
						Count:  1,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid triton recipe",
			spec: &aimodelv1alpha1.ModelRecipeSpec{
				ModelID: "meta-llama/Llama-2-7b-hf",
				Runtime: "triton",
				Resources: aimodelv1alpha1.RecipeResources{
					GPU: aimodelv1alpha1.GPUResources{
						Vendor: "nvidia",
						Count:  2,
					},
				},
				RuntimeArgs: aimodelv1alpha1.RuntimeArgsSpec{
					Triton: &aimodelv1alpha1.TritonArgs{
						Backend: "python",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid tgi recipe",
			spec: &aimodelv1alpha1.ModelRecipeSpec{
				ModelID: "meta-llama/Llama-2-7b-hf",
				Runtime: "tgi",
				Resources: aimodelv1alpha1.RecipeResources{
					GPU: aimodelv1alpha1.GPUResources{
						Vendor: "nvidia",
						Count:  1,
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "nil spec",
			spec:    nil,
			wantErr: true,
			errMsg:  "spec is nil",
		},
		{
			name: "missing modelID",
			spec: &aimodelv1alpha1.ModelRecipeSpec{
				Runtime: "vllm",
				Resources: aimodelv1alpha1.RecipeResources{
					GPU: aimodelv1alpha1.GPUResources{
						Count: 1,
					},
				},
			},
			wantErr: true,
			errMsg:  "modelID is required",
		},
		{
			name: "missing runtime",
			spec: &aimodelv1alpha1.ModelRecipeSpec{
				ModelID: "meta-llama/Llama-2-7b-hf",
				Resources: aimodelv1alpha1.RecipeResources{
					GPU: aimodelv1alpha1.GPUResources{
						Count: 1,
					},
				},
			},
			wantErr: true,
			errMsg:  "runtime is required",
		},
		{
			name: "invalid runtime",
			spec: &aimodelv1alpha1.ModelRecipeSpec{
				ModelID: "meta-llama/Llama-2-7b-hf",
				Runtime: "invalid",
				Resources: aimodelv1alpha1.RecipeResources{
					GPU: aimodelv1alpha1.GPUResources{
						Count: 1,
					},
				},
			},
			wantErr: true,
			errMsg:  "runtime must be one of: vllm, triton, tgi",
		},
		{
			name: "invalid GPU count",
			spec: &aimodelv1alpha1.ModelRecipeSpec{
				ModelID: "meta-llama/Llama-2-7b-hf",
				Runtime: "vllm",
				Resources: aimodelv1alpha1.RecipeResources{
					GPU: aimodelv1alpha1.GPUResources{
						Count: 0,
					},
				},
			},
			wantErr: true,
			errMsg:  "gpu.count must be >= 1",
		},
		{
			name: "invalid GPU vendor",
			spec: &aimodelv1alpha1.ModelRecipeSpec{
				ModelID: "meta-llama/Llama-2-7b-hf",
				Runtime: "vllm",
				Resources: aimodelv1alpha1.RecipeResources{
					GPU: aimodelv1alpha1.GPUResources{
						Vendor: "invalid",
						Count:  1,
					},
				},
			},
			wantErr: true,
			errMsg:  "gpu.vendor must be one of: nvidia, amd, intel",
		},
		{
			name: "valid CPU and memory quantities",
			spec: &aimodelv1alpha1.ModelRecipeSpec{
				ModelID: "meta-llama/Llama-2-7b-hf",
				Runtime: "vllm",
				Resources: aimodelv1alpha1.RecipeResources{
					GPU: aimodelv1alpha1.GPUResources{
						Count: 1,
					},
					CPU: aimodelv1alpha1.CPUResources{
						Requests: "2",
						Limits:   "4",
					},
					Memory: aimodelv1alpha1.MemoryResources{
						Requests: "8Gi",
						Limits:   "16Gi",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid CPU quantity",
			spec: &aimodelv1alpha1.ModelRecipeSpec{
				ModelID: "meta-llama/Llama-2-7b-hf",
				Runtime: "vllm",
				Resources: aimodelv1alpha1.RecipeResources{
					GPU: aimodelv1alpha1.GPUResources{
						Count: 1,
					},
					CPU: aimodelv1alpha1.CPUResources{
						Requests: "invalid",
					},
				},
			},
			wantErr: true,
			errMsg:  "cpu.requests is invalid",
		},
		{
			name: "triton without backend",
			spec: &aimodelv1alpha1.ModelRecipeSpec{
				ModelID: "meta-llama/Llama-2-7b-hf",
				Runtime: "triton",
				Resources: aimodelv1alpha1.RecipeResources{
					GPU: aimodelv1alpha1.GPUResources{
						Count: 1,
					},
				},
				RuntimeArgs: aimodelv1alpha1.RuntimeArgsSpec{
					Triton: &aimodelv1alpha1.TritonArgs{},
				},
			},
			wantErr: true,
			errMsg:  "triton.backend is required",
		},
		{
			name: "vllm with invalid dtype",
			spec: &aimodelv1alpha1.ModelRecipeSpec{
				ModelID: "meta-llama/Llama-2-7b-hf",
				Runtime: "vllm",
				Resources: aimodelv1alpha1.RecipeResources{
					GPU: aimodelv1alpha1.GPUResources{
						Count: 1,
					},
				},
				RuntimeArgs: aimodelv1alpha1.RuntimeArgsSpec{
					VLLM: &aimodelv1alpha1.VLLMArgs{
						DType: "invalid",
					},
				},
			},
			wantErr: true,
			errMsg:  "vllm.dtype must be one of",
		},
		{
			name: "vllm with invalid gpu memory utilization",
			spec: &aimodelv1alpha1.ModelRecipeSpec{
				ModelID: "meta-llama/Llama-2-7b-hf",
				Runtime: "vllm",
				Resources: aimodelv1alpha1.RecipeResources{
					GPU: aimodelv1alpha1.GPUResources{
						Count: 1,
					},
				},
				RuntimeArgs: aimodelv1alpha1.RuntimeArgsSpec{
					VLLM: &aimodelv1alpha1.VLLMArgs{
						GPUMemoryUtilization: "1.5",
					},
				},
			},
			wantErr: true,
			errMsg:  "vllm.gpuMemoryUtilization must be between 0.0 and 1.0",
		},
		{
			name: "tgi with invalid quantize",
			spec: &aimodelv1alpha1.ModelRecipeSpec{
				ModelID: "meta-llama/Llama-2-7b-hf",
				Runtime: "tgi",
				Resources: aimodelv1alpha1.RecipeResources{
					GPU: aimodelv1alpha1.GPUResources{
						Count: 1,
					},
				},
				RuntimeArgs: aimodelv1alpha1.RuntimeArgsSpec{
					TGI: &aimodelv1alpha1.TGIArgs{
						Quantize: "invalid",
					},
				},
			},
			wantErr: true,
			errMsg:  "tgi.quantize must be one of",
		},
		{
			name: "wrong runtime args for runtime",
			spec: &aimodelv1alpha1.ModelRecipeSpec{
				ModelID: "meta-llama/Llama-2-7b-hf",
				Runtime: "vllm",
				Resources: aimodelv1alpha1.RecipeResources{
					GPU: aimodelv1alpha1.GPUResources{
						Count: 1,
					},
				},
				RuntimeArgs: aimodelv1alpha1.RuntimeArgsSpec{
					Triton: &aimodelv1alpha1.TritonArgs{
						Backend: "python",
					},
				},
			},
			wantErr: true,
			errMsg:  "runtimeArgs.triton should not be set when runtime is vllm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.Validate(tt.spec)
			if tt.wantErr && result.Valid {
				t.Errorf("Validate() expected error but got valid result")
			}
			if !tt.wantErr && !result.Valid {
				t.Errorf("Validate() expected valid but got errors: %v", result.Errors)
			}
			if tt.wantErr && tt.errMsg != "" {
				found := false
				for _, err := range result.Errors {
					if containsSubstring(err, tt.errMsg) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Validate() expected error containing %q, got: %v", tt.errMsg, result.Errors)
				}
			}
		})
	}
}

func TestValidator_ValidateOverrides(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name      string
		overrides *aimodelv1alpha1.RecipeOverrides
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "nil overrides",
			overrides: nil,
			wantErr:   false,
		},
		{
			name: "valid runtime override",
			overrides: &aimodelv1alpha1.RecipeOverrides{
				Runtime: "triton",
			},
			wantErr: false,
		},
		{
			name: "invalid runtime override",
			overrides: &aimodelv1alpha1.RecipeOverrides{
				Runtime: "invalid",
			},
			wantErr: true,
			errMsg:  "runtime must be one of",
		},
		{
			name: "valid resource override",
			overrides: &aimodelv1alpha1.RecipeOverrides{
				Resources: &aimodelv1alpha1.RecipeResources{
					GPU: aimodelv1alpha1.GPUResources{
						Count: 2,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid GPU count override",
			overrides: &aimodelv1alpha1.RecipeOverrides{
				Resources: &aimodelv1alpha1.RecipeResources{
					GPU: aimodelv1alpha1.GPUResources{
						Count: 0,
					},
				},
			},
			wantErr: true,
			errMsg:  "gpu.count must be >= 1",
		},
		{
			name: "valid replica overrides",
			overrides: &aimodelv1alpha1.RecipeOverrides{
				Replicas: &aimodelv1alpha1.ReplicaOverrides{
					Min: int32Ptr(1),
					Max: int32Ptr(5),
				},
			},
			wantErr: false,
		},
		{
			name: "invalid replica overrides - min > max",
			overrides: &aimodelv1alpha1.RecipeOverrides{
				Replicas: &aimodelv1alpha1.ReplicaOverrides{
					Min: int32Ptr(5),
					Max: int32Ptr(1),
				},
			},
			wantErr: true,
			errMsg:  "replicas.min cannot be greater than replicas.max",
		},
		{
			name: "invalid replica overrides - negative min",
			overrides: &aimodelv1alpha1.RecipeOverrides{
				Replicas: &aimodelv1alpha1.ReplicaOverrides{
					Min: int32Ptr(-1),
				},
			},
			wantErr: true,
			errMsg:  "replicas.min cannot be negative",
		},
		{
			name: "valid vllm args override",
			overrides: &aimodelv1alpha1.RecipeOverrides{
				RuntimeArgs: &aimodelv1alpha1.RuntimeArgsSpec{
					VLLM: &aimodelv1alpha1.VLLMArgs{
						DType: "float16",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid vllm args override",
			overrides: &aimodelv1alpha1.RecipeOverrides{
				RuntimeArgs: &aimodelv1alpha1.RuntimeArgsSpec{
					VLLM: &aimodelv1alpha1.VLLMArgs{
						GPUMemoryUtilization: "2.0",
					},
				},
			},
			wantErr: true,
			errMsg:  "vllm.gpuMemoryUtilization must be between 0.0 and 1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.ValidateOverrides(tt.overrides)
			if tt.wantErr && result.Valid {
				t.Errorf("ValidateOverrides() expected error but got valid result")
			}
			if !tt.wantErr && !result.Valid {
				t.Errorf("ValidateOverrides() expected valid but got errors: %v", result.Errors)
			}
			if tt.wantErr && tt.errMsg != "" {
				found := false
				for _, err := range result.Errors {
					if containsSubstring(err, tt.errMsg) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("ValidateOverrides() expected error containing %q, got: %v", tt.errMsg, result.Errors)
				}
			}
		})
	}
}

func TestValidator_validateQuantity(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name     string
		quantity string
		wantErr  bool
	}{
		{name: "empty string", quantity: "", wantErr: false},
		{name: "integer", quantity: "4", wantErr: false},
		{name: "millicores", quantity: "1000m", wantErr: false},
		{name: "gigabytes", quantity: "8Gi", wantErr: false},
		{name: "megabytes", quantity: "500Mi", wantErr: false},
		{name: "kilobytes", quantity: "1000Ki", wantErr: false},
		{name: "decimal", quantity: "1.5", wantErr: false},
		{name: "decimal with suffix", quantity: "2.5Gi", wantErr: false},
		{name: "scientific notation", quantity: "1e9", wantErr: false},
		{name: "invalid format", quantity: "abc", wantErr: true},
		{name: "invalid suffix", quantity: "4xyz", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateQuantity(tt.quantity)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateQuantity() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidator_validateTritonArgs(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name    string
		args    *aimodelv1alpha1.TritonArgs
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid triton args",
			args: &aimodelv1alpha1.TritonArgs{
				Backend: "python",
			},
			wantErr: false,
		},
		{
			name:    "missing backend",
			args:    &aimodelv1alpha1.TritonArgs{},
			wantErr: true,
			errMsg:  "triton.backend is required",
		},
		{
			name: "invalid backend",
			args: &aimodelv1alpha1.TritonArgs{
				Backend: "invalid",
			},
			wantErr: true,
			errMsg:  "triton.backend must be one of",
		},
		{
			name: "valid instance group",
			args: &aimodelv1alpha1.TritonArgs{
				Backend: "python",
				InstanceGroup: []aimodelv1alpha1.TritonInstanceGroup{
					{Kind: "GPU", Count: 2},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid instance group - missing kind",
			args: &aimodelv1alpha1.TritonArgs{
				Backend: "python",
				InstanceGroup: []aimodelv1alpha1.TritonInstanceGroup{
					{Count: 2},
				},
			},
			wantErr: true,
			errMsg:  "triton.instanceGroup[0].kind is required",
		},
		{
			name: "invalid instance group - zero count",
			args: &aimodelv1alpha1.TritonArgs{
				Backend: "python",
				InstanceGroup: []aimodelv1alpha1.TritonInstanceGroup{
					{Kind: "GPU", Count: 0},
				},
			},
			wantErr: true,
			errMsg:  "triton.instanceGroup[0].count must be >= 1",
		},
		{
			name: "valid dynamic batching",
			args: &aimodelv1alpha1.TritonArgs{
				Backend: "python",
				DynamicBatching: &aimodelv1alpha1.TritonDynamicBatching{
					MaxBatchSize:              32,
					MaxQueueDelayMicroseconds: 100,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid dynamic batching - zero batch size",
			args: &aimodelv1alpha1.TritonArgs{
				Backend: "python",
				DynamicBatching: &aimodelv1alpha1.TritonDynamicBatching{
					MaxBatchSize: 0,
				},
			},
			wantErr: true,
			errMsg:  "triton.dynamicBatching.maxBatchSize must be >= 1",
		},
		{
			name: "valid input config",
			args: &aimodelv1alpha1.TritonArgs{
				Backend: "python",
				InputConfig: []aimodelv1alpha1.TritonTensorConfig{
					{Name: "input", DataType: "TYPE_FP32", Dims: []int32{1, 224, 224, 3}},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid input config - missing name",
			args: &aimodelv1alpha1.TritonArgs{
				Backend: "python",
				InputConfig: []aimodelv1alpha1.TritonTensorConfig{
					{DataType: "TYPE_FP32", Dims: []int32{1, 224}},
				},
			},
			wantErr: true,
			errMsg:  "triton.inputConfig[0].name is required",
		},
		{
			name: "invalid output config - missing dims",
			args: &aimodelv1alpha1.TritonArgs{
				Backend: "python",
				OutputConfig: []aimodelv1alpha1.TritonTensorConfig{
					{Name: "output", DataType: "TYPE_FP32"},
				},
			},
			wantErr: true,
			errMsg:  "triton.outputConfig[0].dims is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validator.validateTritonArgs(tt.args)
			hasErr := len(errors) > 0
			if hasErr != tt.wantErr {
				t.Errorf("validateTritonArgs() error = %v, wantErr %v", errors, tt.wantErr)
			}
			if tt.wantErr && tt.errMsg != "" {
				found := false
				for _, err := range errors {
					if containsSubstring(err, tt.errMsg) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("validateTritonArgs() expected error containing %q, got: %v", tt.errMsg, errors)
				}
			}
		})
	}
}

func TestValidationResult_ErrorString(t *testing.T) {
	tests := []struct {
		name   string
		result ValidationResult
		want   string
	}{
		{
			name: "valid result",
			result: ValidationResult{
				Valid:  true,
				Errors: []string{},
			},
			want: "",
		},
		{
			name: "single error",
			result: ValidationResult{
				Valid:  false,
				Errors: []string{"error1"},
			},
			want: "error1",
		},
		{
			name: "multiple errors",
			result: ValidationResult{
				Valid:  false,
				Errors: []string{"error1", "error2", "error3"},
			},
			want: "error1; error2; error3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.result.ErrorString()
			if got != tt.want {
				t.Errorf("ErrorString() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Helper functions

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func int32Ptr(i int32) *int32 {
	return &i
}
