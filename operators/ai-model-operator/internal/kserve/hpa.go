package kserve

import (
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	aiv1alpha1 "github.com/ai-aas/ai-model-operator/api/v1alpha1"
)

// HPABuilder builds HorizontalPodAutoscaler resources for RawDeployment mode.
type HPABuilder struct {
	name        string
	namespace   string
	autoscaling *aiv1alpha1.AutoscalingSpec
}

// NewHPABuilder creates a new HPABuilder.
func NewHPABuilder() *HPABuilder {
	return &HPABuilder{}
}

// WithName sets the name for the HPA.
func (b *HPABuilder) WithName(name string) *HPABuilder {
	b.name = name
	return b
}

// WithNamespace sets the namespace for the HPA.
func (b *HPABuilder) WithNamespace(namespace string) *HPABuilder {
	b.namespace = namespace
	return b
}

// WithAutoscaling sets the autoscaling configuration.
func (b *HPABuilder) WithAutoscaling(spec *aiv1alpha1.AutoscalingSpec) *HPABuilder {
	b.autoscaling = spec
	return b
}

// Build creates the HPA resource. Returns nil if autoscaling is not enabled.
func (b *HPABuilder) Build() *autoscalingv2.HorizontalPodAutoscaler {
	if b.autoscaling == nil || !b.autoscaling.Enabled {
		return nil
	}

	minReplicas := b.autoscaling.MinReplicas
	if minReplicas < 1 {
		minReplicas = 1
	}

	maxReplicas := b.autoscaling.MaxReplicas
	if maxReplicas < minReplicas {
		maxReplicas = minReplicas
	}

	targetCPU := b.autoscaling.TargetCPUUtilization
	if targetCPU <= 0 {
		targetCPU = 70
	}

	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      b.name + "-hpa",
			Namespace: b.namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":      b.name,
				"app.kubernetes.io/component": "hpa",
				"app.kubernetes.io/part-of":   "ai-model-operator",
			},
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       b.name + "-predictor",
			},
			MinReplicas: ptr.To(minReplicas),
			MaxReplicas: maxReplicas,
			Metrics: []autoscalingv2.MetricSpec{
				{
					Type: autoscalingv2.ResourceMetricSourceType,
					Resource: &autoscalingv2.ResourceMetricSource{
						Name: "cpu",
						Target: autoscalingv2.MetricTarget{
							Type:               autoscalingv2.UtilizationMetricType,
							AverageUtilization: ptr.To(targetCPU),
						},
					},
				},
			},
		},
	}

	// Add scale-down stabilization if configured
	if b.autoscaling.ScaleDownStabilization > 0 {
		hpa.Spec.Behavior = &autoscalingv2.HorizontalPodAutoscalerBehavior{
			ScaleDown: &autoscalingv2.HPAScalingRules{
				StabilizationWindowSeconds: ptr.To(b.autoscaling.ScaleDownStabilization),
			},
		}
	}

	return hpa
}
