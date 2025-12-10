package kserve

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// InferenceServiceGVK is the GroupVersionKind for KServe InferenceService
var InferenceServiceGVK = schema.GroupVersionKind{
	Group:   "serving.kserve.io",
	Version: "v1beta1",
	Kind:    "InferenceService",
}

// InferenceServiceBuilder provides a fluent API for building InferenceService resources
type InferenceServiceBuilder struct {
	name         string
	namespace    string
	modelID      string
	servedName   string
	runtime      string
	minReplicas  int32
	maxReplicas  int32
	resources    corev1.ResourceRequirements
	tolerations  []corev1.Toleration
	nodeSelector map[string]string
	runtimeArgs  []string
	runtimeEnv   []corev1.EnvVar
	ownerRef     *metav1.OwnerReference
	environment  string
}

// NewInferenceServiceBuilder creates a new InferenceServiceBuilder
func NewInferenceServiceBuilder(name, namespace string) *InferenceServiceBuilder {
	return &InferenceServiceBuilder{
		name:      name,
		namespace: namespace,
		// Default values
		minReplicas: 1,
		maxReplicas: 3,
		resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("4"),
				corev1.ResourceMemory: resource.MustParse("16Gi"),
				"nvidia.com/gpu":      resource.MustParse("1"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("8"),
				corev1.ResourceMemory: resource.MustParse("32Gi"),
				"nvidia.com/gpu":      resource.MustParse("1"),
			},
		},
		tolerations: []corev1.Toleration{
			{
				Key:      "gpu-workload",
				Operator: corev1.TolerationOpEqual,
				Value:    "true",
				Effect:   corev1.TaintEffectNoSchedule,
			},
			{
				Key:      "nvidia.com/gpu",
				Operator: corev1.TolerationOpExists,
				Effect:   corev1.TaintEffectNoSchedule,
			},
		},
		runtimeArgs: []string{
			"--dtype=float16",
			"--max-model-len=4096",
			"--gpu-memory-utilization=0.9",
			"--trust-remote-code",
		},
		environment: "development",
	}
}

// WithModelID sets the model ID (HuggingFace path)
func (b *InferenceServiceBuilder) WithModelID(id string) *InferenceServiceBuilder {
	b.modelID = id
	return b
}

// WithServedName sets the served model name for the inference endpoint
func (b *InferenceServiceBuilder) WithServedName(name string) *InferenceServiceBuilder {
	b.servedName = name
	return b
}

// WithRuntime sets the container runtime image
func (b *InferenceServiceBuilder) WithRuntime(runtime string) *InferenceServiceBuilder {
	b.runtime = runtime
	return b
}

// WithScaling sets the min and max replicas for autoscaling
func (b *InferenceServiceBuilder) WithScaling(min, max int32) *InferenceServiceBuilder {
	b.minReplicas = min
	b.maxReplicas = max
	return b
}

// WithResources sets the resource requirements
func (b *InferenceServiceBuilder) WithResources(r corev1.ResourceRequirements) *InferenceServiceBuilder {
	b.resources = r
	return b
}

// WithTolerations sets the pod tolerations
func (b *InferenceServiceBuilder) WithTolerations(t []corev1.Toleration) *InferenceServiceBuilder {
	b.tolerations = t
	return b
}

// WithNodeSelector sets the node selector
func (b *InferenceServiceBuilder) WithNodeSelector(ns map[string]string) *InferenceServiceBuilder {
	b.nodeSelector = ns
	return b
}

// WithRuntimeArgs sets the container runtime arguments
func (b *InferenceServiceBuilder) WithRuntimeArgs(args []string) *InferenceServiceBuilder {
	b.runtimeArgs = args
	return b
}

// WithRuntimeEnv sets the container environment variables
func (b *InferenceServiceBuilder) WithRuntimeEnv(env []corev1.EnvVar) *InferenceServiceBuilder {
	b.runtimeEnv = env
	return b
}

// WithOwnerReference sets the owner reference for garbage collection
func (b *InferenceServiceBuilder) WithOwnerReference(ref *metav1.OwnerReference) *InferenceServiceBuilder {
	b.ownerRef = ref
	return b
}

// WithEnvironment sets the environment label (development, staging, production)
func (b *InferenceServiceBuilder) WithEnvironment(env string) *InferenceServiceBuilder {
	b.environment = env
	return b
}

// Build constructs the unstructured InferenceService resource
func (b *InferenceServiceBuilder) Build() (*unstructured.Unstructured, error) {
	// Validate required fields
	if b.modelID == "" {
		return nil, fmt.Errorf("modelID is required")
	}
	if b.servedName == "" {
		return nil, fmt.Errorf("servedName is required")
	}
	if b.runtime == "" {
		return nil, fmt.Errorf("runtime image is required")
	}

	// Build labels
	labels := map[string]interface{}{
		"app":         "vllm-inference",
		"model":       b.servedName,
		"environment": b.environment,
		"managed-by":  "ai-model-operator",
	}

	// Build annotations
	annotations := map[string]interface{}{
		"serving.kserve.io/deploymentMode":           "Serverless",
		"serving.knative.dev/progress-deadline":      "360s",
		"autoscaling.knative.dev/class":              "kpa.autoscaling.knative.dev",
		"autoscaling.knative.dev/metric":             "concurrency",
		"autoscaling.knative.dev/target":             "1",
		"autoscaling.knative.dev/scaleDownDelay":     "2m",
		"autoscaling.knative.dev/window":             "30s",
		"autoscaling.knative.dev/panicThreshold":     "150",
		"autoscaling.knative.dev/panicWindowPercentage": "10",
		"autoscaling.knative.dev/targetUtilizationPercentage": "70",
	}

	// Build runtime arguments
	args := make([]interface{}, 0, len(b.runtimeArgs)+2)
	args = append(args, fmt.Sprintf("--model=%s", b.modelID))
	for _, arg := range b.runtimeArgs {
		args = append(args, arg)
	}
	args = append(args, fmt.Sprintf("--served-model-name=%s", b.servedName))

	// Build environment variables
	env := make([]interface{}, 0, len(b.runtimeEnv)+1)
	// Add default HF_HOME
	env = append(env, map[string]interface{}{
		"name":  "HF_HOME",
		"value": "/tmp/hf_home",
	})
	// Add custom env vars
	for _, e := range b.runtimeEnv {
		envVar := map[string]interface{}{
			"name": e.Name,
		}
		if e.Value != "" {
			envVar["value"] = e.Value
		}
		if e.ValueFrom != nil {
			envVar["valueFrom"] = convertValueFrom(e.ValueFrom)
		}
		env = append(env, envVar)
	}

	// Build resources
	resources := map[string]interface{}{
		"requests": convertResourceList(b.resources.Requests),
		"limits":   convertResourceList(b.resources.Limits),
	}

	// Build tolerations
	tolerations := make([]interface{}, 0, len(b.tolerations))
	for _, t := range b.tolerations {
		toleration := map[string]interface{}{
			"key":      t.Key,
			"operator": string(t.Operator),
			"effect":   string(t.Effect),
		}
		if t.Value != "" {
			toleration["value"] = t.Value
		}
		tolerations = append(tolerations, toleration)
	}

	// Build container
	container := map[string]interface{}{
		"name":  "kserve-container",
		"image": b.runtime,
		"args":  args,
		"env":   env,
		"resources": resources,
		"ports": []interface{}{
			map[string]interface{}{
				"containerPort": int64(8000),
				"name":          "http1",
				"protocol":      "TCP",
			},
		},
		"startupProbe": map[string]interface{}{
			"httpGet": map[string]interface{}{
				"path": "/health",
				"port": int64(8000),
			},
			"initialDelaySeconds": int64(30),
			"periodSeconds":       int64(10),
			"failureThreshold":    int64(36),
			"timeoutSeconds":      int64(5),
		},
		"readinessProbe": map[string]interface{}{
			"httpGet": map[string]interface{}{
				"path": "/health",
				"port": int64(8000),
			},
			"periodSeconds":    int64(10),
			"failureThreshold": int64(3),
			"timeoutSeconds":   int64(5),
		},
		"livenessProbe": map[string]interface{}{
			"httpGet": map[string]interface{}{
				"path": "/health",
				"port": int64(8000),
			},
			"periodSeconds":    int64(30),
			"failureThreshold": int64(3),
			"timeoutSeconds":   int64(5),
		},
	}

	// Build predictor spec
	predictor := map[string]interface{}{
		"minReplicas": int64(b.minReplicas),
		"maxReplicas": int64(b.maxReplicas),
		"tolerations": tolerations,
		"containers":  []interface{}{container},
	}

	// Add node selector if provided
	if len(b.nodeSelector) > 0 {
		nodeSelector := make(map[string]interface{})
		for k, v := range b.nodeSelector {
			nodeSelector[k] = v
		}
		predictor["nodeSelector"] = nodeSelector
	}

	// Build the unstructured object
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": InferenceServiceGVK.GroupVersion().String(),
			"kind":       InferenceServiceGVK.Kind,
			"metadata": map[string]interface{}{
				"name":        b.name,
				"namespace":   b.namespace,
				"labels":      labels,
				"annotations": annotations,
			},
			"spec": map[string]interface{}{
				"predictor": predictor,
			},
		},
	}

	// Add owner reference if provided
	if b.ownerRef != nil {
		obj.SetOwnerReferences([]metav1.OwnerReference{*b.ownerRef})
	}

	return obj, nil
}

// convertResourceList converts corev1.ResourceList to map[string]interface{}
func convertResourceList(rl corev1.ResourceList) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range rl {
		result[string(k)] = v.String()
	}
	return result
}

// convertValueFrom converts corev1.EnvVarSource to map[string]interface{}
func convertValueFrom(source *corev1.EnvVarSource) map[string]interface{} {
	result := make(map[string]interface{})

	if source.SecretKeyRef != nil {
		result["secretKeyRef"] = map[string]interface{}{
			"name": source.SecretKeyRef.Name,
			"key":  source.SecretKeyRef.Key,
		}
	}
	if source.ConfigMapKeyRef != nil {
		result["configMapKeyRef"] = map[string]interface{}{
			"name": source.ConfigMapKeyRef.Name,
			"key":  source.ConfigMapKeyRef.Key,
		}
	}
	if source.FieldRef != nil {
		result["fieldRef"] = map[string]interface{}{
			"fieldPath": source.FieldRef.FieldPath,
		}
	}

	return result
}
