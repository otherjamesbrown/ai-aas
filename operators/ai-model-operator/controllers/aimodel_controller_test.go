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

package controllers

import (
	"context"
	"testing"

	aimodelv1alpha1 "github.com/ai-aas/ai-model-operator/api/v1alpha1"
	"github.com/ai-aas/ai-model-operator/internal/kserve"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func setupScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)   // Core Kubernetes types (corev1, appsv1, batchv1, etc.)
	_ = aimodelv1alpha1.AddToScheme(s)  // AIModel CRD types
	return s
}

func TestAIModelReconciler_InitialReconciliation(t *testing.T) {
	// Register operator types with the scheme
	s := setupScheme()

	// Define a sample AIModel object
	aiModelName := "test-model"
	aiModelNamespace := "default"
	replicas := int32(1)
	aiModel := &aimodelv1alpha1.AIModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aiModelName,
			Namespace: aiModelNamespace,
		},
		Spec: aimodelv1alpha1.AIModelSpec{
			ModelName: "mistral-7b",
			ModelID:   "mistral-7b-v0.1",
			S3Bucket:  "ai-models",
			S3Key:     "mistral-7b",
			Replicas:  &replicas,
			Enabled:   true,
		},
	}

	// Create a fake client with the sample object
	// Use WithStatusSubresource to handle status updates properly
	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(aiModel).
		WithStatusSubresource(aiModel).
		Build()

	// Create the Reconciler
	r := &AIModelReconciler{
		Client: cl,
		Scheme: s,
	}

	// Mock request
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      aiModelName,
			Namespace: aiModelNamespace,
		},
	}

	// Test: Initial Reconciliation
	// Note: In unit tests, the S3 artifact check will fail (no real AWS/S3 connection),
	// so the controller will create a downloader job to fetch from HuggingFace.
	// This validates that missing S3 artifacts trigger the download workflow.
	t.Log("Test: Initial Reconciliation")
	res, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}

	// Check result: Should requeue to wait for downloader job completion
	if !res.Requeue {
		t.Error("expected requeue when downloader job is created")
	}

	// Check Status Update: Phase should be "Downloading" as job is being created
	updatedAIModel := &aimodelv1alpha1.AIModel{}
	err = cl.Get(context.Background(), types.NamespacedName{Name: aiModelName, Namespace: aiModelNamespace}, updatedAIModel)
	if err != nil {
		t.Fatalf("get aimodel: (%v)", err)
	}
	if updatedAIModel.Status.Phase != aimodelv1alpha1.AIModelPhaseDownloading {
		t.Errorf("expected phase 'Downloading', got '%s'", updatedAIModel.Status.Phase)
	}

	t.Log("Test passed: Controller correctly creates downloader job for missing S3 artifacts")
}

func TestAIModelReconciler_CreatesInferenceService(t *testing.T) {
	// Register operator types with the scheme
	s := setupScheme()

	// Define a sample AIModel object
	aiModelName := "test-model"
	aiModelNamespace := "default"
	minReplicas := int32(0)
	maxReplicas := int32(2)
	aiModel := &aimodelv1alpha1.AIModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aiModelName,
			Namespace: aiModelNamespace,
			UID:       types.UID("test-uid-123"),
		},
		Spec: aimodelv1alpha1.AIModelSpec{
			ModelName:   "mistral-7b",
			ModelID:     "mistral-7b-v0.1",
			S3Bucket:    "ai-models",
			S3Key:       "mistral-7b",
			Runtime:     "vllm",
			MinReplicas: &minReplicas,
			MaxReplicas: &maxReplicas,
			Enabled:     true,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("2"),
					corev1.ResourceMemory: resource.MustParse("8Gi"),
					"nvidia.com/gpu":      resource.MustParse("1"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4"),
					corev1.ResourceMemory: resource.MustParse("16Gi"),
					"nvidia.com/gpu":      resource.MustParse("1"),
				},
			},
		},
	}

	// Create a completed job manually (to skip S3 check and downloader job)
	jobName := aiModelName + "-downloader"
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: aiModelNamespace,
		},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{
					Type:   batchv1.JobComplete,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}

	// Create a fake client with the sample objects
	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(aiModel, job).
		WithStatusSubresource(aiModel).
		Build()

	// Create the Reconciler
	r := &AIModelReconciler{
		Client: cl,
		Scheme: s,
	}

	// Update AIModel status to "Downloaded" (as if the job completed earlier)
	aiModel.Status.Phase = "Downloaded"
	err := cl.Status().Update(context.Background(), aiModel)
	if err != nil {
		t.Fatalf("update aimodel status: (%v)", err)
	}

	// Mock request
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      aiModelName,
			Namespace: aiModelNamespace,
		},
	}

	// Test: Reconcile should create InferenceService
	t.Log("Test: Reconciling to create InferenceService")
	res, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}

	// Should requeue for InferenceService readiness
	if !res.Requeue {
		t.Error("expected requeue for InferenceService creation/readiness")
	}

	// Check if InferenceService was created
	isvc := &unstructured.Unstructured{}
	isvc.SetGroupVersionKind(kserve.InferenceServiceGVK)
	err = cl.Get(context.Background(), types.NamespacedName{Name: aiModelName, Namespace: aiModelNamespace}, isvc)
	if err != nil {
		t.Fatalf("get InferenceService: (%v)", err)
	}

	// Verify InferenceService properties
	t.Log("Verifying InferenceService properties")

	// Check name and namespace
	if isvc.GetName() != aiModelName {
		t.Errorf("expected InferenceService name '%s', got '%s'", aiModelName, isvc.GetName())
	}
	if isvc.GetNamespace() != aiModelNamespace {
		t.Errorf("expected InferenceService namespace '%s', got '%s'", aiModelNamespace, isvc.GetNamespace())
	}

	// Check spec.predictor.minReplicas
	predictor, found, err := unstructured.NestedMap(isvc.Object, "spec", "predictor")
	if err != nil || !found {
		t.Fatalf("failed to get spec.predictor: %v", err)
	}

	minReplicasVal, found, err := unstructured.NestedInt64(predictor, "minReplicas")
	if err != nil || !found {
		t.Fatalf("failed to get minReplicas: %v", err)
	}
	if minReplicasVal != int64(minReplicas) {
		t.Errorf("expected minReplicas %d, got %d", minReplicas, minReplicasVal)
	}

	maxReplicasVal, found, err := unstructured.NestedInt64(predictor, "maxReplicas")
	if err != nil || !found {
		t.Fatalf("failed to get maxReplicas: %v", err)
	}
	if maxReplicasVal != int64(maxReplicas) {
		t.Errorf("expected maxReplicas %d, got %d", maxReplicas, maxReplicasVal)
	}

	// Check containers
	containers, found, err := unstructured.NestedSlice(predictor, "containers")
	if err != nil || !found || len(containers) == 0 {
		t.Fatalf("failed to get containers: %v", err)
	}

	container, ok := containers[0].(map[string]interface{})
	if !ok {
		t.Fatal("container is not a map")
	}

	// Check image
	image, found, err := unstructured.NestedString(container, "image")
	if err != nil || !found {
		t.Fatalf("failed to get image: %v", err)
	}
	expectedImage := "vllm/vllm-openai:v0.6.3"
	if image != expectedImage {
		t.Errorf("expected image '%s', got '%s'", expectedImage, image)
	}

	// Check args contain model ID
	args, found, err := unstructured.NestedStringSlice(container, "args")
	if err != nil || !found {
		t.Fatalf("failed to get args: %v", err)
	}
	modelArgFound := false
	for _, arg := range args {
		if arg == "--model=s3://ai-models/mistral-7b" {
			modelArgFound = true
			break
		}
	}
	if !modelArgFound {
		t.Errorf("expected --model arg with S3 path, got args: %v", args)
	}

	// Check Status Update: Phase should be "Deploying"
	updatedAIModel := &aimodelv1alpha1.AIModel{}
	err = cl.Get(context.Background(), types.NamespacedName{Name: aiModelName, Namespace: aiModelNamespace}, updatedAIModel)
	if err != nil {
		t.Fatalf("get aimodel: (%v)", err)
	}
	if updatedAIModel.Status.Phase != aimodelv1alpha1.AIModelPhaseDeploying {
		t.Errorf("expected phase '%s', got '%s'", aimodelv1alpha1.AIModelPhaseDeploying, updatedAIModel.Status.Phase)
	}

	t.Log("Test passed: Controller correctly creates InferenceService with proper configuration")
}

func TestAIModelReconciler_StatusUpdateFromInferenceService(t *testing.T) {
	// Note: This test verifies the updateStatusFromInferenceService method directly
	// Testing the full reconcile loop with status updates is complex due to
	// the fake client not supporting status subresources for unstructured objects.

	// Register operator types with the scheme
	s := setupScheme()

	// Define a sample AIModel object
	aiModelName := "test-model"
	aiModelNamespace := "default"
	aiModel := &aimodelv1alpha1.AIModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aiModelName,
			Namespace: aiModelNamespace,
			UID:       types.UID("test-uid-123"),
		},
		Spec: aimodelv1alpha1.AIModelSpec{
			ModelName: "mistral-7b",
			ModelID:   "mistral-7b-v0.1",
			S3Bucket:  "ai-models",
			S3Key:     "mistral-7b",
			Runtime:   "vllm",
			Enabled:   true,
		},
		Status: aimodelv1alpha1.AIModelStatus{
			Phase: aimodelv1alpha1.AIModelPhaseDeploying,
		},
	}

	// Create a ready InferenceService
	readyIsvc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "serving.kserve.io/v1beta1",
			"kind":       "InferenceService",
			"metadata": map[string]interface{}{
				"name":      aiModelName,
				"namespace": aiModelNamespace,
			},
			"spec": map[string]interface{}{
				"predictor": map[string]interface{}{
					"minReplicas": int64(1),
					"maxReplicas": int64(3),
				},
			},
			"status": map[string]interface{}{
				"url": "http://test-model.default.example.com",
				"conditions": []interface{}{
					map[string]interface{}{
						"type":    "Ready",
						"status":  "True",
						"reason":  "PredictorReady",
						"message": "Predictor is ready",
					},
				},
				"readyReplicas": int64(1),
			},
		},
	}

	// Create a fake client with the sample objects
	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(aiModel, readyIsvc).
		WithStatusSubresource(aiModel).
		Build()

	// Create the Reconciler
	r := &AIModelReconciler{
		Client: cl,
		Scheme: s,
	}

	// Test: Call updateStatusFromInferenceService directly
	t.Log("Test: Updating status from ready InferenceService")
	err := r.updateStatusFromInferenceService(context.Background(), aiModel)
	if err != nil {
		t.Fatalf("updateStatusFromInferenceService: (%v)", err)
	}

	// The method updates the AIModel in-memory, then calls Status().Update()
	// In the test, we need to verify the in-memory object was updated correctly

	// Check phase is Ready
	if aiModel.Status.Phase != aimodelv1alpha1.AIModelPhaseReady {
		t.Errorf("expected phase '%s', got '%s'", aimodelv1alpha1.AIModelPhaseReady, aiModel.Status.Phase)
	}

	// Check endpoint URL
	expectedURL := "http://test-model.default.example.com"
	if aiModel.Status.InferenceEndpoint != expectedURL {
		t.Errorf("expected endpoint '%s', got '%s'", expectedURL, aiModel.Status.InferenceEndpoint)
	}

	// Check ready replicas
	if aiModel.Status.ReadyReplicas != 1 {
		t.Errorf("expected readyReplicas 1, got %d", aiModel.Status.ReadyReplicas)
	}

	// Check InferenceService name
	if aiModel.Status.InferenceServiceName != aiModelName {
		t.Errorf("expected inferenceServiceName '%s', got '%s'", aiModelName, aiModel.Status.InferenceServiceName)
	}

	t.Log("Test passed: Controller correctly extracts and updates status from InferenceService")
}

func TestAIModelReconciler_FinalizerDeletesInferenceService(t *testing.T) {
	// Register operator types with the scheme
	s := setupScheme()

	// Define a sample AIModel object marked for deletion
	aiModelName := "test-model"
	aiModelNamespace := "default"
	now := metav1.Now()
	aiModel := &aimodelv1alpha1.AIModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:              aiModelName,
			Namespace:         aiModelNamespace,
			UID:               types.UID("test-uid-123"),
			Finalizers:        []string{aiModelFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: aimodelv1alpha1.AIModelSpec{
			ModelName: "mistral-7b",
			ModelID:   "mistral-7b-v0.1",
			S3Bucket:  "ai-models",
			S3Key:     "mistral-7b",
			Runtime:   "vllm",
			Enabled:   true,
		},
	}

	// Create an InferenceService that should be deleted
	isvc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "serving.kserve.io/v1beta1",
			"kind":       "InferenceService",
			"metadata": map[string]interface{}{
				"name":      aiModelName,
				"namespace": aiModelNamespace,
			},
			"spec": map[string]interface{}{
				"predictor": map[string]interface{}{
					"minReplicas": int64(1),
					"maxReplicas": int64(3),
				},
			},
		},
	}

	// Create a fake client with the sample objects
	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(aiModel, isvc).
		WithStatusSubresource(aiModel).
		Build()

	// Create the Reconciler
	r := &AIModelReconciler{
		Client: cl,
		Scheme: s,
	}

	// Mock request
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      aiModelName,
			Namespace: aiModelNamespace,
		},
	}

	// Check that finalizer exists before reconciliation
	beforeAIModel := &aimodelv1alpha1.AIModel{}
	err := cl.Get(context.Background(), types.NamespacedName{Name: aiModelName, Namespace: aiModelNamespace}, beforeAIModel)
	if err != nil {
		t.Fatalf("get aimodel before: (%v)", err)
	}
	if len(beforeAIModel.Finalizers) != 1 || beforeAIModel.Finalizers[0] != aiModelFinalizer {
		t.Errorf("expected finalizer to exist before reconcile, got %v", beforeAIModel.Finalizers)
	}

	// Test: Reconcile should delete InferenceService via finalizer
	t.Log("Test: Reconciling to delete InferenceService via finalizer")
	_, err = r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}

	// Check that InferenceService was deleted
	deletedIsvc := &unstructured.Unstructured{}
	deletedIsvc.SetGroupVersionKind(kserve.InferenceServiceGVK)
	err = cl.Get(context.Background(), types.NamespacedName{Name: aiModelName, Namespace: aiModelNamespace}, deletedIsvc)
	if err == nil {
		t.Error("expected InferenceService to be deleted, but it still exists")
	}

	// Check that finalizer was removed from AIModel
	// Note: After finalizer is removed, the object may be deleted by Kubernetes,
	// so we check if either the finalizer is removed OR the object is gone
	updatedAIModel := &aimodelv1alpha1.AIModel{}
	err = cl.Get(context.Background(), types.NamespacedName{Name: aiModelName, Namespace: aiModelNamespace}, updatedAIModel)
	if err == nil {
		// Object still exists, check that finalizer was removed
		if len(updatedAIModel.Finalizers) > 0 {
			t.Errorf("expected finalizers to be removed, got %v", updatedAIModel.Finalizers)
		}
	} else {
		// Object was deleted - this is also acceptable behavior
		t.Log("AIModel was deleted after finalizer removal (expected behavior)")
	}

	t.Log("Test passed: Controller correctly deletes InferenceService via finalizer")
}

func TestAIModelReconciler_DisabledModel(t *testing.T) {
	// Register operator types with the scheme
	s := setupScheme()

	// Define a disabled AIModel object
	aiModelName := "test-model"
	aiModelNamespace := "default"
	aiModel := &aimodelv1alpha1.AIModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aiModelName,
			Namespace: aiModelNamespace,
		},
		Spec: aimodelv1alpha1.AIModelSpec{
			ModelName: "mistral-7b",
			ModelID:   "mistral-7b-v0.1",
			S3Bucket:  "ai-models",
			S3Key:     "mistral-7b",
			Enabled:   false, // Disabled
		},
	}

	// Create a fake client with the sample object
	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(aiModel).
		WithStatusSubresource(aiModel).
		Build()

	// Create the Reconciler
	r := &AIModelReconciler{
		Client: cl,
		Scheme: s,
	}

	// Mock request
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      aiModelName,
			Namespace: aiModelNamespace,
		},
	}

	// Test: Reconcile should set phase to Disabled
	t.Log("Test: Reconciling disabled model")
	res, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}

	// Should NOT requeue
	if res.Requeue {
		t.Error("expected no requeue for disabled model")
	}

	// Check Status Update: Phase should be "Disabled"
	updatedAIModel := &aimodelv1alpha1.AIModel{}
	err = cl.Get(context.Background(), types.NamespacedName{Name: aiModelName, Namespace: aiModelNamespace}, updatedAIModel)
	if err != nil {
		t.Fatalf("get aimodel: (%v)", err)
	}
	if updatedAIModel.Status.Phase != "Disabled" {
		t.Errorf("expected phase 'Disabled', got '%s'", updatedAIModel.Status.Phase)
	}

	t.Log("Test passed: Controller correctly handles disabled models")
}
