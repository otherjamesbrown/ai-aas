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

package webhook

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	aimodelv1alpha1 "github.com/ai-aas/ai-model-operator/api/v1alpha1"
)

// AIModelValidator validates AIModel resources
type AIModelValidator struct {
	Client client.Client
}

// ValidateCreate validates the AIModel on creation
func (v *AIModelValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	aimodel, ok := obj.(*aimodelv1alpha1.AIModel)
	if !ok {
		return nil, fmt.Errorf("expected AIModel but got %T", obj)
	}
	return v.validate(ctx, aimodel)
}

// ValidateUpdate validates the AIModel on update
func (v *AIModelValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	newAIModel, ok := newObj.(*aimodelv1alpha1.AIModel)
	if !ok {
		return nil, fmt.Errorf("expected AIModel but got %T", newObj)
	}
	return v.validate(ctx, newAIModel)
}

// ValidateDelete validates the AIModel on deletion
func (v *AIModelValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// No validation needed for deletion
	return nil, nil
}

// validate performs validation checks on the AIModel
func (v *AIModelValidator) validate(ctx context.Context, aimodel *aimodelv1alpha1.AIModel) ([]string, error) {
	var warnings []string

	// Get GPU nodes and their taints
	nodes := &corev1.NodeList{}
	if err := v.Client.List(ctx, nodes, client.MatchingLabels{"nvidia.com/gpu.present": "true"}); err != nil {
		// If we can't list nodes, we can't validate tolerations
		// Log this but don't fail the admission - cluster might not have GPU nodes yet
		return warnings, nil
	}

	if len(nodes.Items) == 0 {
		// No GPU nodes in cluster - skip toleration validation
		return warnings, nil
	}

	// Check if AIModel tolerations cover all GPU node taints
	requiredTolerations := v.getRequiredTolerations(nodes)
	missingTolerations := v.findMissingTolerations(aimodel.Spec.Tolerations, requiredTolerations)

	if len(missingTolerations) > 0 {
		var taintStrings []string
		for _, taint := range missingTolerations {
			taintStrings = append(taintStrings, formatTaint(taint))
		}
		return nil, fmt.Errorf("AIModel is missing required tolerations for GPU nodes. "+
			"The following taints are not tolerated: %s. "+
			"Add these tolerations to spec.tolerations to allow scheduling on GPU nodes",
			strings.Join(taintStrings, ", "))
	}

	// Warn if CPU request exceeds smallest GPU node
	if len(aimodel.Spec.Resources.Requests) > 0 {
		smallestCPU := v.getSmallestGPUNodeCPU(nodes)
		if requestedCPU, ok := aimodel.Spec.Resources.Requests[corev1.ResourceCPU]; ok && !smallestCPU.IsZero() {
			if requestedCPU.Cmp(smallestCPU) > 0 {
				warnings = append(warnings, fmt.Sprintf(
					"CPU request %s exceeds smallest GPU node allocatable capacity %s - "+
						"pod may not be schedulable on all GPU nodes",
					requestedCPU.String(), smallestCPU.String()))
			}
		}
	}

	return warnings, nil
}

// getRequiredTolerations extracts all taints from GPU nodes that need tolerations
func (v *AIModelValidator) getRequiredTolerations(nodes *corev1.NodeList) []corev1.Taint {
	taintMap := make(map[string]corev1.Taint)

	for _, node := range nodes.Items {
		for _, taint := range node.Spec.Taints {
			// Skip taints with effect PreferNoSchedule - they're advisory only
			if taint.Effect == corev1.TaintEffectPreferNoSchedule {
				continue
			}
			// Use key+effect as unique identifier
			key := taint.Key + string(taint.Effect)
			taintMap[key] = taint
		}
	}

	taints := make([]corev1.Taint, 0, len(taintMap))
	for _, taint := range taintMap {
		taints = append(taints, taint)
	}
	return taints
}

// findMissingTolerations identifies taints that are not covered by the provided tolerations
func (v *AIModelValidator) findMissingTolerations(tolerations []corev1.Toleration, requiredTaints []corev1.Taint) []corev1.Taint {
	var missing []corev1.Taint

	for _, taint := range requiredTaints {
		if !v.tolerationCovers(tolerations, taint) {
			missing = append(missing, taint)
		}
	}

	return missing
}

// tolerationCovers checks if any toleration in the list covers the given taint
func (v *AIModelValidator) tolerationCovers(tolerations []corev1.Toleration, taint corev1.Taint) bool {
	for _, toleration := range tolerations {
		if v.tolerationMatchesTaint(toleration, taint) {
			return true
		}
	}
	return false
}

// tolerationMatchesTaint checks if a toleration matches a taint
// Based on Kubernetes scheduler logic
func (v *AIModelValidator) tolerationMatchesTaint(toleration corev1.Toleration, taint corev1.Taint) bool {
	// Operator=Exists with empty key matches all taints
	if toleration.Operator == corev1.TolerationOpExists && toleration.Key == "" {
		return true
	}

	// Key must match
	if toleration.Key != taint.Key {
		return false
	}

	// Effect must match (empty effect in toleration matches all effects)
	if toleration.Effect != "" && toleration.Effect != taint.Effect {
		return false
	}

	// For Exists operator, just key and effect match is enough
	if toleration.Operator == corev1.TolerationOpExists {
		return true
	}

	// For Equal operator (default), value must also match
	return toleration.Value == taint.Value
}

// getSmallestGPUNodeCPU returns the allocatable CPU of the smallest GPU node
func (v *AIModelValidator) getSmallestGPUNodeCPU(nodes *corev1.NodeList) resource.Quantity {
	if len(nodes.Items) == 0 {
		return resource.Quantity{}
	}

	smallest := nodes.Items[0].Status.Allocatable[corev1.ResourceCPU]
	for i := 1; i < len(nodes.Items); i++ {
		cpu := nodes.Items[i].Status.Allocatable[corev1.ResourceCPU]
		if cpu.Cmp(smallest) < 0 {
			smallest = cpu
		}
	}
	return smallest
}

// formatTaint formats a taint for display in error messages
func formatTaint(taint corev1.Taint) string {
	if taint.Value == "" {
		return fmt.Sprintf("%s:%s", taint.Key, taint.Effect)
	}
	return fmt.Sprintf("%s=%s:%s", taint.Key, taint.Value, taint.Effect)
}
