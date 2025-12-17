package kserve

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestNewInferenceServiceBuilder(t *testing.T) {
	builder := NewInferenceServiceBuilder("test-model", "test-namespace")

	if builder == nil {
		t.Fatal("Expected non-nil builder")
	}

	if builder.name != "test-model" {
		t.Errorf("Expected name 'test-model', got '%s'", builder.name)
	}

	if builder.namespace != "test-namespace" {
		t.Errorf("Expected namespace 'test-namespace', got '%s'", builder.namespace)
	}

	// Check defaults
	if builder.minReplicas != 1 {
		t.Errorf("Expected default minReplicas 1, got %d", builder.minReplicas)
	}

	if builder.maxReplicas != 3 {
		t.Errorf("Expected default maxReplicas 3, got %d", builder.maxReplicas)
	}

	if builder.environment != "development" {
		t.Errorf("Expected default environment 'development', got '%s'", builder.environment)
	}
}

func TestInferenceServiceBuilder_WithStorageUri(t *testing.T) {
	builder := NewInferenceServiceBuilder("test", "test-ns").
		WithStorageUri("s3://my-bucket/models/llama-2-7b")

	if builder.storageUri != "s3://my-bucket/models/llama-2-7b" {
		t.Errorf("Expected storageUri 's3://my-bucket/models/llama-2-7b', got '%s'", builder.storageUri)
	}
}

func TestInferenceServiceBuilder_WithServedName(t *testing.T) {
	builder := NewInferenceServiceBuilder("test", "test-ns").
		WithServedName("llama-7b")

	if builder.servedName != "llama-7b" {
		t.Errorf("Expected servedName 'llama-7b', got '%s'", builder.servedName)
	}
}

func TestInferenceServiceBuilder_WithRuntime(t *testing.T) {
	builder := NewInferenceServiceBuilder("test", "test-ns").
		WithRuntime("vllm/vllm-openai:v0.6.3")

	if builder.runtime != "vllm/vllm-openai:v0.6.3" {
		t.Errorf("Expected runtime 'vllm/vllm-openai:v0.6.3', got '%s'", builder.runtime)
	}
}

func TestInferenceServiceBuilder_WithScaling(t *testing.T) {
	builder := NewInferenceServiceBuilder("test", "test-ns").
		WithScaling(2, 10)

	if builder.minReplicas != 2 {
		t.Errorf("Expected minReplicas 2, got %d", builder.minReplicas)
	}

	if builder.maxReplicas != 10 {
		t.Errorf("Expected maxReplicas 10, got %d", builder.maxReplicas)
	}
}

func TestInferenceServiceBuilder_WithResources(t *testing.T) {
	resources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("8"),
			corev1.ResourceMemory: resource.MustParse("32Gi"),
			"nvidia.com/gpu":      resource.MustParse("2"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("16"),
			corev1.ResourceMemory: resource.MustParse("64Gi"),
			"nvidia.com/gpu":      resource.MustParse("2"),
		},
	}

	builder := NewInferenceServiceBuilder("test", "test-ns").
		WithResources(resources)

	if builder.resources.Requests.Cpu().String() != "8" {
		t.Errorf("Expected CPU request '8', got '%s'", builder.resources.Requests.Cpu().String())
	}

	if builder.resources.Requests.Memory().String() != "32Gi" {
		t.Errorf("Expected memory request '32Gi', got '%s'", builder.resources.Requests.Memory().String())
	}
}

func TestInferenceServiceBuilder_WithTolerations(t *testing.T) {
	tolerations := []corev1.Toleration{
		{
			Key:      "custom-key",
			Operator: corev1.TolerationOpEqual,
			Value:    "custom-value",
			Effect:   corev1.TaintEffectNoSchedule,
		},
	}

	builder := NewInferenceServiceBuilder("test", "test-ns").
		WithTolerations(tolerations)

	if len(builder.tolerations) != 1 {
		t.Fatalf("Expected 1 toleration, got %d", len(builder.tolerations))
	}

	if builder.tolerations[0].Key != "custom-key" {
		t.Errorf("Expected toleration key 'custom-key', got '%s'", builder.tolerations[0].Key)
	}
}

func TestInferenceServiceBuilder_WithNodeSelector(t *testing.T) {
	nodeSelector := map[string]string{
		"gpu-type": "nvidia-a100",
	}

	builder := NewInferenceServiceBuilder("test", "test-ns").
		WithNodeSelector(nodeSelector)

	if len(builder.nodeSelector) != 1 {
		t.Fatalf("Expected 1 node selector, got %d", len(builder.nodeSelector))
	}

	if builder.nodeSelector["gpu-type"] != "nvidia-a100" {
		t.Errorf("Expected node selector 'gpu-type: nvidia-a100', got '%s'", builder.nodeSelector["gpu-type"])
	}
}

func TestInferenceServiceBuilder_WithRuntimeArgs(t *testing.T) {
	args := []string{"--custom-arg=value", "--another-arg"}

	builder := NewInferenceServiceBuilder("test", "test-ns").
		WithRuntimeArgs(args)

	if len(builder.runtimeArgs) != 2 {
		t.Fatalf("Expected 2 runtime args, got %d", len(builder.runtimeArgs))
	}

	if builder.runtimeArgs[0] != "--custom-arg=value" {
		t.Errorf("Expected arg '--custom-arg=value', got '%s'", builder.runtimeArgs[0])
	}
}

func TestInferenceServiceBuilder_WithRuntimeEnv(t *testing.T) {
	env := []corev1.EnvVar{
		{
			Name:  "CUSTOM_ENV",
			Value: "custom-value",
		},
	}

	builder := NewInferenceServiceBuilder("test", "test-ns").
		WithRuntimeEnv(env)

	if len(builder.runtimeEnv) != 1 {
		t.Fatalf("Expected 1 env var, got %d", len(builder.runtimeEnv))
	}

	if builder.runtimeEnv[0].Name != "CUSTOM_ENV" {
		t.Errorf("Expected env name 'CUSTOM_ENV', got '%s'", builder.runtimeEnv[0].Name)
	}
}

func TestInferenceServiceBuilder_WithOwnerReference(t *testing.T) {
	ownerRef := &metav1.OwnerReference{
		APIVersion: "v1",
		Kind:       "AIModel",
		Name:       "test-model",
		UID:        "12345",
	}

	builder := NewInferenceServiceBuilder("test", "test-ns").
		WithOwnerReference(ownerRef)

	if builder.ownerRef == nil {
		t.Fatal("Expected non-nil owner reference")
	}

	if builder.ownerRef.Kind != "AIModel" {
		t.Errorf("Expected owner reference kind 'AIModel', got '%s'", builder.ownerRef.Kind)
	}
}

func TestInferenceServiceBuilder_WithEnvironment(t *testing.T) {
	builder := NewInferenceServiceBuilder("test", "test-ns").
		WithEnvironment("production")

	if builder.environment != "production" {
		t.Errorf("Expected environment 'production', got '%s'", builder.environment)
	}
}

func TestInferenceServiceBuilder_Build(t *testing.T) {
	builder := NewInferenceServiceBuilder("test-model", "test-namespace").
		WithStorageUri("s3://my-bucket/models/mistral-7b").
		WithServedName("mistral-7b").
		WithRuntime("vllm/vllm-openai:v0.6.3").
		WithScaling(1, 3).
		WithEnvironment("staging")

	obj, err := builder.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if obj == nil {
		t.Fatal("Expected non-nil unstructured object")
	}

	// Verify GVK
	gvk := obj.GroupVersionKind()
	if gvk.Group != "serving.kserve.io" {
		t.Errorf("Expected group 'serving.kserve.io', got '%s'", gvk.Group)
	}
	if gvk.Version != "v1beta1" {
		t.Errorf("Expected version 'v1beta1', got '%s'", gvk.Version)
	}
	if gvk.Kind != "InferenceService" {
		t.Errorf("Expected kind 'InferenceService', got '%s'", gvk.Kind)
	}

	// Verify metadata
	if obj.GetName() != "test-model" {
		t.Errorf("Expected name 'test-model', got '%s'", obj.GetName())
	}
	if obj.GetNamespace() != "test-namespace" {
		t.Errorf("Expected namespace 'test-namespace', got '%s'", obj.GetNamespace())
	}

	// Verify labels
	labels := obj.GetLabels()
	if labels["app"] != "vllm-inference" {
		t.Errorf("Expected label 'app: vllm-inference', got '%s'", labels["app"])
	}
	if labels["model"] != "mistral-7b" {
		t.Errorf("Expected label 'model: mistral-7b', got '%s'", labels["model"])
	}
	if labels["environment"] != "staging" {
		t.Errorf("Expected label 'environment: staging', got '%s'", labels["environment"])
	}
	if labels["managed-by"] != "ai-model-operator" {
		t.Errorf("Expected label 'managed-by: ai-model-operator', got '%s'", labels["managed-by"])
	}

	// Verify annotations
	annotations := obj.GetAnnotations()
	if annotations["serving.kserve.io/deploymentMode"] != "Serverless" {
		t.Errorf("Expected annotation 'serving.kserve.io/deploymentMode: Serverless', got '%s'",
			annotations["serving.kserve.io/deploymentMode"])
	}

	// Verify probe config version annotation
	probeVersion := annotations["ai-aas.io/probe-config-version"]
	if probeVersion != probeConfigVersion {
		t.Errorf("Expected probe-config-version '%s', got '%s'", probeConfigVersion, probeVersion)
	}

	// Verify spec.predictor
	predictor, found, err := unstructured.NestedMap(obj.Object, "spec", "predictor")
	if err != nil {
		t.Fatalf("Error getting predictor: %v", err)
	}
	if !found {
		t.Fatal("Expected predictor in spec")
	}

	// Verify minReplicas/maxReplicas
	minReplicas, found, err := unstructured.NestedInt64(predictor, "minReplicas")
	if err != nil || !found {
		t.Fatal("Expected minReplicas in predictor")
	}
	if minReplicas != 1 {
		t.Errorf("Expected minReplicas 1, got %d", minReplicas)
	}

	maxReplicas, found, err := unstructured.NestedInt64(predictor, "maxReplicas")
	if err != nil || !found {
		t.Fatal("Expected maxReplicas in predictor")
	}
	if maxReplicas != 3 {
		t.Errorf("Expected maxReplicas 3, got %d", maxReplicas)
	}

	// Verify model spec (KServe native)
	model, found, err := unstructured.NestedMap(predictor, "model")
	if err != nil || !found {
		t.Fatal("Expected model in predictor")
	}

	// Verify storageUri
	storageUri, found, err := unstructured.NestedString(model, "storageUri")
	if err != nil || !found {
		t.Fatal("Expected storageUri in model")
	}
	if storageUri != "s3://my-bucket/models/mistral-7b" {
		t.Errorf("Expected storageUri 's3://my-bucket/models/mistral-7b', got '%s'", storageUri)
	}

	// Verify runtime
	runtime, found, err := unstructured.NestedString(model, "runtime")
	if err != nil || !found {
		t.Fatal("Expected runtime in model")
	}
	if runtime != "vllm/vllm-openai:v0.6.3" {
		t.Errorf("Expected runtime 'vllm/vllm-openai:v0.6.3', got '%s'", runtime)
	}

	// Verify modelFormat
	modelFormat, found, err := unstructured.NestedMap(model, "modelFormat")
	if err != nil || !found {
		t.Fatal("Expected modelFormat in model")
	}
	formatName, found, err := unstructured.NestedString(modelFormat, "name")
	if err != nil || !found {
		t.Fatal("Expected name in modelFormat")
	}
	if formatName != "vllm" {
		t.Errorf("Expected modelFormat name 'vllm', got '%s'", formatName)
	}

	// Verify tolerations
	tolerations, found, err := unstructured.NestedSlice(predictor, "tolerations")
	if err != nil || !found {
		t.Fatal("Expected tolerations in predictor")
	}
	if len(tolerations) != 2 {
		t.Fatalf("Expected 2 tolerations, got %d", len(tolerations))
	}
}

func TestInferenceServiceBuilder_Build_MissingStorageUri(t *testing.T) {
	builder := NewInferenceServiceBuilder("test", "test-ns").
		WithRuntime("vllm/vllm-openai:v0.6.3")

	_, err := builder.Build()
	if err == nil {
		t.Fatal("Expected error for missing storageUri")
	}
	if err.Error() != "storageUri is required" {
		t.Errorf("Expected error 'storageUri is required', got '%s'", err.Error())
	}
}

func TestInferenceServiceBuilder_Build_MissingRuntime(t *testing.T) {
	builder := NewInferenceServiceBuilder("test", "test-ns").
		WithStorageUri("s3://my-bucket/models/test")

	_, err := builder.Build()
	if err == nil {
		t.Fatal("Expected error for missing runtime")
	}
	if err.Error() != "runtime is required" {
		t.Errorf("Expected error 'runtime is required', got '%s'", err.Error())
	}
}

func TestGetStatus_Ready(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "serving.kserve.io/v1beta1",
			"kind":       "InferenceService",
			"metadata": map[string]interface{}{
				"name":      "test-model",
				"namespace": "test-ns",
			},
			"status": map[string]interface{}{
				"url": "http://test-model.test-ns.svc.cluster.local",
				"conditions": []interface{}{
					map[string]interface{}{
						"type":    "Ready",
						"status":  "True",
						"reason":  "ServiceReady",
						"message": "Service is ready",
					},
				},
				"readyReplicas": int64(2),
			},
		},
	}

	status, err := GetStatus(obj)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if !status.Ready {
		t.Error("Expected status to be ready")
	}

	if status.URL != "http://test-model.test-ns.svc.cluster.local" {
		t.Errorf("Expected URL 'http://test-model.test-ns.svc.cluster.local', got '%s'", status.URL)
	}

	if status.ReadyReplicas != 2 {
		t.Errorf("Expected 2 ready replicas, got %d", status.ReadyReplicas)
	}

	if len(status.Conditions) != 1 {
		t.Fatalf("Expected 1 condition, got %d", len(status.Conditions))
	}

	if status.Conditions[0].Type != "Ready" {
		t.Errorf("Expected condition type 'Ready', got '%s'", status.Conditions[0].Type)
	}
	if status.Conditions[0].Status != "True" {
		t.Errorf("Expected condition status 'True', got '%s'", status.Conditions[0].Status)
	}
}

func TestGetStatus_NotReady(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "serving.kserve.io/v1beta1",
			"kind":       "InferenceService",
			"metadata": map[string]interface{}{
				"name":      "test-model",
				"namespace": "test-ns",
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":    "Ready",
						"status":  "False",
						"reason":  "PodNotReady",
						"message": "Waiting for pod to be ready",
					},
				},
			},
		},
	}

	status, err := GetStatus(obj)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if status.Ready {
		t.Error("Expected status to not be ready")
	}

	if len(status.Conditions) != 1 {
		t.Fatalf("Expected 1 condition, got %d", len(status.Conditions))
	}

	if status.Conditions[0].Reason != "PodNotReady" {
		t.Errorf("Expected condition reason 'PodNotReady', got '%s'", status.Conditions[0].Reason)
	}
}

func TestGetStatus_EmptyStatus(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "serving.kserve.io/v1beta1",
			"kind":       "InferenceService",
			"metadata": map[string]interface{}{
				"name":      "test-model",
				"namespace": "test-ns",
			},
		},
	}

	status, err := GetStatus(obj)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if status.Ready {
		t.Error("Expected status to not be ready")
	}

	if status.URL != "" {
		t.Errorf("Expected empty URL, got '%s'", status.URL)
	}

	if len(status.Conditions) != 0 {
		t.Errorf("Expected 0 conditions, got %d", len(status.Conditions))
	}
}

func TestGetStatus_NilObject(t *testing.T) {
	_, err := GetStatus(nil)
	if err == nil {
		t.Fatal("Expected error for nil object")
	}
	if err.Error() != "unstructured object is nil" {
		t.Errorf("Expected error 'unstructured object is nil', got '%s'", err.Error())
	}
}

func TestInferenceServiceStatus_IsReady(t *testing.T) {
	status := &InferenceServiceStatus{Ready: true}
	if !status.IsReady() {
		t.Error("Expected IsReady to return true")
	}

	status = &InferenceServiceStatus{Ready: false}
	if status.IsReady() {
		t.Error("Expected IsReady to return false")
	}
}

func TestInferenceServiceStatus_GetCondition(t *testing.T) {
	status := &InferenceServiceStatus{
		Conditions: []Condition{
			{Type: "Ready", Status: "True"},
			{Type: "PredictorReady", Status: "True"},
		},
	}

	cond := status.GetCondition("Ready")
	if cond == nil {
		t.Fatal("Expected to find Ready condition")
	}
	if cond.Type != "Ready" {
		t.Errorf("Expected condition type 'Ready', got '%s'", cond.Type)
	}

	cond = status.GetCondition("NonExistent")
	if cond != nil {
		t.Error("Expected nil for non-existent condition")
	}
}

func TestInferenceServiceStatus_String(t *testing.T) {
	// Test ready status
	status := &InferenceServiceStatus{
		Ready:         true,
		URL:           "http://test.example.com",
		ReadyReplicas: 2,
	}
	str := status.String()
	expectedStr := "Ready (URL: http://test.example.com, Replicas: 2)"
	if str != expectedStr {
		t.Errorf("Expected string '%s', got '%s'", expectedStr, str)
	}

	// Test not ready with condition
	status = &InferenceServiceStatus{
		Ready: false,
		Conditions: []Condition{
			{Type: "Ready", Status: "False", Reason: "PodNotReady", Message: "Waiting for pod"},
		},
	}
	str = status.String()
	expectedStr = "Not Ready: PodNotReady - Waiting for pod"
	if str != expectedStr {
		t.Errorf("Expected string '%s', got '%s'", expectedStr, str)
	}

	// Test not ready without conditions
	status = &InferenceServiceStatus{
		Ready: false,
	}
	str = status.String()
	if str != "Not Ready" {
		t.Errorf("Expected string 'Not Ready', got '%s'", str)
	}
}

func TestGetString(t *testing.T) {
	m := map[string]interface{}{
		"key1": "value1",
		"key2": 123,
		"key3": nil,
	}

	// Test existing string
	if val := getString(m, "key1"); val != "value1" {
		t.Errorf("Expected 'value1', got '%s'", val)
	}

	// Test non-string value
	if val := getString(m, "key2"); val != "" {
		t.Errorf("Expected empty string for non-string value, got '%s'", val)
	}

	// Test nil value
	if val := getString(m, "key3"); val != "" {
		t.Errorf("Expected empty string for nil value, got '%s'", val)
	}

	// Test non-existent key
	if val := getString(m, "key4"); val != "" {
		t.Errorf("Expected empty string for non-existent key, got '%s'", val)
	}

	// Test nil map
	if val := getString(nil, "key1"); val != "" {
		t.Errorf("Expected empty string for nil map, got '%s'", val)
	}
}

// TestInferenceServiceBuilder_BuildContainerBased_HasLivenessProbe verifies that
// container-based InferenceServices have proper liveness probe configuration to
// prevent pod restarts during model initialization.
//
// This test addresses ai-aas-jkwm: Verifies proper initialDelaySeconds for liveness probe.
func TestInferenceServiceBuilder_BuildContainerBased_HasLivenessProbe(t *testing.T) {
	builder := NewInferenceServiceBuilder("test-model", "test-namespace").
		WithContainerImage("vllm/vllm-openai:v0.6.3").
		WithModelID("unsloth/gpt-oss-20b").
		WithServedName("gpt-oss-20b").
		WithScaling(1, 3)

	obj, err := builder.BuildContainerBased()
	if err != nil {
		t.Fatalf("BuildContainerBased failed: %v", err)
	}

	// Verify probe config version annotation is set
	annotations := obj.GetAnnotations()
	probeVersion, found := annotations["ai-aas.io/probe-config-version"]
	if !found {
		t.Error("Expected ai-aas.io/probe-config-version annotation to be set")
	}
	if probeVersion != probeConfigVersion {
		t.Errorf("Expected probe-config-version '%s', got '%s'", probeConfigVersion, probeVersion)
	}

	// Get predictor.containers[0]
	predictor, found, err := unstructured.NestedMap(obj.Object, "spec", "predictor")
	if err != nil || !found {
		t.Fatal("Expected predictor in spec")
	}

	containers, found, err := unstructured.NestedSlice(predictor, "containers")
	if err != nil || !found {
		t.Fatal("Expected containers in predictor")
	}

	if len(containers) != 1 {
		t.Fatalf("Expected 1 container, got %d", len(containers))
	}

	container, ok := containers[0].(map[string]interface{})
	if !ok {
		t.Fatal("Container is not a map")
	}

	// Verify livenessProbe exists
	livenessProbe, found, err := unstructured.NestedMap(container, "livenessProbe")
	if err != nil || !found {
		t.Fatal("Expected livenessProbe in container")
	}

	// Verify initialDelaySeconds is set to 300 (5 minutes)
	initialDelaySeconds, found, err := unstructured.NestedInt64(livenessProbe, "initialDelaySeconds")
	if err != nil || !found {
		t.Fatal("Expected initialDelaySeconds in livenessProbe")
	}
	if initialDelaySeconds != 300 {
		t.Errorf("Expected initialDelaySeconds 300 (5 min), got %d", initialDelaySeconds)
	}

	// Verify periodSeconds
	periodSeconds, found, err := unstructured.NestedInt64(livenessProbe, "periodSeconds")
	if err != nil || !found {
		t.Fatal("Expected periodSeconds in livenessProbe")
	}
	if periodSeconds != 30 {
		t.Errorf("Expected periodSeconds 30, got %d", periodSeconds)
	}

	// Verify failureThreshold
	failureThreshold, found, err := unstructured.NestedInt64(livenessProbe, "failureThreshold")
	if err != nil || !found {
		t.Fatal("Expected failureThreshold in livenessProbe")
	}
	if failureThreshold != 3 {
		t.Errorf("Expected failureThreshold 3, got %d", failureThreshold)
	}

	// Verify timeoutSeconds
	timeoutSeconds, found, err := unstructured.NestedInt64(livenessProbe, "timeoutSeconds")
	if err != nil || !found {
		t.Fatal("Expected timeoutSeconds in livenessProbe")
	}
	if timeoutSeconds != 5 {
		t.Errorf("Expected timeoutSeconds 5, got %d", timeoutSeconds)
	}

	// Verify httpGet path and port
	httpGet, found, err := unstructured.NestedMap(livenessProbe, "httpGet")
	if err != nil || !found {
		t.Fatal("Expected httpGet in livenessProbe")
	}

	path, found, err := unstructured.NestedString(httpGet, "path")
	if err != nil || !found {
		t.Fatal("Expected path in httpGet")
	}
	if path != "/health" {
		t.Errorf("Expected path '/health', got '%s'", path)
	}

	port, found, err := unstructured.NestedInt64(httpGet, "port")
	if err != nil || !found {
		t.Fatal("Expected port in httpGet")
	}
	if port != 8000 {
		t.Errorf("Expected port 8000, got %d", port)
	}

	// Verify readinessProbe exists and has correct configuration
	readinessProbe, found, err := unstructured.NestedMap(container, "readinessProbe")
	if err != nil || !found {
		t.Fatal("Expected readinessProbe in container")
	}

	// Verify readinessProbe periodSeconds
	readinessPeriodSeconds, found, err := unstructured.NestedInt64(readinessProbe, "periodSeconds")
	if err != nil || !found {
		t.Fatal("Expected periodSeconds in readinessProbe")
	}
	if readinessPeriodSeconds != 10 {
		t.Errorf("Expected readinessProbe periodSeconds 10, got %d", readinessPeriodSeconds)
	}

	// Verify readinessProbe failureThreshold
	readinessFailureThreshold, found, err := unstructured.NestedInt64(readinessProbe, "failureThreshold")
	if err != nil || !found {
		t.Fatal("Expected failureThreshold in readinessProbe")
	}
	if readinessFailureThreshold != 3 {
		t.Errorf("Expected readinessProbe failureThreshold 3, got %d", readinessFailureThreshold)
	}

	// Verify readinessProbe timeoutSeconds
	readinessTimeoutSeconds, found, err := unstructured.NestedInt64(readinessProbe, "timeoutSeconds")
	if err != nil || !found {
		t.Fatal("Expected timeoutSeconds in readinessProbe")
	}
	if readinessTimeoutSeconds != 5 {
		t.Errorf("Expected readinessProbe timeoutSeconds 5, got %d", readinessTimeoutSeconds)
	}

	// Verify readinessProbe httpGet
	readinessHttpGet, found, err := unstructured.NestedMap(readinessProbe, "httpGet")
	if err != nil || !found {
		t.Fatal("Expected httpGet in readinessProbe")
	}

	readinessPath, found, err := unstructured.NestedString(readinessHttpGet, "path")
	if err != nil || !found {
		t.Fatal("Expected path in readinessProbe httpGet")
	}
	if readinessPath != "/health" {
		t.Errorf("Expected readinessProbe path '/health', got '%s'", readinessPath)
	}

	readinessPort, found, err := unstructured.NestedInt64(readinessHttpGet, "port")
	if err != nil || !found {
		t.Fatal("Expected port in readinessProbe httpGet")
	}
	if readinessPort != 8000 {
		t.Errorf("Expected readinessProbe port 8000, got %d", readinessPort)
	}

	// Verify startupProbe is NOT set (Knative rejects it in serverless mode)
	// See: https://github.com/knative/serving/issues/10037
	_, found, err = unstructured.NestedMap(container, "startupProbe")
	if found {
		t.Error("Expected NO startupProbe (Knative serverless mode rejects it)")
	}

	t.Log("Test passed: Container-based InferenceService has proper liveness and readiness probe configuration (no startupProbe)")
}

func TestInferenceServiceBuilder_WithDeploymentMode(t *testing.T) {
	tests := []struct {
		name           string
		deploymentMode string
		wantMode       string
	}{
		{
			name:           "explicit Serverless mode",
			deploymentMode: "Serverless",
			wantMode:       "Serverless",
		},
		{
			name:           "explicit RawDeployment mode",
			deploymentMode: "RawDeployment",
			wantMode:       "RawDeployment",
		},
		{
			name:           "empty mode defaults to Serverless",
			deploymentMode: "",
			wantMode:       "Serverless",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewInferenceServiceBuilder("test-model", "test-ns").
				WithStorageUri("s3://bucket/model").
				WithRuntime("vllm/vllm-openai:v0.6.3")

			if tt.deploymentMode != "" {
				builder = builder.WithDeploymentMode(tt.deploymentMode)
			}

			isvc, err := builder.Build()
			if err != nil {
				t.Fatalf("Build() failed: %v", err)
			}

			if isvc == nil {
				t.Fatal("Build() returned nil")
			}

			gotMode := isvc.GetAnnotations()["serving.kserve.io/deploymentMode"]
			if gotMode != tt.wantMode {
				t.Errorf("deploymentMode = %q, want %q", gotMode, tt.wantMode)
			}
		})
	}
}

// TestInferenceServiceBuilder_Build_RawDeployment verifies that Build() correctly
// generates InferenceService with RawDeployment mode annotations and configuration.
// This test addresses T014 from GPU Deployment Mode Migration (spec 030).
func TestInferenceServiceBuilder_Build_RawDeployment(t *testing.T) {
	builder := NewInferenceServiceBuilder("gpu-model", "system").
		WithDeploymentMode("RawDeployment").
		WithModelFormat("pytorch").
		WithStorageUri("s3://models/gpu-model").
		WithRuntime("vllm/vllm-openai:v0.6.3").
		WithNodeSelector(map[string]string{"gpu": "nvidia"}).
		WithScaling(1, 1)

	obj, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() failed: %v", err)
	}

	// Verify basic metadata
	if obj.GetName() != "gpu-model" {
		t.Errorf("Name = %q, want %q", obj.GetName(), "gpu-model")
	}
	if obj.GetNamespace() != "system" {
		t.Errorf("Namespace = %q, want %q", obj.GetNamespace(), "system")
	}

	// Verify deployment mode annotation
	annotations := obj.GetAnnotations()
	gotMode := annotations["serving.kserve.io/deploymentMode"]
	if gotMode != "RawDeployment" {
		t.Errorf("deploymentMode annotation = %q, want %q", gotMode, "RawDeployment")
	}

	// Verify nodeSelector is applied (important for GPU scheduling)
	predictor, found, err := unstructured.NestedMap(obj.Object, "spec", "predictor")
	if err != nil {
		t.Fatalf("Error getting predictor: %v", err)
	}
	if !found {
		t.Fatal("Expected predictor in spec")
	}

	nodeSelector, found, err := unstructured.NestedStringMap(predictor, "nodeSelector")
	if err != nil {
		t.Fatalf("Error getting nodeSelector: %v", err)
	}
	if !found {
		t.Error("NodeSelector is nil, expected gpu: nvidia")
	} else if nodeSelector["gpu"] != "nvidia" {
		t.Errorf("NodeSelector[gpu] = %q, want %q", nodeSelector["gpu"], "nvidia")
	}

	// Verify minReplicas and maxReplicas (GPU workloads often use min=max=1)
	minReplicas, found, err := unstructured.NestedInt64(predictor, "minReplicas")
	if err != nil || !found {
		t.Fatal("Expected minReplicas in predictor")
	}
	if minReplicas != 1 {
		t.Errorf("minReplicas = %d, want 1", minReplicas)
	}

	maxReplicas, found, err := unstructured.NestedInt64(predictor, "maxReplicas")
	if err != nil || !found {
		t.Fatal("Expected maxReplicas in predictor")
	}
	if maxReplicas != 1 {
		t.Errorf("maxReplicas = %d, want 1", maxReplicas)
	}

	// Verify model spec contains correct fields
	model, found, err := unstructured.NestedMap(predictor, "model")
	if err != nil || !found {
		t.Fatal("Expected model in predictor")
	}

	storageUri, found, err := unstructured.NestedString(model, "storageUri")
	if err != nil || !found {
		t.Fatal("Expected storageUri in model")
	}
	if storageUri != "s3://models/gpu-model" {
		t.Errorf("storageUri = %q, want %q", storageUri, "s3://models/gpu-model")
	}

	// Verify modelFormat
	modelFormat, found, err := unstructured.NestedMap(model, "modelFormat")
	if err != nil || !found {
		t.Fatal("Expected modelFormat in model")
	}
	formatName, found, err := unstructured.NestedString(modelFormat, "name")
	if err != nil || !found {
		t.Fatal("Expected name in modelFormat")
	}
	if formatName != "pytorch" {
		t.Errorf("modelFormat.name = %q, want %q", formatName, "pytorch")
	}
}
