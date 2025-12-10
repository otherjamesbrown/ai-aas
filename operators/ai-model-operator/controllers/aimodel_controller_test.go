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
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func setupScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)          // Core Kubernetes types (corev1, appsv1, batchv1, etc.)
	_ = aimodelv1alpha1.AddToScheme(s)         // AIModel CRD types
	return s
}

func TestAIModelReconciler_Reconcile(t *testing.T) {
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

	// --- Test 1: Initial Reconciliation ---
	// Note: In unit tests, the S3 artifact check will fail (no real AWS connection),
	// so the controller will set phase to "ArtifactMissing" instead of creating a job.
	// This is expected behavior and validates the S3 check logic.
	t.Log("Test 1: Initial Reconciliation")
	res, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}

	// Check result: Should NOT requeue (artifact missing stops reconciliation)
	if res.Requeue {
		t.Error("expected no requeue when artifact is missing")
	}

	// Check Status Update: Phase should be "ArtifactMissing" due to failed S3 check
	updatedAIModel := &aimodelv1alpha1.AIModel{}
	err = cl.Get(context.Background(), types.NamespacedName{Name: aiModelName, Namespace: aiModelNamespace}, updatedAIModel)
	if err != nil {
		t.Fatalf("get aimodel: (%v)", err)
	}
	if updatedAIModel.Status.Phase != "ArtifactMissing" {
		t.Errorf("expected phase 'ArtifactMissing', got '%s'", updatedAIModel.Status.Phase)
	}

	t.Log("Test 1 passed: Controller correctly handles missing S3 artifacts")

	// --- Test 2: Simulated Job Complete, Create Deployment ---
	// Since S3 check failed in Test 1, we manually create and complete the job
	// to test the deployment creation logic
	t.Log("Test 2: Simulating completed downloader job")

	// Create a completed job manually
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aiModelName + "-downloader",
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
	err = cl.Create(context.Background(), job)
	if err != nil {
		t.Fatalf("create job: (%v)", err)
	}

	// Update AIModel status to "Downloading" (as if the job was created earlier)
	updatedAIModel.Status.Phase = "Downloading"
	err = cl.Status().Update(context.Background(), updatedAIModel)
	if err != nil {
		t.Fatalf("update aimodel status: (%v)", err)
	}

	// Reconcile again - should detect completed job and create deployment
	res, err = r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}

	// Should requeue for Deployment creation/readiness
	if !res.Requeue {
		t.Error("expected requeue for deployment creation")
	}

	// Check if Deployment was created
	dep := &appsv1.Deployment{}
	depName := aiModelName + "-vllm"
	err = cl.Get(context.Background(), types.NamespacedName{Name: depName, Namespace: aiModelNamespace}, dep)
	if err != nil {
		t.Fatalf("get deployment: (%v)", err)
	}

	// Check Status Update: Phase should be "Deploying"
	err = cl.Get(context.Background(), types.NamespacedName{Name: aiModelName, Namespace: aiModelNamespace}, updatedAIModel)
	if err != nil {
		t.Fatalf("get aimodel: (%v)", err)
	}
	if updatedAIModel.Status.Phase != "Deploying" {
		t.Errorf("expected phase 'Deploying', got '%s'", updatedAIModel.Status.Phase)
	}

	t.Log("Test 2 passed: Controller correctly creates deployment after job completion")
}