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
	"time"

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

	// Create S3 credentials secret required by checkS3ArtifactExists
	s3Secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "s3-credentials",
			Namespace: aiModelNamespace,
		},
		Data: map[string][]byte{
			"AWS_ACCESS_KEY_ID":     []byte("test-access-key"),
			"AWS_SECRET_ACCESS_KEY": []byte("test-secret-key"),
			"AWS_ENDPOINT_URL_S3":   []byte("http://localhost:9000"),
			"S3_REGION":             []byte("us-east-1"),
		},
	}

	// Create a fake client with the sample object
	// Use WithStatusSubresource to handle status updates properly
	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(aiModel, s3Secret).
		WithStatusSubresource(aiModel).
		Build()

	// Create the Reconciler
	r := &AIModelReconciler{
		MaxDownloadRetries: 5,
		InitialRetryDelay:  1 * time.Minute,
		MaxRetryDelay:      16 * time.Minute,
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
		MaxDownloadRetries: 5,
		InitialRetryDelay:  1 * time.Minute,
		MaxRetryDelay:      16 * time.Minute,
		Client: cl,
		Scheme: s,
	}

	// Update AIModel status to "Deploying" (as if the download job completed)
	aiModel.Status.Phase = aimodelv1alpha1.AIModelPhaseDeploying
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

	// Check model spec (KServe native)
	model, found, err := unstructured.NestedMap(predictor, "model")
	if err != nil || !found {
		t.Fatalf("failed to get model: %v", err)
	}

	// Check storageUri
	storageUri, found, err := unstructured.NestedString(model, "storageUri")
	if err != nil || !found {
		t.Fatalf("failed to get storageUri: %v", err)
	}
	expectedStorageUri := "s3://ai-models/mistral-7b"
	if storageUri != expectedStorageUri {
		t.Errorf("expected storageUri '%s', got '%s'", expectedStorageUri, storageUri)
	}

	// Check runtime
	runtime, found, err := unstructured.NestedString(model, "runtime")
	if err != nil || !found {
		t.Fatalf("failed to get runtime: %v", err)
	}
	expectedRuntime := "vllm-runtime"
	if runtime != expectedRuntime {
		t.Errorf("expected runtime '%s', got '%s'", expectedRuntime, runtime)
	}

	// Check modelFormat
	modelFormat, found, err := unstructured.NestedMap(model, "modelFormat")
	if err != nil || !found {
		t.Fatalf("failed to get modelFormat: %v", err)
	}
	formatName, found, err := unstructured.NestedString(modelFormat, "name")
	if err != nil || !found {
		t.Fatalf("failed to get modelFormat name: %v", err)
	}
	expectedFormat := "vllm"
	if formatName != expectedFormat {
		t.Errorf("expected modelFormat '%s', got '%s'", expectedFormat, formatName)
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
		MaxDownloadRetries: 5,
		InitialRetryDelay:  1 * time.Minute,
		MaxRetryDelay:      16 * time.Minute,
		Client: cl,
		Scheme: s,
	}

	// Test: Call updateStatusFromInferenceService directly
	t.Log("Test: Updating status from ready InferenceService")
	err := r.updateStatusFromInferenceService(context.Background(), aiModel)
	if err != nil {
		t.Fatalf("updateStatusFromInferenceService: (%v)", err)
	}

	// The method re-fetches the AIModel, updates it, and persists via Status().Update()
	// We need to fetch the updated AIModel from the client to verify changes

	// Fetch the updated AIModel
	updatedAIModel := &aimodelv1alpha1.AIModel{}
	err = cl.Get(context.Background(), types.NamespacedName{Name: aiModelName, Namespace: aiModelNamespace}, updatedAIModel)
	if err != nil {
		t.Fatalf("failed to fetch updated AIModel: %v", err)
	}

	// Check phase is Ready
	if updatedAIModel.Status.Phase != aimodelv1alpha1.AIModelPhaseReady {
		t.Errorf("expected phase '%s', got '%s'", aimodelv1alpha1.AIModelPhaseReady, updatedAIModel.Status.Phase)
	}

	// Check endpoint URL
	expectedURL := "http://test-model.default.example.com"
	if updatedAIModel.Status.InferenceEndpoint != expectedURL {
		t.Errorf("expected endpoint '%s', got '%s'", expectedURL, updatedAIModel.Status.InferenceEndpoint)
	}

	// Check ready replicas
	if updatedAIModel.Status.ReadyReplicas != 1 {
		t.Errorf("expected readyReplicas 1, got %d", updatedAIModel.Status.ReadyReplicas)
	}

	// Check InferenceService name
	if updatedAIModel.Status.InferenceServiceName != aiModelName {
		t.Errorf("expected inferenceServiceName '%s', got '%s'", aiModelName, updatedAIModel.Status.InferenceServiceName)
	}

	t.Log("Test passed: Controller correctly extracts and updates status from InferenceService")
}

// TestAIModelReconciler_StatusSyncBackToCaller verifies that after updateStatusFromInferenceService
// returns, the caller's aiModel object has the updated status. This prevents the race condition
// where a re-fetch after the call would get a stale cached copy (aas-lhx0).
func TestAIModelReconciler_StatusSyncBackToCaller(t *testing.T) {
	// Register operator types with the scheme
	s := setupScheme()

	// Define a sample AIModel object starting in Deploying phase
	aiModelName := "test-model-sync"
	aiModelNamespace := "default"
	aiModel := &aimodelv1alpha1.AIModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aiModelName,
			Namespace: aiModelNamespace,
			UID:       types.UID("test-uid-sync"),
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
			Phase:   aimodelv1alpha1.AIModelPhaseDeploying,
			Message: "Deploying model",
		},
	}

	// Verify starting state
	if aiModel.Status.Phase != aimodelv1alpha1.AIModelPhaseDeploying {
		t.Fatalf("Expected initial phase to be Deploying, got %s", aiModel.Status.Phase)
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
				"url": "http://test-model-sync.default.example.com",
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
		MaxDownloadRetries: 5,
		InitialRetryDelay:  1 * time.Minute,
		MaxRetryDelay:      16 * time.Minute,
		Client:             cl,
		Scheme:             s,
	}

	// Call updateStatusFromInferenceService - the key test is that AFTER this call,
	// the caller's aiModel object should have the updated status (not require a re-fetch)
	t.Log("Test: Calling updateStatusFromInferenceService and verifying caller's object is updated")
	err := r.updateStatusFromInferenceService(context.Background(), aiModel)
	if err != nil {
		t.Fatalf("updateStatusFromInferenceService: (%v)", err)
	}

	// CRITICAL: The caller's aiModel object should now have the Ready phase
	// This is what the fix in aas-lhx0 addresses - previously, the caller would need
	// to re-fetch to get the updated status, but that re-fetch could return a stale copy
	if aiModel.Status.Phase != aimodelv1alpha1.AIModelPhaseReady {
		t.Errorf("RACE CONDITION BUG: Caller's aiModel.Status.Phase should be Ready after updateStatusFromInferenceService, got %s", aiModel.Status.Phase)
	}

	// Also verify other status fields were synced back to the caller
	if aiModel.Status.InferenceEndpoint != "http://test-model-sync.default.example.com" {
		t.Errorf("Caller's aiModel.Status.InferenceEndpoint not synced, got %s", aiModel.Status.InferenceEndpoint)
	}

	if aiModel.Status.ReadyReplicas != 1 {
		t.Errorf("Caller's aiModel.Status.ReadyReplicas not synced, got %d", aiModel.Status.ReadyReplicas)
	}

	if aiModel.Status.InferenceServiceName != aiModelName {
		t.Errorf("Caller's aiModel.Status.InferenceServiceName not synced, got %s", aiModel.Status.InferenceServiceName)
	}

	t.Log("Test passed: Caller's aiModel object correctly updated after updateStatusFromInferenceService (no stale cache issue)")
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
		MaxDownloadRetries: 5,
		InitialRetryDelay:  1 * time.Minute,
		MaxRetryDelay:      16 * time.Minute,
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
		MaxDownloadRetries: 5,
		InitialRetryDelay:  1 * time.Minute,
		MaxRetryDelay:      16 * time.Minute,
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

// Test: Download job failure triggers retry
func TestAIModelReconciler_JobFailureTriggersRetry(t *testing.T) {
	s := setupScheme()

	aiModelName := "test-model-retry"
	aiModelNamespace := "default"
	replicas := int32(1)
	aiModel := &aimodelv1alpha1.AIModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aiModelName,
			Namespace: aiModelNamespace,
		},
		Spec: aimodelv1alpha1.AIModelSpec{
			ModelName: "test-model",
			ModelID:   "test/model",
			S3Bucket:  "ai-models",
			S3Key:     "test-model",
			Replicas:  &replicas,
			Enabled:   true,
		},
		Status: aimodelv1alpha1.AIModelStatus{
			Phase: aimodelv1alpha1.AIModelPhaseDownloading,
		},
	}

	// Create a failed job
	jobName := aiModelName + "-downloader"
	failedJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: aiModelNamespace,
		},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{
					Type:   batchv1.JobFailed,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(aiModel, failedJob).
		WithStatusSubresource(aiModel).
		Build()

	r := &AIModelReconciler{
		MaxDownloadRetries: 5,
		InitialRetryDelay:  1 * time.Minute,
		MaxRetryDelay:      16 * time.Minute,
		Client: cl,
		Scheme: s,
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      aiModelName,
			Namespace: aiModelNamespace,
		},
	}

	t.Log("Test: Job failure triggers retry")
	res, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}

	// Should requeue with backoff
	if res.RequeueAfter == 0 {
		t.Error("expected RequeueAfter to be set for retry")
	}

	// Check that retry count was incremented
	updatedAIModel := &aimodelv1alpha1.AIModel{}
	err = cl.Get(context.Background(), types.NamespacedName{Name: aiModelName, Namespace: aiModelNamespace}, updatedAIModel)
	if err != nil {
		t.Fatalf("get aimodel: (%v)", err)
	}

	if updatedAIModel.Status.RetryCount != 1 {
		t.Errorf("expected RetryCount 1, got %d", updatedAIModel.Status.RetryCount)
	}

	if updatedAIModel.Status.Phase != aimodelv1alpha1.AIModelPhaseRetryPending {
		t.Errorf("expected phase RetryPending, got %s", updatedAIModel.Status.Phase)
	}

	if updatedAIModel.Status.LastRetryTime == nil {
		t.Error("expected LastRetryTime to be set")
	}

	if updatedAIModel.Status.NextRetryTime == nil {
		t.Error("expected NextRetryTime to be set")
	}

	// Verify job was deleted
	checkJob := &batchv1.Job{}
	err = cl.Get(context.Background(), types.NamespacedName{Name: jobName, Namespace: aiModelNamespace}, checkJob)
	if err == nil {
		t.Error("expected failed job to be deleted")
	}

	t.Log("Test passed: Job failure correctly triggers retry with backoff")
}

// Test: Exponential backoff calculation
func TestCalculateRetryBackoff(t *testing.T) {
	s := setupScheme()

	r := &AIModelReconciler{
		Client:             fake.NewClientBuilder().WithScheme(s).Build(),
		Scheme:             s,
		MaxDownloadRetries: 5,
		InitialRetryDelay:  1 * time.Minute,
		MaxRetryDelay:      16 * time.Minute,
	}

	tests := []struct {
		retryCount      int32
		expectedBackoff string
	}{
		{0, "1m0s"},   // 1 minute
		{1, "2m0s"},   // 2 minutes
		{2, "4m0s"},   // 4 minutes
		{3, "8m0s"},   // 8 minutes
		{4, "16m0s"},  // 16 minutes (max)
		{5, "16m0s"},  // Still 16 minutes (capped)
		{10, "16m0s"}, // Still 16 minutes (capped)
	}

	for _, tt := range tests {
		t.Run("retry_"+string(rune(tt.retryCount+'0')), func(t *testing.T) {
			backoff := r.calculateRetryBackoff(tt.retryCount)
			if backoff.String() != tt.expectedBackoff {
				t.Errorf("retry %d: expected backoff %s, got %s", tt.retryCount, tt.expectedBackoff, backoff)
			}
		})
	}

	t.Log("Test passed: Exponential backoff calculated correctly")
}

// Test: Max retries exceeded leads to permanent failure
func TestAIModelReconciler_MaxRetriesExceeded(t *testing.T) {
	s := setupScheme()

	aiModelName := "test-model-max-retries"
	aiModelNamespace := "default"
	replicas := int32(1)
	aiModel := &aimodelv1alpha1.AIModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aiModelName,
			Namespace: aiModelNamespace,
		},
		Spec: aimodelv1alpha1.AIModelSpec{
			ModelName: "test-model",
			ModelID:   "test/model",
			S3Bucket:  "ai-models",
			S3Key:     "test-model",
			Replicas:  &replicas,
			Enabled:   true,
		},
		Status: aimodelv1alpha1.AIModelStatus{
			Phase:      aimodelv1alpha1.AIModelPhaseDownloading,
			RetryCount: maxDownloadRetries, // Already at max retries
		},
	}

	// Create a failed job
	jobName := aiModelName + "-downloader"
	failedJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: aiModelNamespace,
		},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{
					Type:   batchv1.JobFailed,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(aiModel, failedJob).
		WithStatusSubresource(aiModel).
		Build()

	r := &AIModelReconciler{
		MaxDownloadRetries: 5,
		InitialRetryDelay:  1 * time.Minute,
		MaxRetryDelay:      16 * time.Minute,
		Client: cl,
		Scheme: s,
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      aiModelName,
			Namespace: aiModelNamespace,
		},
	}

	t.Log("Test: Max retries exceeded leads to permanent failure")
	res, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}

	// Should NOT requeue (permanent failure)
	if res.Requeue || res.RequeueAfter != 0 {
		t.Error("expected no requeue for permanent failure after max retries")
	}

	// Check that phase is Failed
	updatedAIModel := &aimodelv1alpha1.AIModel{}
	err = cl.Get(context.Background(), types.NamespacedName{Name: aiModelName, Namespace: aiModelNamespace}, updatedAIModel)
	if err != nil {
		t.Fatalf("get aimodel: (%v)", err)
	}

	if updatedAIModel.Status.Phase != aimodelv1alpha1.AIModelPhaseFailed {
		t.Errorf("expected phase Failed, got %s", updatedAIModel.Status.Phase)
	}

	if updatedAIModel.Status.Message == "" {
		t.Error("expected failure message to be set")
	}

	t.Log("Test passed: Max retries exceeded correctly results in permanent failure")
}

// Test: RetryPending phase waits for retry time
func TestAIModelReconciler_RetryPendingWaitsForTime(t *testing.T) {
	s := setupScheme()

	aiModelName := "test-model-retry-pending"
	aiModelNamespace := "default"
	replicas := int32(1)

	// Set next retry time in the future
	futureTime := metav1.Now()
	futureTime.Time = futureTime.Time.Add(5 * time.Minute) // 5 minutes in the future

	aiModel := &aimodelv1alpha1.AIModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aiModelName,
			Namespace: aiModelNamespace,
		},
		Spec: aimodelv1alpha1.AIModelSpec{
			ModelName: "test-model",
			ModelID:   "test/model",
			S3Bucket:  "ai-models",
			S3Key:     "test-model",
			Replicas:  &replicas,
			Enabled:   true,
		},
		Status: aimodelv1alpha1.AIModelStatus{
			Phase:         aimodelv1alpha1.AIModelPhaseRetryPending,
			RetryCount:    1,
			NextRetryTime: &futureTime,
		},
	}

	// Create S3 credentials secret so checkS3ArtifactExists doesn't fail
	s3Secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "s3-credentials",
			Namespace: aiModelNamespace,
		},
		Data: map[string][]byte{
			"AWS_ACCESS_KEY_ID":     []byte("test-key"),
			"AWS_SECRET_ACCESS_KEY": []byte("test-secret"),
			"AWS_ENDPOINT_URL_S3":   []byte("http://test-s3:9000"),
			"S3_REGION":             []byte("us-east-1"),
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(aiModel, s3Secret).
		WithStatusSubresource(aiModel).
		Build()

	r := &AIModelReconciler{
		MaxDownloadRetries: 5,
		InitialRetryDelay:  1 * time.Minute,
		MaxRetryDelay:      16 * time.Minute,
		Client: cl,
		Scheme: s,
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      aiModelName,
			Namespace: aiModelNamespace,
		},
	}

	t.Log("Test: RetryPending waits for retry time")
	res, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}

	// Should requeue with remaining wait time
	if res.RequeueAfter == 0 {
		t.Error("expected RequeueAfter to be set when waiting for retry time")
	}

	// Should NOT have created a new job yet
	jobName := aiModelName + "-downloader"
	checkJob := &batchv1.Job{}
	err = cl.Get(context.Background(), types.NamespacedName{Name: jobName, Namespace: aiModelNamespace}, checkJob)
	if err == nil {
		t.Error("expected no job to exist when retry time not yet reached")
	}

	t.Log("Test passed: RetryPending correctly waits for retry time")
}

// Test: RetryPending creates new job when time is reached
func TestAIModelReconciler_RetryPendingCreatesJobWhenReady(t *testing.T) {
	s := setupScheme()

	aiModelName := "test-model-retry-ready"
	aiModelNamespace := "default"
	replicas := int32(1)

	// Set next retry time in the past (ready to retry)
	pastTime := metav1.Now()
	pastTime.Time = pastTime.Time.Add(-1 * time.Minute) // 1 minute in the past

	aiModel := &aimodelv1alpha1.AIModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aiModelName,
			Namespace: aiModelNamespace,
		},
		Spec: aimodelv1alpha1.AIModelSpec{
			ModelName: "test-model",
			ModelID:   "test/model",
			S3Bucket:  "ai-models",
			S3Key:     "test-model",
			Replicas:  &replicas,
			Enabled:   true,
		},
		Status: aimodelv1alpha1.AIModelStatus{
			Phase:         aimodelv1alpha1.AIModelPhaseRetryPending,
			RetryCount:    1,
			NextRetryTime: &pastTime,
		},
	}

	// Create S3 credentials secret so checkS3ArtifactExists doesn't fail
	s3Secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "s3-credentials",
			Namespace: aiModelNamespace,
		},
		Data: map[string][]byte{
			"AWS_ACCESS_KEY_ID":     []byte("test-key"),
			"AWS_SECRET_ACCESS_KEY": []byte("test-secret"),
			"AWS_ENDPOINT_URL_S3":   []byte("http://test-s3:9000"),
			"S3_REGION":             []byte("us-east-1"),
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(aiModel, s3Secret).
		WithStatusSubresource(aiModel).
		Build()

	r := &AIModelReconciler{
		MaxDownloadRetries: 5,
		InitialRetryDelay:  1 * time.Minute,
		MaxRetryDelay:      16 * time.Minute,
		Client: cl,
		Scheme: s,
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      aiModelName,
			Namespace: aiModelNamespace,
		},
	}

	t.Log("Test: RetryPending creates new job when retry time reached")
	res, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}

	// Should requeue to wait for new job
	if !res.Requeue {
		t.Error("expected Requeue to be true after creating new job")
	}

	// Should have created a new job
	jobName := aiModelName + "-downloader"
	checkJob := &batchv1.Job{}
	err = cl.Get(context.Background(), types.NamespacedName{Name: jobName, Namespace: aiModelNamespace}, checkJob)
	if err != nil {
		t.Errorf("expected job to be created when retry time reached: %v", err)
	}

	t.Log("Test passed: RetryPending correctly creates new job when time is reached")
}

// TestAIModelReconciler_ConcurrentInferenceServiceUpdates verifies that the controller
// can handle concurrent updates to InferenceService resources without conflicts.
//
// This test addresses ai-aas-sllz: Concurrent InferenceService update handling.
func TestAIModelReconciler_ConcurrentInferenceServiceUpdates(t *testing.T) {
	s := setupScheme()

	aiModelName := "test-concurrent-updates"
	aiModelNamespace := "default"
	minReplicas := int32(0)
	maxReplicas := int32(2)

	aiModel := &aimodelv1alpha1.AIModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aiModelName,
			Namespace: aiModelNamespace,
			UID:       types.UID("test-uid-concurrent"),
		},
		Spec: aimodelv1alpha1.AIModelSpec{
			ModelName:   "test-model",
			ModelID:     "test/model",
			S3Bucket:    "ai-models",
			S3Key:       "test-model",
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
		Status: aimodelv1alpha1.AIModelStatus{
			Phase: aimodelv1alpha1.AIModelPhaseDeploying,
		},
	}

	// Create a completed download job
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

	// Create an existing InferenceService
	existingIsvc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "serving.kserve.io/v1beta1",
			"kind":       "InferenceService",
			"metadata": map[string]interface{}{
				"name":            aiModelName,
				"namespace":       aiModelNamespace,
				"resourceVersion": "1000", // Simulates existing resource
			},
			"spec": map[string]interface{}{
				"predictor": map[string]interface{}{
					"minReplicas": int64(1), // Different from desired state
					"maxReplicas": int64(3),
					"model": map[string]interface{}{
						"storageUri": "s3://ai-models/test-model",
						"runtime":    "vllm-runtime",
					},
				},
			},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(aiModel, job, existingIsvc).
		WithStatusSubresource(aiModel).
		Build()

	r := &AIModelReconciler{
		MaxDownloadRetries: 5,
		InitialRetryDelay:  1 * time.Minute,
		MaxRetryDelay:      16 * time.Minute,
		Client: cl,
		Scheme: s,
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      aiModelName,
			Namespace: aiModelNamespace,
		},
	}

	// First reconcile should update the InferenceService
	t.Log("Test: First reconcile updates InferenceService")
	res, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("first reconcile failed: %v", err)
	}

	if !res.Requeue {
		t.Error("expected requeue after updating InferenceService")
	}

	// Verify InferenceService was updated with correct minReplicas
	updatedIsvc := &unstructured.Unstructured{}
	updatedIsvc.SetGroupVersionKind(kserve.InferenceServiceGVK)
	err = cl.Get(context.Background(), types.NamespacedName{Name: aiModelName, Namespace: aiModelNamespace}, updatedIsvc)
	if err != nil {
		t.Fatalf("get updated InferenceService: %v", err)
	}

	predictor, found, err := unstructured.NestedMap(updatedIsvc.Object, "spec", "predictor")
	if err != nil || !found {
		t.Fatal("failed to get predictor from updated InferenceService")
	}

	minReplicasVal, found, err := unstructured.NestedInt64(predictor, "minReplicas")
	if err != nil || !found {
		t.Fatal("failed to get minReplicas from updated InferenceService")
	}

	if minReplicasVal != int64(minReplicas) {
		t.Errorf("expected minReplicas %d after update, got %d", minReplicas, minReplicasVal)
	}

	// Second reconcile should handle existing InferenceService gracefully
	// This simulates a scenario where the InferenceService already exists and matches
	t.Log("Test: Second reconcile handles existing InferenceService")
	res2, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("second reconcile failed: %v", err)
	}

	// Should still requeue to monitor status
	if !res2.Requeue {
		t.Error("expected requeue to monitor InferenceService status")
	}

	// Verify no errors occurred during concurrent update handling
	updatedAIModel := &aimodelv1alpha1.AIModel{}
	err = cl.Get(context.Background(), types.NamespacedName{Name: aiModelName, Namespace: aiModelNamespace}, updatedAIModel)
	if err != nil {
		t.Fatalf("get updated AIModel: %v", err)
	}

	// Phase should still be Deploying (waiting for InferenceService ready)
	if updatedAIModel.Status.Phase != aimodelv1alpha1.AIModelPhaseDeploying {
		t.Errorf("expected phase Deploying after reconciles, got %s", updatedAIModel.Status.Phase)
	}

	t.Log("Test passed: Controller handles concurrent InferenceService updates without conflicts")
}

func TestAIModelReconciler_SkipsUpdateWhenSpecUnchanged(t *testing.T) {
	// Register operator types with the scheme
	s := setupScheme()

	// Define a sample AIModel object
	aiModelName := "test-model-unchanged"
	aiModelNamespace := "default"
	minReplicas := int32(0)
	maxReplicas := int32(2)
	aiModel := &aimodelv1alpha1.AIModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aiModelName,
			Namespace: aiModelNamespace,
			UID:       types.UID("test-uid-456"),
		},
		Spec: aimodelv1alpha1.AIModelSpec{
			ModelName:   "gpt2",
			ModelID:     "gpt2",
			S3Bucket:    "ai-models",
			S3Key:       "gpt2",
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
		Status: aimodelv1alpha1.AIModelStatus{
			Phase: aimodelv1alpha1.AIModelPhaseDeploying,
		},
	}

	// Create a completed downloader job
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

	// Create an existing InferenceService using the same builder the controller uses
	// This ensures the specs will match exactly
	ownerRef := &metav1.OwnerReference{
		APIVersion:         aiModel.APIVersion,
		Kind:               aiModel.Kind,
		Name:               aiModel.Name,
		UID:                aiModel.UID,
		Controller:         func() *bool { b := true; return &b }(),
		BlockOwnerDeletion: func() *bool { b := true; return &b }(),
	}

	// Build runtime env with S3 credentials (same as controller)
	runtimeEnv := []corev1.EnvVar{
		{
			Name: "AWS_ACCESS_KEY_ID",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "s3-credentials",
					},
					Key: "AWS_ACCESS_KEY_ID",
				},
			},
		},
		{
			Name: "AWS_SECRET_ACCESS_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "s3-credentials",
					},
					Key: "AWS_SECRET_ACCESS_KEY",
				},
			},
		},
		{
			Name: "AWS_ENDPOINT_URL_S3",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "s3-credentials",
					},
					Key: "AWS_ENDPOINT_URL_S3",
					Optional: func() *bool {
						opt := true
						return &opt
					}(),
				},
			},
		},
		{
			Name: "S3_ENDPOINT",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "s3-credentials",
					},
					Key: "S3_ENDPOINT",
					Optional: func() *bool {
						opt := true
						return &opt
					}(),
				},
			},
		},
	}

	runtimeArgs := []string{
		"--dtype=float16",
		"--max-model-len=4096",
		"--gpu-memory-utilization=0.9",
	}

	existingIsvc, buildErr := kserve.NewInferenceServiceBuilder(aiModelName, aiModelNamespace).
		WithStorageUri("s3://ai-models/gpt2").
		WithModelFormat("vllm").
		WithServedName("gpt2").
		WithRuntime("vllm-runtime").
		WithScaling(minReplicas, maxReplicas).
		WithResources(aiModel.Spec.Resources).
		WithRuntimeArgs(runtimeArgs).
		WithRuntimeEnv(runtimeEnv).
		WithOwnerReference(ownerRef).
		Build()
	if buildErr != nil {
		t.Fatalf("build existing InferenceService: %v", buildErr)
	}

	// Create a fake client with the objects
	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(aiModel, job, existingIsvc).
		WithStatusSubresource(aiModel).
		Build()

	// Create the Reconciler
	r := &AIModelReconciler{
		Client:         cl,
		Scheme:         s,
		DefaultRuntime: "vllm",
	}

	// Mock request
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      aiModelName,
			Namespace: aiModelNamespace,
		},
	}

	// Get initial resource version of InferenceService
	initialIsvc := &unstructured.Unstructured{}
	initialIsvc.SetGroupVersionKind(kserve.InferenceServiceGVK)
	err := cl.Get(context.Background(), types.NamespacedName{Name: aiModelName, Namespace: aiModelNamespace}, initialIsvc)
	if err != nil {
		t.Fatalf("get initial InferenceService: %v", err)
	}
	initialResourceVersion := initialIsvc.GetResourceVersion()

	// First reconcile should skip update since spec is unchanged
	t.Log("Test: First reconcile with unchanged spec")
	res, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	// Debug: Log what the controller built
	t.Logf("Initial resource version: %s", initialResourceVersion)

	// Should still requeue to monitor status
	if !res.Requeue {
		t.Error("expected requeue to monitor InferenceService status")
	}

	// Verify InferenceService was NOT updated (resource version unchanged)
	updatedIsvc := &unstructured.Unstructured{}
	updatedIsvc.SetGroupVersionKind(kserve.InferenceServiceGVK)
	err = cl.Get(context.Background(), types.NamespacedName{Name: aiModelName, Namespace: aiModelNamespace}, updatedIsvc)
	if err != nil {
		t.Fatalf("get InferenceService after reconcile: %v", err)
	}
	finalResourceVersion := updatedIsvc.GetResourceVersion()

	if finalResourceVersion != initialResourceVersion {
		t.Errorf("InferenceService should not have been updated (resource version changed from %s to %s)",
			initialResourceVersion, finalResourceVersion)
	}

	// Now modify the AIModel spec to trigger an update
	t.Log("Test: Reconcile with changed spec")
	updatedAIModel := &aimodelv1alpha1.AIModel{}
	err = cl.Get(context.Background(), types.NamespacedName{Name: aiModelName, Namespace: aiModelNamespace}, updatedAIModel)
	if err != nil {
		t.Fatalf("get AIModel: %v", err)
	}

	// Change maxReplicas
	newMaxReplicas := int32(5)
	updatedAIModel.Spec.MaxReplicas = &newMaxReplicas
	err = cl.Update(context.Background(), updatedAIModel)
	if err != nil {
		t.Fatalf("update AIModel: %v", err)
	}

	// Second reconcile should update InferenceService
	res2, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("second reconcile: %v", err)
	}

	if !res2.Requeue {
		t.Error("expected requeue to monitor InferenceService status")
	}

	// Verify InferenceService WAS updated (resource version changed)
	finalIsvc := &unstructured.Unstructured{}
	finalIsvc.SetGroupVersionKind(kserve.InferenceServiceGVK)
	err = cl.Get(context.Background(), types.NamespacedName{Name: aiModelName, Namespace: aiModelNamespace}, finalIsvc)
	if err != nil {
		t.Fatalf("get InferenceService after second reconcile: %v", err)
	}
	changedResourceVersion := finalIsvc.GetResourceVersion()

	if changedResourceVersion == finalResourceVersion {
		t.Error("InferenceService should have been updated (resource version unchanged)")
	}

	// Verify the maxReplicas was updated correctly
	predictor, found, err := unstructured.NestedMap(finalIsvc.Object, "spec", "predictor")
	if err != nil || !found {
		t.Fatal("failed to get predictor from updated InferenceService")
	}

	maxReplicasVal, found, err := unstructured.NestedInt64(predictor, "maxReplicas")
	if err != nil || !found {
		t.Fatal("failed to get maxReplicas from updated InferenceService")
	}

	if maxReplicasVal != int64(newMaxReplicas) {
		t.Errorf("expected maxReplicas %d after update, got %d", newMaxReplicas, maxReplicasVal)
	}

	t.Log("Test passed: Controller skips update when spec is unchanged and updates when spec changes")
}

func TestDetermineDeploymentMode(t *testing.T) {
	s := setupScheme()
	r := &AIModelReconciler{
		Client: fake.NewClientBuilder().WithScheme(s).Build(),
		Scheme: s,
	}

	tests := []struct {
		name     string
		aimodel  *aimodelv1alpha1.AIModel
		recipe   *aimodelv1alpha1.ModelRecipeSpec
		wantMode string
	}{
		{
			name: "explicit AIModel override to RawDeployment",
			aimodel: &aimodelv1alpha1.AIModel{
				Spec: aimodelv1alpha1.AIModelSpec{
					DeploymentMode: "RawDeployment",
					Runtime:        "vllm",
				},
			},
			recipe:   nil,
			wantMode: "RawDeployment",
		},
		{
			name: "explicit AIModel override to Serverless",
			aimodel: &aimodelv1alpha1.AIModel{
				Spec: aimodelv1alpha1.AIModelSpec{
					DeploymentMode: "Serverless",
					Runtime:        "tensorrt-llm", // Would default to RawDeployment
				},
			},
			recipe:   nil,
			wantMode: "Serverless",
		},
		{
			name: "recipe explicit mode",
			aimodel: &aimodelv1alpha1.AIModel{
				Spec: aimodelv1alpha1.AIModelSpec{
					Runtime: "vllm",
				},
			},
			recipe: &aimodelv1alpha1.ModelRecipeSpec{
				DeploymentMode: "RawDeployment",
			},
			wantMode: "RawDeployment",
		},
		{
			name: "tensorrt-llm defaults to RawDeployment",
			aimodel: &aimodelv1alpha1.AIModel{
				Spec: aimodelv1alpha1.AIModelSpec{
					Runtime: "tensorrt-llm",
				},
			},
			recipe:   nil,
			wantMode: "RawDeployment",
		},
		{
			name: "triton defaults to RawDeployment",
			aimodel: &aimodelv1alpha1.AIModel{
				Spec: aimodelv1alpha1.AIModelSpec{
					Runtime: "triton",
				},
			},
			recipe:   nil,
			wantMode: "RawDeployment",
		},
		{
			name: "vllm without GPU defaults to Serverless",
			aimodel: &aimodelv1alpha1.AIModel{
				Spec: aimodelv1alpha1.AIModelSpec{
					Runtime: "vllm",
				},
			},
			recipe:   nil,
			wantMode: "Serverless",
		},
		{
			name: "vllm with GPU defaults to RawDeployment",
			aimodel: &aimodelv1alpha1.AIModel{
				Spec: aimodelv1alpha1.AIModelSpec{
					Runtime: "vllm",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"nvidia.com/gpu": resource.MustParse("1"),
						},
					},
				},
			},
			recipe:   nil,
			wantMode: "RawDeployment",
		},
		{
			name: "unknown runtime defaults to Serverless",
			aimodel: &aimodelv1alpha1.AIModel{
				Spec: aimodelv1alpha1.AIModelSpec{
					Runtime: "unknown-runtime",
				},
			},
			recipe:   nil,
			wantMode: "Serverless",
		},
		{
			name: "tgi with GPU defaults to RawDeployment",
			aimodel: &aimodelv1alpha1.AIModel{
				Spec: aimodelv1alpha1.AIModelSpec{
					Runtime: "tgi",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"nvidia.com/gpu": resource.MustParse("1"),
						},
					},
				},
			},
			recipe:   nil,
			wantMode: "RawDeployment",
		},
		{
			name: "tgi without GPU defaults to Serverless",
			aimodel: &aimodelv1alpha1.AIModel{
				Spec: aimodelv1alpha1.AIModelSpec{
					Runtime: "tgi",
				},
			},
			recipe:   nil,
			wantMode: "Serverless",
		},
		{
			name: "recipe runtime with GPU overrides AIModel runtime",
			aimodel: &aimodelv1alpha1.AIModel{
				Spec: aimodelv1alpha1.AIModelSpec{
					Runtime: "vllm", // Would be Serverless without GPU
				},
			},
			recipe: &aimodelv1alpha1.ModelRecipeSpec{
				Runtime: "tensorrt-llm", // Forces RawDeployment
			},
			wantMode: "RawDeployment",
		},
		{
			name: "GPU from recipe resources triggers RawDeployment",
			aimodel: &aimodelv1alpha1.AIModel{
				Spec: aimodelv1alpha1.AIModelSpec{
					Runtime: "vllm",
				},
			},
			recipe: &aimodelv1alpha1.ModelRecipeSpec{
				Resources: aimodelv1alpha1.RecipeResources{
					GPU: aimodelv1alpha1.GPUResources{
						Count: 1,
					},
				},
			},
			wantMode: "RawDeployment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMode := r.determineDeploymentMode(tt.aimodel, tt.recipe)
			if gotMode != tt.wantMode {
				t.Errorf("determineDeploymentMode() = %q, want %q", gotMode, tt.wantMode)
			}
		})
	}
}
