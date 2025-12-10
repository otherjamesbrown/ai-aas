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

func TestInferenceServiceBuilder_WithModelID(t *testing.T) {
	builder := NewInferenceServiceBuilder("test", "test-ns").
		WithModelID("meta-llama/Llama-2-7b-hf")

	if builder.modelID != "meta-llama/Llama-2-7b-hf" {
		t.Errorf("Expected modelID 'meta-llama/Llama-2-7b-hf', got '%s'", builder.modelID)
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
		WithModelID("mistralai/Mistral-7B-Instruct-v0.2").
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
	if annotations["serving.knative.dev/progress-deadline"] != "360s" {
		t.Errorf("Expected annotation 'serving.knative.dev/progress-deadline: 360s', got '%s'",
			annotations["serving.knative.dev/progress-deadline"])
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

	// Verify containers
	containers, found, err := unstructured.NestedSlice(predictor, "containers")
	if err != nil || !found {
		t.Fatal("Expected containers in predictor")
	}
	if len(containers) != 1 {
		t.Fatalf("Expected 1 container, got %d", len(containers))
	}

	container, ok := containers[0].(map[string]interface{})
	if !ok {
		t.Fatal("Expected container to be a map")
	}

	// Verify container image
	image, found, err := unstructured.NestedString(container, "image")
	if err != nil || !found {
		t.Fatal("Expected image in container")
	}
	if image != "vllm/vllm-openai:v0.6.3" {
		t.Errorf("Expected image 'vllm/vllm-openai:v0.6.3', got '%s'", image)
	}

	// Verify container args include model
	args, found, err := unstructured.NestedSlice(container, "args")
	if err != nil || !found {
		t.Fatal("Expected args in container")
	}
	if len(args) == 0 {
		t.Fatal("Expected non-empty args")
	}

	// First arg should be --model
	firstArg, ok := args[0].(string)
	if !ok {
		t.Fatal("Expected first arg to be string")
	}
	expectedArg := "--model=mistralai/Mistral-7B-Instruct-v0.2"
	if firstArg != expectedArg {
		t.Errorf("Expected first arg '%s', got '%s'", expectedArg, firstArg)
	}

	// Verify health probes
	if _, found, _ := unstructured.NestedMap(container, "startupProbe"); !found {
		t.Error("Expected startupProbe in container")
	}
	if _, found, _ := unstructured.NestedMap(container, "readinessProbe"); !found {
		t.Error("Expected readinessProbe in container")
	}
	if _, found, _ := unstructured.NestedMap(container, "livenessProbe"); !found {
		t.Error("Expected livenessProbe in container")
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

func TestInferenceServiceBuilder_Build_MissingModelID(t *testing.T) {
	builder := NewInferenceServiceBuilder("test", "test-ns").
		WithServedName("test-model").
		WithRuntime("vllm/vllm-openai:v0.6.3")

	_, err := builder.Build()
	if err == nil {
		t.Fatal("Expected error for missing modelID")
	}
	if err.Error() != "modelID is required" {
		t.Errorf("Expected error 'modelID is required', got '%s'", err.Error())
	}
}

func TestInferenceServiceBuilder_Build_MissingServedName(t *testing.T) {
	builder := NewInferenceServiceBuilder("test", "test-ns").
		WithModelID("test-model-id").
		WithRuntime("vllm/vllm-openai:v0.6.3")

	_, err := builder.Build()
	if err == nil {
		t.Fatal("Expected error for missing servedName")
	}
	if err.Error() != "servedName is required" {
		t.Errorf("Expected error 'servedName is required', got '%s'", err.Error())
	}
}

func TestInferenceServiceBuilder_Build_MissingRuntime(t *testing.T) {
	builder := NewInferenceServiceBuilder("test", "test-ns").
		WithModelID("test-model-id").
		WithServedName("test-model")

	_, err := builder.Build()
	if err == nil {
		t.Fatal("Expected error for missing runtime")
	}
	if err.Error() != "runtime image is required" {
		t.Errorf("Expected error 'runtime image is required', got '%s'", err.Error())
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
