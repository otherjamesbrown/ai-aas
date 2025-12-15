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
	"fmt"
	"regexp"
	"strconv"
	"strings"

	aimodelv1alpha1 "github.com/ai-aas/ai-model-operator/api/v1alpha1"
)

// Validator validates ModelRecipe configurations
type Validator struct{}

// NewValidator creates a new recipe validator
func NewValidator() *Validator {
	return &Validator{}
}

// ValidationResult contains validation results
type ValidationResult struct {
	Valid  bool
	Errors []string
}

// Validate validates a ModelRecipeSpec and returns any errors
func (v *Validator) Validate(spec *aimodelv1alpha1.ModelRecipeSpec) ValidationResult {
	if spec == nil {
		return ValidationResult{
			Valid:  false,
			Errors: []string{"spec is nil"},
		}
	}

	var errors []string

	// Validate required fields
	if spec.ModelID == "" {
		errors = append(errors, "modelID is required")
	}

	// Validate runtime
	if err := v.validateRuntime(spec.Runtime); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate resources
	if errs := v.validateResources(&spec.Resources); len(errs) > 0 {
		errors = append(errors, errs...)
	}

	// Validate runtime-specific args
	if errs := v.validateRuntimeArgs(spec.Runtime, &spec.RuntimeArgs); len(errs) > 0 {
		errors = append(errors, errs...)
	}

	return ValidationResult{
		Valid:  len(errors) == 0,
		Errors: errors,
	}
}

// ValidateOverrides validates RecipeOverrides (less strict - allows partial)
func (v *Validator) ValidateOverrides(overrides *aimodelv1alpha1.RecipeOverrides) ValidationResult {
	if overrides == nil {
		return ValidationResult{Valid: true, Errors: []string{}}
	}

	var errors []string

	// Validate runtime if specified
	if overrides.Runtime != "" {
		if err := v.validateRuntime(overrides.Runtime); err != nil {
			errors = append(errors, err.Error())
		}
	}

	// Validate resources if specified
	if overrides.Resources != nil {
		if errs := v.validateResources(overrides.Resources); len(errs) > 0 {
			errors = append(errors, errs...)
		}
	}

	// Validate runtime args if specified
	if overrides.RuntimeArgs != nil {
		// For overrides, we validate args structure but don't enforce runtime matching
		// since the final runtime may come from the base recipe
		if overrides.RuntimeArgs.VLLM != nil {
			if errs := v.validateVLLMArgs(overrides.RuntimeArgs.VLLM); len(errs) > 0 {
				errors = append(errors, errs...)
			}
		}
		if overrides.RuntimeArgs.Triton != nil {
			if errs := v.validateTritonArgs(overrides.RuntimeArgs.Triton); len(errs) > 0 {
				errors = append(errors, errs...)
			}
		}
		if overrides.RuntimeArgs.TGI != nil {
			if errs := v.validateTGIArgs(overrides.RuntimeArgs.TGI); len(errs) > 0 {
				errors = append(errors, errs...)
			}
		}
	}

	// Validate replica overrides if specified
	if overrides.Replicas != nil {
		if errs := v.validateReplicaOverrides(overrides.Replicas); len(errs) > 0 {
			errors = append(errors, errs...)
		}
	}

	return ValidationResult{
		Valid:  len(errors) == 0,
		Errors: errors,
	}
}

// validateRuntime validates the runtime field
func (v *Validator) validateRuntime(runtime string) error {
	if runtime == "" {
		return fmt.Errorf("runtime is required")
	}

	validRuntimes := map[string]bool{
		"vllm":   true,
		"triton": true,
		"tgi":    true,
	}

	if !validRuntimes[runtime] {
		return fmt.Errorf("runtime must be one of: vllm, triton, tgi (got: %s)", runtime)
	}

	return nil
}

// validateResources validates resource requirements
func (v *Validator) validateResources(resources *aimodelv1alpha1.RecipeResources) []string {
	if resources == nil {
		return []string{"resources are required"}
	}

	var errors []string

	// Validate GPU
	if errs := v.validateGPU(&resources.GPU); len(errs) > 0 {
		errors = append(errors, errs...)
	}

	// Validate CPU if specified
	if errs := v.validateCPU(&resources.CPU); len(errs) > 0 {
		errors = append(errors, errs...)
	}

	// Validate Memory if specified
	if errs := v.validateMemory(&resources.Memory); len(errs) > 0 {
		errors = append(errors, errs...)
	}

	return errors
}

// validateGPU validates GPU resources
func (v *Validator) validateGPU(gpu *aimodelv1alpha1.GPUResources) []string {
	var errors []string

	// Validate GPU count
	if gpu.Count < 1 {
		errors = append(errors, "gpu.count must be >= 1")
	}

	// Validate GPU vendor if specified
	if gpu.Vendor != "" {
		validVendors := map[string]bool{
			"nvidia": true,
			"amd":    true,
			"intel":  true,
		}
		if !validVendors[gpu.Vendor] {
			errors = append(errors, fmt.Sprintf("gpu.vendor must be one of: nvidia, amd, intel (got: %s)", gpu.Vendor))
		}
	}

	// Validate MinMemoryGB if specified
	if gpu.MinMemoryGB < 0 {
		errors = append(errors, "gpu.minMemoryGB cannot be negative")
	}

	return errors
}

// validateCPU validates CPU resources
func (v *Validator) validateCPU(cpu *aimodelv1alpha1.CPUResources) []string {
	var errors []string

	if cpu.Requests != "" {
		if err := v.validateQuantity(cpu.Requests); err != nil {
			errors = append(errors, fmt.Sprintf("cpu.requests is invalid: %v", err))
		}
	}

	if cpu.Limits != "" {
		if err := v.validateQuantity(cpu.Limits); err != nil {
			errors = append(errors, fmt.Sprintf("cpu.limits is invalid: %v", err))
		}
	}

	return errors
}

// validateMemory validates memory resources
func (v *Validator) validateMemory(memory *aimodelv1alpha1.MemoryResources) []string {
	var errors []string

	if memory.Requests != "" {
		if err := v.validateQuantity(memory.Requests); err != nil {
			errors = append(errors, fmt.Sprintf("memory.requests is invalid: %v", err))
		}
	}

	if memory.Limits != "" {
		if err := v.validateQuantity(memory.Limits); err != nil {
			errors = append(errors, fmt.Sprintf("memory.limits is invalid: %v", err))
		}
	}

	return errors
}

// validateQuantity validates a Kubernetes quantity string
// Format: <number><suffix> where suffix is optional (e.g., "1000m", "2Gi", "500Mi", "4")
func (v *Validator) validateQuantity(quantity string) error {
	if quantity == "" {
		return nil
	}

	// Kubernetes quantity regex pattern
	// Supports: integers, decimals, scientific notation, and suffixes (m, K, M, G, T, P, E, Ki, Mi, Gi, Ti, Pi, Ei)
	pattern := `^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$`
	matched, err := regexp.MatchString(pattern, quantity)
	if err != nil {
		return err
	}
	if !matched {
		return fmt.Errorf("invalid quantity format: %s", quantity)
	}

	return nil
}

// validateRuntimeArgs validates runtime-specific arguments
func (v *Validator) validateRuntimeArgs(runtime string, args *aimodelv1alpha1.RuntimeArgsSpec) []string {
	if args == nil {
		return []string{}
	}

	var errors []string

	// Check that args match the specified runtime
	switch runtime {
	case "vllm":
		if args.VLLM != nil {
			if errs := v.validateVLLMArgs(args.VLLM); len(errs) > 0 {
				errors = append(errors, errs...)
			}
		}
		// Warn if other runtime args are set
		if args.Triton != nil {
			errors = append(errors, "runtimeArgs.triton should not be set when runtime is vllm")
		}
		if args.TGI != nil {
			errors = append(errors, "runtimeArgs.tgi should not be set when runtime is vllm")
		}
	case "triton":
		if args.Triton != nil {
			if errs := v.validateTritonArgs(args.Triton); len(errs) > 0 {
				errors = append(errors, errs...)
			}
		}
		// Warn if other runtime args are set
		if args.VLLM != nil {
			errors = append(errors, "runtimeArgs.vllm should not be set when runtime is triton")
		}
		if args.TGI != nil {
			errors = append(errors, "runtimeArgs.tgi should not be set when runtime is triton")
		}
	case "tgi":
		if args.TGI != nil {
			if errs := v.validateTGIArgs(args.TGI); len(errs) > 0 {
				errors = append(errors, errs...)
			}
		}
		// Warn if other runtime args are set
		if args.VLLM != nil {
			errors = append(errors, "runtimeArgs.vllm should not be set when runtime is tgi")
		}
		if args.Triton != nil {
			errors = append(errors, "runtimeArgs.triton should not be set when runtime is tgi")
		}
	}

	return errors
}

// validateVLLMArgs validates vLLM-specific arguments
func (v *Validator) validateVLLMArgs(args *aimodelv1alpha1.VLLMArgs) []string {
	var errors []string

	// Validate DType if specified
	if args.DType != "" {
		validDTypes := map[string]bool{
			"auto":    true,
			"float16": true,
			"bfloat16": true,
			"float32": true,
		}
		if !validDTypes[args.DType] {
			errors = append(errors, fmt.Sprintf("vllm.dtype must be one of: auto, float16, bfloat16, float32 (got: %s)", args.DType))
		}
	}

	// Validate MaxModelLen if specified
	if args.MaxModelLen < 0 {
		errors = append(errors, "vllm.maxModelLen cannot be negative")
	}

	// Validate GPUMemoryUtilization if specified
	if args.GPUMemoryUtilization != "" {
		utilization, err := strconv.ParseFloat(args.GPUMemoryUtilization, 64)
		if err != nil {
			errors = append(errors, fmt.Sprintf("vllm.gpuMemoryUtilization must be a valid float: %v", err))
		} else if utilization < 0.0 || utilization > 1.0 {
			errors = append(errors, "vllm.gpuMemoryUtilization must be between 0.0 and 1.0")
		}
	}

	return errors
}

// validateTritonArgs validates Triton-specific arguments
func (v *Validator) validateTritonArgs(args *aimodelv1alpha1.TritonArgs) []string {
	var errors []string

	// Backend is required for Triton
	if args.Backend == "" {
		errors = append(errors, "triton.backend is required")
	} else {
		validBackends := map[string]bool{
			"python":      true,
			"tensorrt":    true,
			"onnxruntime": true,
			"pytorch":     true,
		}
		if !validBackends[args.Backend] {
			errors = append(errors, fmt.Sprintf("triton.backend must be one of: python, tensorrt, onnxruntime, pytorch (got: %s)", args.Backend))
		}
	}

	// Validate InstanceGroup if specified
	for i, ig := range args.InstanceGroup {
		if ig.Kind == "" {
			errors = append(errors, fmt.Sprintf("triton.instanceGroup[%d].kind is required", i))
		}
		if ig.Count < 1 {
			errors = append(errors, fmt.Sprintf("triton.instanceGroup[%d].count must be >= 1", i))
		}
	}

	// Validate DynamicBatching if specified
	if args.DynamicBatching != nil {
		if args.DynamicBatching.MaxBatchSize < 1 {
			errors = append(errors, "triton.dynamicBatching.maxBatchSize must be >= 1")
		}
		if args.DynamicBatching.MaxQueueDelayMicroseconds < 0 {
			errors = append(errors, "triton.dynamicBatching.maxQueueDelayMicroseconds cannot be negative")
		}
	}

	// Validate TensorConfig
	for i, tc := range args.InputConfig {
		if tc.Name == "" {
			errors = append(errors, fmt.Sprintf("triton.inputConfig[%d].name is required", i))
		}
		if tc.DataType == "" {
			errors = append(errors, fmt.Sprintf("triton.inputConfig[%d].dataType is required", i))
		}
		if len(tc.Dims) == 0 {
			errors = append(errors, fmt.Sprintf("triton.inputConfig[%d].dims is required", i))
		}
	}

	for i, tc := range args.OutputConfig {
		if tc.Name == "" {
			errors = append(errors, fmt.Sprintf("triton.outputConfig[%d].name is required", i))
		}
		if tc.DataType == "" {
			errors = append(errors, fmt.Sprintf("triton.outputConfig[%d].dataType is required", i))
		}
		if len(tc.Dims) == 0 {
			errors = append(errors, fmt.Sprintf("triton.outputConfig[%d].dims is required", i))
		}
	}

	return errors
}

// validateTGIArgs validates TGI-specific arguments
func (v *Validator) validateTGIArgs(args *aimodelv1alpha1.TGIArgs) []string {
	var errors []string

	// Validate Quantize if specified
	if args.Quantize != "" {
		validQuantize := map[string]bool{
			"bitsandbytes": true,
			"gptq":         true,
			"awq":          true,
		}
		if !validQuantize[args.Quantize] {
			errors = append(errors, fmt.Sprintf("tgi.quantize must be one of: bitsandbytes, gptq, awq (got: %s)", args.Quantize))
		}
	}

	// Validate lengths if specified
	if args.MaxInputLength < 0 {
		errors = append(errors, "tgi.maxInputLength cannot be negative")
	}
	if args.MaxTotalTokens < 0 {
		errors = append(errors, "tgi.maxTotalTokens cannot be negative")
	}
	if args.MaxBatchPrefillTokens < 0 {
		errors = append(errors, "tgi.maxBatchPrefillTokens cannot be negative")
	}
	if args.NumShard < 0 {
		errors = append(errors, "tgi.numShard cannot be negative")
	}

	return errors
}

// validateReplicaOverrides validates replica overrides
func (v *Validator) validateReplicaOverrides(replicas *aimodelv1alpha1.ReplicaOverrides) []string {
	var errors []string

	if replicas.Min != nil && *replicas.Min < 0 {
		errors = append(errors, "replicas.min cannot be negative")
	}

	if replicas.Max != nil && *replicas.Max < 0 {
		errors = append(errors, "replicas.max cannot be negative")
	}

	if replicas.Min != nil && replicas.Max != nil && *replicas.Min > *replicas.Max {
		errors = append(errors, "replicas.min cannot be greater than replicas.max")
	}

	return errors
}

// ErrorString returns a formatted error string from validation result
func (r ValidationResult) ErrorString() string {
	if r.Valid {
		return ""
	}
	return strings.Join(r.Errors, "; ")
}
