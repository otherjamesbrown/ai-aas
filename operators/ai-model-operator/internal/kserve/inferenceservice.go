package kserve

import (
	"fmt"
	"strings"

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
	storageUri   string
	modelFormat  string
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
	// Container-based deployment fields (for trust_remote_code models)
	containerImage string
	modelID        string // HuggingFace model ID for direct loading
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
		runtimeArgs: []string{}, // Runtime args are now set per-model in the controller
		environment: "development",
	}
}

// sanitizeLabelValue converts a string to be valid as a Kubernetes label value.
// Kubernetes labels must be alphanumeric with '-', '_', or '.', starting and ending
// with an alphanumeric character. This function replaces "/" with "-" to handle
// HuggingFace model IDs like "unsloth/gpt-oss-20b".
func sanitizeLabelValue(value string) string {
	// Replace "/" with "-" to handle HF model IDs
	return strings.ReplaceAll(value, "/", "-")
}

// WithStorageUri sets the S3 storage URI for the model
func (b *InferenceServiceBuilder) WithStorageUri(uri string) *InferenceServiceBuilder {
	b.storageUri = uri
	return b
}

// WithModelFormat sets the model format name (e.g., "vllm", "vllm-gpt-oss")
func (b *InferenceServiceBuilder) WithModelFormat(format string) *InferenceServiceBuilder {
	b.modelFormat = format
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

// WithContainerImage sets the container image for container-based deployment
func (b *InferenceServiceBuilder) WithContainerImage(image string) *InferenceServiceBuilder {
	b.containerImage = image
	return b
}

// WithModelID sets the HuggingFace model ID for direct loading
func (b *InferenceServiceBuilder) WithModelID(modelID string) *InferenceServiceBuilder {
	b.modelID = modelID
	return b
}

// Build constructs the unstructured InferenceService resource using KServe native model serving
func (b *InferenceServiceBuilder) Build() (*unstructured.Unstructured, error) {
	// Validate required fields
	if b.storageUri == "" {
		return nil, fmt.Errorf("storageUri is required")
	}
	if b.runtime == "" {
		return nil, fmt.Errorf("runtime is required")
	}

	// Default modelFormat to "vllm" if not specified
	modelFormat := b.modelFormat
	if modelFormat == "" {
		modelFormat = "vllm"
	}

	// Build labels
	labels := map[string]interface{}{
		"app":         "vllm-inference",
		"model":       sanitizeLabelValue(b.servedName),
		"environment": b.environment,
		"managed-by":  "ai-model-operator",
	}

	// Build annotations
	// Note: Revision garbage collection is configured cluster-wide via the
	// config-gc ConfigMap in knative-serving namespace. For GPU workloads,
	// we use aggressive GC settings (min=1, max=2 non-active revisions) to
	// prevent old revisions from holding GPU resources. See:
	// infra/k8s/knative-serving/config-gc.yaml
	annotations := map[string]interface{}{
		"serving.kserve.io/deploymentMode": "Serverless",
	}

	// Build environment variables for the model
	env := make([]interface{}, 0, len(b.runtimeEnv))
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

	// Build model spec using KServe native model serving
	// This leverages KServe's storage initializer to download from S3
	model := map[string]interface{}{
		"storageUri": b.storageUri,
		"runtime":    b.runtime,
		"modelFormat": map[string]interface{}{
			"name": modelFormat,
		},
		"resources": resources,
	}

	// Add runtime arguments if specified
	if len(b.runtimeArgs) > 0 {
		// Convert []string to []interface{} for unstructured
		args := make([]interface{}, len(b.runtimeArgs))
		for i, arg := range b.runtimeArgs {
			args[i] = arg
		}
		model["args"] = args
	}

	// Add environment variables if specified
	if len(env) > 0 {
		model["env"] = env
	}

	// Build predictor spec using native model serving
	predictor := map[string]interface{}{
		"minReplicas": int64(b.minReplicas),
		"maxReplicas": int64(b.maxReplicas),
		"model":       model,
	}

	// Add tolerations if specified
	if len(tolerations) > 0 {
		predictor["tolerations"] = tolerations
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

// BuildContainerBased constructs an InferenceService that uses a custom container
// instead of KServe's native model serving. This is required for models that need
// trust_remote_code because vLLM must load directly from HuggingFace to execute
// custom model Python code. The native model serving downloads to /mnt/models
// which doesn't preserve the HuggingFace repo metadata needed for trust_remote_code.
func (b *InferenceServiceBuilder) BuildContainerBased() (*unstructured.Unstructured, error) {
	// Validate required fields
	if b.containerImage == "" {
		return nil, fmt.Errorf("containerImage is required for container-based deployment")
	}
	if b.modelID == "" {
		return nil, fmt.Errorf("modelID is required for container-based deployment")
	}

	// Build labels
	labels := map[string]interface{}{
		"app":         "vllm-inference",
		"model":       sanitizeLabelValue(b.servedName),
		"environment": b.environment,
		"managed-by":  "ai-model-operator",
	}

	// Build annotations with extended timeout for model loading
	annotations := map[string]interface{}{
		"serving.kserve.io/deploymentMode":      "Serverless",
		"serving.knative.dev/progress-deadline": "900s", // 15 min for large model download
	}

	// Build environment variables
	env := []interface{}{
		map[string]interface{}{
			"name":  "HF_HOME",
			"value": "/tmp/hf_home",
		},
	}
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

	// Build args: prepend --model=<modelID> to ensure it's used
	// Also add --served-model-name for API compatibility
	args := []interface{}{
		fmt.Sprintf("--model=%s", b.modelID),
	}
	if b.servedName != "" {
		args = append(args, fmt.Sprintf("--served-model-name=%s", b.servedName))
	}
	for _, arg := range b.runtimeArgs {
		args = append(args, arg)
	}

	// Build container spec
	container := map[string]interface{}{
		"name":      "kserve-container",
		"image":     b.containerImage,
		"args":      args,
		"env":       env,
		"resources": resources,
		"ports": []interface{}{
			map[string]interface{}{
				"containerPort": int64(8000),
				"name":          "http1",
				"protocol":      "TCP",
			},
		},
		// Startup probe with extended timeout for model download
		"startupProbe": map[string]interface{}{
			"httpGet": map[string]interface{}{
				"path": "/health",
				"port": int64(8000),
			},
			"initialDelaySeconds": int64(30),
			"periodSeconds":       int64(10),
			"failureThreshold":    int64(90), // 30s + 90*10s = 15 min max
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

	// Build predictor spec using containers (not native model serving)
	predictor := map[string]interface{}{
		"minReplicas": int64(b.minReplicas),
		"maxReplicas": int64(b.maxReplicas),
		"containers":  []interface{}{container},
	}

	// Add tolerations if specified
	if len(tolerations) > 0 {
		predictor["tolerations"] = tolerations
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
