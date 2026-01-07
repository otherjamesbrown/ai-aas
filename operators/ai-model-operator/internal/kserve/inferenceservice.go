package kserve

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// SpecHashAnnotation is the annotation key for storing the desired spec hash.
// This hash is computed from the operator's desired spec and used to detect
// real changes vs server-side mutations (like KServe adding default fields).
const SpecHashAnnotation = "ai-aas.io/spec-hash"

// InferenceServiceGVK is the GroupVersionKind for KServe InferenceService
var InferenceServiceGVK = schema.GroupVersionKind{
	Group:   "serving.kserve.io",
	Version: "v1beta1",
	Kind:    "InferenceService",
}

// probeConfigVersion tracks the version of health probe configuration.
// Increment this when probe settings change (initialDelaySeconds, periodSeconds, etc.)
// to trigger automatic updates of existing InferenceServices.
const probeConfigVersion = "v1"

// InferenceServiceBuilder provides a fluent API for building InferenceService resources
type InferenceServiceBuilder struct {
	name           string
	namespace      string
	storageUri     string
	modelFormat    string
	servedName     string
	runtime        string
	minReplicas    int32
	maxReplicas    int32
	resources      corev1.ResourceRequirements
	tolerations    []corev1.Toleration
	nodeSelector   map[string]string
	runtimeArgs    []string
	runtimeEnv     []corev1.EnvVar
	ownerRef       *metav1.OwnerReference
	environment    string
	updateStrategy string // "RollingUpdate" or "Recreate"
	// Container-based deployment fields (for trust_remote_code models)
	containerImage              string
	modelID                     string // HuggingFace model ID for direct loading
	containerRuntime            string // Runtime type for container-based deployment (vllm, tgi)
	livenessInitialDelaySeconds int32  // InitialDelaySeconds for liveness probe
	// Health probe configuration
	livenessPath   string // Path for liveness probe (default: /health)
	readinessPath  string // Path for readiness probe (default: /health)
	probePort      int32  // Port for health probes (default: 8000)
	deploymentMode string // Explicit deployment mode ("Serverless" or "RawDeployment")
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
				corev1.ResourceMemory: resource.MustParse("24Gi"),
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
		runtimeArgs:                 []string{}, // Runtime args are now set per-model in the controller
		environment:                 "development",
		livenessInitialDelaySeconds: 300, // Default 5 minutes for model loading
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

// WithContainerRuntime sets the runtime type for container-based deployment (vllm, tgi)
func (b *InferenceServiceBuilder) WithContainerRuntime(runtime string) *InferenceServiceBuilder {
	b.containerRuntime = runtime
	return b
}

// WithUpdateStrategy sets the update strategy (RollingUpdate or Recreate)
func (b *InferenceServiceBuilder) WithUpdateStrategy(strategy string) *InferenceServiceBuilder {
	b.updateStrategy = strategy
	return b
}

// WithLivenessInitialDelay sets the initialDelaySeconds for the liveness probe
func (b *InferenceServiceBuilder) WithLivenessInitialDelay(seconds int32) *InferenceServiceBuilder {
	b.livenessInitialDelaySeconds = seconds
	return b
}

// WithHealthProbes sets the health probe paths and port
func (b *InferenceServiceBuilder) WithHealthProbes(livenessPath, readinessPath string, port int32) *InferenceServiceBuilder {
	b.livenessPath = livenessPath
	b.readinessPath = readinessPath
	b.probePort = port
	return b
}

// WithDeploymentMode sets the explicit deployment mode.
// Valid values: "Serverless" (Knative), "RawDeployment" (standard K8s Deployment)
// If not set, defaults to "Serverless" for backward compatibility.
func (b *InferenceServiceBuilder) WithDeploymentMode(mode string) *InferenceServiceBuilder {
	b.deploymentMode = mode
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
	//
	// Deployment mode selection:
	// - Serverless (Knative): Default mode with autoscaling, but has restrictions
	//   (single port, no nodeSelector allowed by Knative validation)
	// - RawDeployment: Uses standard Kubernetes Deployments, allows nodeSelector
	//   and multiple ports for runtimes like TensorRT-LLM/Triton
	//
	// Use explicit mode if set, otherwise default to Serverless for backward compatibility
	mode := b.deploymentMode
	if mode == "" {
		mode = "Serverless"
	}
	annotations := map[string]interface{}{
		"serving.kserve.io/deploymentMode": mode,
		"ai-aas.io/probe-config-version":   probeConfigVersion, // Track probe config version for reconciliation
	}

	// Apply update strategy annotation for Knative
	// Recreate strategy: Set rollout duration to 0 to terminate old revision immediately
	// This frees GPU resources before new pods are scheduled
	if b.updateStrategy == "Recreate" {
		annotations["serving.knative.dev/rollout-duration"] = "0"
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

	// Compute and store spec hash for change detection
	if err := addSpecHash(obj); err != nil {
		return nil, fmt.Errorf("failed to compute spec hash: %w", err)
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
	// Use explicit mode if set, otherwise default to Serverless for backward compatibility
	mode := b.deploymentMode
	if mode == "" {
		mode = "Serverless"
	}
	annotations := map[string]interface{}{
		"serving.kserve.io/deploymentMode":      mode,
		"serving.knative.dev/progress-deadline": "900s",             // 15 min for large model download
		"ai-aas.io/probe-config-version":        probeConfigVersion, // Track probe config version for reconciliation
	}

	// Apply update strategy annotation for Knative
	// Recreate strategy: Set rollout duration to 0 to terminate old revision immediately
	// This frees GPU resources before new pods are scheduled
	if b.updateStrategy == "Recreate" {
		annotations["serving.knative.dev/rollout-duration"] = "0"
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

	// Build args based on container runtime type
	var args []interface{}
	switch b.containerRuntime {
	case "tgi":
		// TGI uses --model-id for model path and requires explicit port
		args = append(args, fmt.Sprintf("--model-id=%s", b.modelID))
		args = append(args, "--port=8000") // TGI defaults to 80, but we need 8000 for probes
	default:
		// vLLM uses --model and --served-model-name
		args = append(args, fmt.Sprintf("--model=%s", b.modelID))
		if b.servedName != "" {
			args = append(args, fmt.Sprintf("--served-model-name=%s", b.servedName))
		}
	}
	for _, arg := range b.runtimeArgs {
		args = append(args, arg)
	}

	// Set default probe paths and port if not specified
	livenessPath := b.livenessPath
	if livenessPath == "" {
		livenessPath = "/health"
	}
	readinessPath := b.readinessPath
	if readinessPath == "" {
		readinessPath = "/health"
	}
	probePort := b.probePort
	if probePort == 0 {
		probePort = 8000
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
		// NOTE: We do NOT use startupProbe here because Knative Serving's admission
		// webhook actively rejects startupProbe in serverless mode. Instead, we rely
		// on livenessProbe.initialDelaySeconds to allow time for model loading.
		// See: https://github.com/knative/serving/issues/10037
		"readinessProbe": map[string]interface{}{
			"httpGet": map[string]interface{}{
				"path": readinessPath,
				"port": int64(probePort),
			},
			"periodSeconds":    int64(10),
			"failureThreshold": int64(3),
			"timeoutSeconds":   int64(5),
		},
		"livenessProbe": map[string]interface{}{
			"httpGet": map[string]interface{}{
				"path": livenessPath,
				"port": int64(probePort),
			},
			"initialDelaySeconds": int64(b.livenessInitialDelaySeconds),
			"periodSeconds":       int64(30),
			"failureThreshold":    int64(3),
			"timeoutSeconds":      int64(5),
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

	// Compute and store spec hash for change detection
	if err := addSpecHash(obj); err != nil {
		return nil, fmt.Errorf("failed to compute spec hash: %w", err)
	}

	return obj, nil
}

// computeSpecHash computes a SHA256 hash of the spec for change detection.
// This allows us to detect real changes to the desired spec without being
// affected by server-side mutations (KServe adding defaults, etc.).
func computeSpecHash(spec map[string]interface{}) (string, error) {
	// JSON marshal with sorted keys for deterministic output
	data, err := json.Marshal(spec)
	if err != nil {
		return "", fmt.Errorf("failed to marshal spec: %w", err)
	}
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash[:8]), nil // Use first 8 bytes (16 hex chars) for readability
}

// addSpecHash computes the spec hash and adds it to the object's annotations.
func addSpecHash(obj *unstructured.Unstructured) error {
	spec, found, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil || !found {
		return fmt.Errorf("spec not found in object")
	}

	hash, err := computeSpecHash(spec)
	if err != nil {
		return err
	}

	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[SpecHashAnnotation] = hash
	obj.SetAnnotations(annotations)
	return nil
}

// TritonMultiModelBuilder provides a builder for multi-model Triton deployments.
// This creates an InferenceService that hosts multiple TRT-LLM models on a single GPU
// using Triton's --model-control-mode=explicit and --load-model flags.
type TritonMultiModelBuilder struct {
	name         string
	namespace    string
	s3Bucket     string
	models       []MultiModelEntry
	resources    corev1.ResourceRequirements
	tolerations  []corev1.Toleration
	nodeSelector map[string]string
	runtimeEnv   []corev1.EnvVar
	ownerRef     *metav1.OwnerReference
	environment  string
	runtimeName  string // ClusterServingRuntime name (e.g., "kserve-triton-multi-model-blackwell")
}

// MultiModelEntry represents a single model in a multi-model deployment
type MultiModelEntry struct {
	Name           string
	S3Key          string
	KVCacheFraction string
	MaxBatchSize   *int32
}

// NewTritonMultiModelBuilder creates a new builder for Triton multi-model deployments
func NewTritonMultiModelBuilder(name, namespace string) *TritonMultiModelBuilder {
	return &TritonMultiModelBuilder{
		name:      name,
		namespace: namespace,
		// Default resources for multi-model (higher than single model)
		resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("8"),
				corev1.ResourceMemory: resource.MustParse("48Gi"),
				"nvidia.com/gpu":      resource.MustParse("1"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("16"),
				corev1.ResourceMemory: resource.MustParse("96Gi"),
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
		environment: "development",
		runtimeName: "kserve-triton-multi-model-blackwell", // Default runtime
	}
}

// WithS3Bucket sets the S3 bucket for model storage
func (b *TritonMultiModelBuilder) WithS3Bucket(bucket string) *TritonMultiModelBuilder {
	b.s3Bucket = bucket
	return b
}

// WithModels sets the models to deploy
func (b *TritonMultiModelBuilder) WithModels(models []MultiModelEntry) *TritonMultiModelBuilder {
	b.models = models
	return b
}

// WithResources sets the resource requirements
func (b *TritonMultiModelBuilder) WithResources(r corev1.ResourceRequirements) *TritonMultiModelBuilder {
	b.resources = r
	return b
}

// WithTolerations sets the pod tolerations
func (b *TritonMultiModelBuilder) WithTolerations(t []corev1.Toleration) *TritonMultiModelBuilder {
	b.tolerations = t
	return b
}

// WithNodeSelector sets the node selector
func (b *TritonMultiModelBuilder) WithNodeSelector(ns map[string]string) *TritonMultiModelBuilder {
	b.nodeSelector = ns
	return b
}

// WithRuntimeEnv sets the container environment variables
func (b *TritonMultiModelBuilder) WithRuntimeEnv(env []corev1.EnvVar) *TritonMultiModelBuilder {
	b.runtimeEnv = env
	return b
}

// WithOwnerReference sets the owner reference for garbage collection
func (b *TritonMultiModelBuilder) WithOwnerReference(ref *metav1.OwnerReference) *TritonMultiModelBuilder {
	b.ownerRef = ref
	return b
}

// WithEnvironment sets the environment label
func (b *TritonMultiModelBuilder) WithEnvironment(env string) *TritonMultiModelBuilder {
	b.environment = env
	return b
}

// WithRuntimeName sets the ClusterServingRuntime name to use
func (b *TritonMultiModelBuilder) WithRuntimeName(name string) *TritonMultiModelBuilder {
	b.runtimeName = name
	return b
}

// Build constructs the InferenceService for multi-model Triton deployment.
// Uses RawDeployment mode because:
// 1. Triton requires multiple ports (HTTP, gRPC, metrics)
// 2. Knative Serverless only supports single port containers
// 3. We need nodeSelector support for GPU pinning
func (b *TritonMultiModelBuilder) Build() (*unstructured.Unstructured, error) {
	// Validate required fields
	if len(b.models) < 2 {
		return nil, fmt.Errorf("at least 2 models required for multi-model deployment")
	}
	if b.s3Bucket == "" {
		return nil, fmt.Errorf("s3Bucket is required for multi-model deployment")
	}

	// Build the storage URI pointing to the model repository root
	// Each model's S3Key is relative to this bucket
	storageUri := fmt.Sprintf("s3://%s/", b.s3Bucket)

	// Build model format name - matches supportedModelFormats in ClusterServingRuntime
	modelFormat := "triton-multi-model-blackwell"

	// Build labels
	labels := map[string]interface{}{
		"app":         "triton-multi-model",
		"environment": b.environment,
		"managed-by":  "ai-model-operator",
	}

	// Build annotations
	// RawDeployment is required for multi-port Triton
	annotations := map[string]interface{}{
		"serving.kserve.io/deploymentMode": "RawDeployment",
		"ai-aas.io/probe-config-version":   probeConfigVersion,
		"ai-aas.io/multi-model":            "true",
	}

	// Build --load-model args for each model
	// Format: --load-model=<model-name>
	// The model name must match the directory name in the model repository
	args := make([]interface{}, 0, len(b.models))
	for _, model := range b.models {
		args = append(args, fmt.Sprintf("--load-model=%s", model.Name))
	}

	// Build environment variables
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

	// Build model spec using KServe native model serving with args
	// The ClusterServingRuntime handles the Triton server configuration
	model := map[string]interface{}{
		"storageUri": storageUri,
		"runtime":    b.runtimeName,
		"modelFormat": map[string]interface{}{
			"name": modelFormat,
		},
		"resources": resources,
		"args":      args,
	}

	// Add environment variables if specified
	if len(env) > 0 {
		model["env"] = env
	}

	// Build predictor spec
	// minReplicas=1 because multi-model deployments should stay running
	// (scale-to-zero would require re-loading all models)
	predictor := map[string]interface{}{
		"minReplicas": int64(1),
		"maxReplicas": int64(1), // No autoscaling for multi-model (single GPU)
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

	// Compute and store spec hash for change detection
	if err := addSpecHash(obj); err != nil {
		return nil, fmt.Errorf("failed to compute spec hash: %w", err)
	}

	return obj, nil
}

// GetSpecHash returns the spec hash from an InferenceService's annotations.
func GetSpecHash(obj *unstructured.Unstructured) string {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return ""
	}
	return annotations[SpecHashAnnotation]
}

// SpecHashMatches checks if the spec hash of the desired object matches the existing object.
// Returns true if hashes match (no update needed), false if they differ (update needed).
func SpecHashMatches(desired, existing *unstructured.Unstructured) bool {
	desiredHash := GetSpecHash(desired)
	existingHash := GetSpecHash(existing)

	// If either hash is missing, fall back to requiring an update
	if desiredHash == "" || existingHash == "" {
		return false
	}

	return desiredHash == existingHash
}
