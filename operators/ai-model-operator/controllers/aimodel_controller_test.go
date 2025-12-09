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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	aimodelv1alpha1 "github.com/ai-aas/ai-model-operator/api/v1alpha1"
)

var _ = Describe("AIModel controller", func() {
	const (
		AIModelName = "test-aimodel"
		AIModelNamespace = "default"
		ModelID = "test/model"
		Image = "test-image:latest"

		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	// Mock for IsModelArtifactsReady
	var mockIsModelArtifactsReady func(ctx context.Context, aiModel *aimodelv1alpha1.AIModel) (bool, error)

	BeforeEach(func() {
		// Reset the mock before each test
		mockIsModelArtifactsReady = func(ctx context.Context, aiModel *aimodelv1alpha1.AIModel) (bool, error) {
			return true, nil // Default to true, override in specific tests
		}
		// Override the actual function in the reconciler for testing
		reconciler.IsModelArtifactsReady = mockIsModelArtifactsReady
	})

	Context("When reconciling an AIModel", func() {
		It("should successfully reconcile and create Deployment/Service when artifacts are ready", func() {
			ctx := context.Background()

			By("Creating a new AIModel with artifacts ready")
			aiModel := &aimodelv1alpha1.AIModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      AIModelName,
					Namespace: AIModelNamespace,
				},
				Spec: aimodelv1alpha1.AIModelSpec{
					ModelID: ModelID,
					Image:   Image,
					Enabled: true,
				},
			}
			Expect(k8sClient.Create(ctx, aiModel)).Should(Succeed())

			By("Checking if Deployment is created")
			lookUpKeyDeployment := types.NamespacedName{Name: AIModelName + "-vllm-deployment", Namespace: AIModelNamespace}
			createdDeployment := &appsv1.Deployment{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, lookUpKeyDeployment, createdDeployment)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdDeployment.Spec.Template.Spec.Containers[0].Image).Should(Equal(Image))
			Expect(createdDeployment.Spec.Template.Spec.Containers[0].Env[0].Value).Should(Equal(ModelID))

			By("Checking if Service is created")
			lookUpKeyService := types.NamespacedName{Name: AIModelName + "-vllm-service", Namespace: AIModelNamespace}
			createdService := &corev1.Service{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, lookUpKeyService, createdService)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			
			By("Checking AIModel status is updated to Ready and InferenceEndpoint is set")
			Eventually(func() bool {
				_ = k8sClient.Get(ctx, types.NamespacedName{Name: AIModelName, Namespace: AIModelNamespace}, aiModel)
				return aiModel.Status.Phase == "Ready" &&
					aiModel.Status.InferenceEndpoint == fmt.Sprintf("http://%s.%s.svc.cluster.local", AIModelName+"-vllm-service", AIModelNamespace)
			}, timeout, interval).Should(BeTrue())

			By("Deleting the AIModel resource")
			Expect(k8sClient.Delete(ctx, aiModel)).Should(Succeed())

			By("Checking if Deployment is deleted")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, lookUpKeyDeployment, createdDeployment)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

			By("Checking if Service is deleted")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, lookUpKeyService, createdService)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})

		It("should update existing Deployment and Service", func() {
			ctx := context.Background()
			updatedReplicas := int32(2)
			updatedImage := "new-test-image:2.0"

			By("Creating a new AIModel for update test")
			aiModel := &aimodelv1alpha1.AIModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      AIModelName + "-update",
					Namespace: AIModelNamespace,
				},
				Spec: aimodelv1alpha1.AIModelSpec{
					ModelID: ModelID,
					Image:   Image,
					Enabled: true,
				},
			}
			Expect(k8sClient.Create(ctx, aiModel)).Should(Succeed())
			
			// Ensure it reaches Ready state first and endpoint is set
			Eventually(func() bool {
				_ = k8sClient.Get(ctx, types.NamespacedName{Name: aiModel.Name, Namespace: AIModelNamespace}, aiModel)
				return aiModel.Status.Phase == "Ready" &&
					aiModel.Status.InferenceEndpoint == fmt.Sprintf("http://%s.%s.svc.cluster.local", aiModel.Name+"-vllm-service", AIModelNamespace)
			}, timeout, interval).Should(BeTrue())

			By("Updating the AIModel replicas and image")
			aiModel.Spec.Replicas = &updatedReplicas
			aiModel.Spec.Image = updatedImage
			Expect(k8sClient.Update(ctx, aiModel)).Should(Succeed())

			By("Checking if Deployment is updated")
			lookUpKeyDeployment := types.NamespacedName{Name: aiModel.Name + "-vllm-deployment", Namespace: AIModelNamespace}
			updatedDeployment := &appsv1.Deployment{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, lookUpKeyDeployment, updatedDeployment)
				return err == nil &&
					*updatedDeployment.Spec.Replicas == updatedReplicas &&
					updatedDeployment.Spec.Template.Spec.Containers[0].Image == updatedImage
			}, timeout, interval).Should(BeTrue())

			By("Deleting the AIModel resource after update test")
			Expect(k8sClient.Delete(ctx, aiModel)).Should(Succeed())

			// Wait for deletion to complete for subsequent tests
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: aiModel.Name, Namespace: AIModelNamespace}, aiModel)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})

		It("should delete Deployment and Service when AIModel is disabled", func() {
			ctx := context.Background()

			By("Creating a new AIModel to disable")
			aiModel := &aimodelv1alpha1.AIModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      AIModelName + "-disabled",
					Namespace: AIModelNamespace,
				},
				Spec: aimodelv1alpha1.AIModelSpec{
					ModelID: ModelID + "-disabled",
					Image:   Image,
					Enabled: true, // Initially enabled
				},
			}
			Expect(k8sClient.Create(ctx, aiModel)).Should(Succeed())

			// Ensure Deployment and Service are created first
			lookUpKeyDeployment := types.NamespacedName{Name: aiModel.Name + "-vllm-deployment", Namespace: AIModelNamespace}
			createdDeployment := &appsv1.Deployment{}
			Eventually(func() bool {
				_ = k8sClient.Get(ctx, lookUpKeyDeployment, createdDeployment)
				return !createdDeployment.ObjectMeta.CreationTimestamp.IsZero()
			}, timeout, interval).Should(BeTrue())
			
			By("Disabling the AIModel")
			aiModel.Spec.Enabled = false
			Expect(k8sClient.Update(ctx, aiModel)).Should(Succeed())

			By("Checking if Deployment is deleted after disabling")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, lookUpKeyDeployment, &appsv1.Deployment{})
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

			By("Checking if Service is deleted after disabling")
			lookUpKeyService := types.NamespacedName{Name: aiModel.Name + "-vllm-service", Namespace: AIModelNamespace}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, lookUpKeyService, &corev1.Service{})
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

			By("Checking AIModel status is updated to Disabled and InferenceEndpoint is empty")
			Eventually(func() bool {
				_ = k8sClient.Get(ctx, types.NamespacedName{Name: aiModel.Name, Namespace: AIModelNamespace}, aiModel)
				return aiModel.Status.Phase == "Disabled" &&
					aiModel.Status.InferenceEndpoint == ""
			}, timeout, interval).Should(BeTrue())

			By("Deleting the disabled AIModel resource")
			Expect(k8sClient.Delete(ctx, aiModel)).Should(Succeed())

			// Wait for deletion to complete for subsequent tests
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: aiModel.Name, Namespace: AIModelNamespace}, aiModel)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})

		It("should create a downloader Job when artifacts are not ready", func() {
			ctx := context.Background()

			// Mock IsModelArtifactsReady to initially return false, then true after a few calls
			callCount := 0
			mockIsModelArtifactsReady = func(ctx context.Context, aiModel *aimodelv1alpha1.AIModel) (bool, error) {
				callCount++
				if callCount < 3 {
					return false, nil
				}
				return true, nil
			}
			reconciler.IsModelArtifactsReady = mockIsModelArtifactsReady

			By("Creating a new AIModel that needs artifacts downloaded")
			aiModel := &aimodelv1alpha1.AIModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      AIModelName + "-download",
					Namespace: AIModelNamespace,
				},
				Spec: aimodelv1alpha1.AIModelSpec{
					ModelID: ModelID + "-download",
					Image:   Image,
					Enabled: true,
				},
			}
			Expect(k8sClient.Create(ctx, aiModel)).Should(Succeed())

			By("Checking AIModel status is updated to Downloading")
			Eventually(func() string {
				_ = k8sClient.Get(ctx, types.NamespacedName{Name: aiModel.Name, Namespace: AIModelNamespace}, aiModel)
				return aiModel.Status.Phase
			}, timeout, interval).Should(Equal("Downloading"))

			By("Checking if Downloader Job is created")
			lookUpKeyJob := types.NamespacedName{Name: aiModel.Name + "-downloader", Namespace: AIModelNamespace}
			createdJob := &batchv1.Job{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, lookUpKeyJob, createdJob)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			
			By("Simulating Downloader Job success")
			createdJob.Status.Succeeded = 1
			Expect(k8sClient.Status().Update(ctx, createdJob)).Should(Succeed())

			By("Checking AIModel status transitions to Ready and InferenceEndpoint is set")
			Eventually(func() bool {
				_ = k8sClient.Get(ctx, types.NamespacedName{Name: aiModel.Name, Namespace: AIModelNamespace}, aiModel)
				return aiModel.Status.Phase == "Ready" &&
					aiModel.Status.InferenceEndpoint == fmt.Sprintf("http://%s.%s.svc.cluster.local", aiModel.Name+"-vllm-service", AIModelNamespace)
			}, timeout, interval).Should(BeTrue())

			By("Checking if Deployment is created after download")
			lookUpKeyDeployment := types.NamespacedName{Name: aiModel.Name + "-vllm-deployment", Namespace: AIModelNamespace}
			createdDeployment := &appsv1.Deployment{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, lookUpKeyDeployment, createdDeployment)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Deleting the AIModel resource")
			Expect(k8sClient.Delete(ctx, aiModel)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: aiModel.Name, Namespace: AIModelNamespace}, aiModel)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})

		It("should set AIModel status to Failed if downloader Job fails", func() {
			ctx := context.Background()

			// Mock IsModelArtifactsReady to always return false to trigger job creation
			mockIsModelArtifactsReady = func(ctx context.Context, aiModel *aimodelv1alpha1.AIModel) (bool, error) {
				return false, nil
			}
			reconciler.IsModelArtifactsReady = mockIsModelArtifactsReady

			By("Creating a new AIModel for failed download test")
			aiModel := &aimodelv1alpha1.AIModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      AIModelName + "-failed-download",
					Namespace: AIModelNamespace,
				},
				Spec: aimodelv1alpha1.AIModelSpec{
					ModelID: ModelID + "-failed-download",
					Image:   Image,
					Enabled: true,
				},
			}
			Expect(k8sClient.Create(ctx, aiModel)).Should(Succeed())

			By("Checking AIModel status is updated to Downloading")
			Eventually(func() string {
				_ = k8sClient.Get(ctx, types.NamespacedName{Name: aiModel.Name, Namespace: AIModelNamespace}, aiModel)
				return aiModel.Status.Phase
			}, timeout, interval).Should(Equal("Downloading"))

			By("Checking if Downloader Job is created")
			lookUpKeyJob := types.NamespacedName{Name: aiModel.Name + "-downloader", Namespace: AIModelNamespace}
			createdJob := &batchv1.Job{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, lookUpKeyJob, createdJob)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Simulating Downloader Job failure")
			createdJob.Status.Failed = 1
			createdJob.Status.Conditions = []batchv1.JobCondition{
				{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, Reason: "DownloadFailed", Message: "Model download failed due to network error"},
			}
			Expect(k8sClient.Status().Update(ctx, createdJob)).Should(Succeed())

			By("Checking AIModel status transitions to Failed and InferenceEndpoint is empty")
			Eventually(func() bool {
				_ = k8sClient.Get(ctx, types.NamespacedName{Name: aiModel.Name, Namespace: AIModelNamespace}, aiModel)
				return aiModel.Status.Phase == "Failed" &&
					aiModel.Status.InferenceEndpoint == ""
			}, timeout, interval).Should(BeTrue())

			By("Deleting the AIModel resource")
			Expect(k8sClient.Delete(ctx, aiModel)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: aiModel.Name, Namespace: AIModelNamespace}, aiModel)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})

	})
})
