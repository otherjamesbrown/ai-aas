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
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	aimodelv1alpha1 "github.com/ai-aas/ai-model-operator/api/v1alpha1"
)

// AIModelReconciler reconciles a AIModel object
type AIModelReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=aimodel.ai-aas.io,resources=aimodels,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=aimodel.ai-aas.io,resources=aimodels/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=aimodel.ai-aas.io,resources=aimodels/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify Reconcile to be able to reconcile some object type. (e.g. Pod)
// For more details, check Reconcile and its Result here: 
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.0/pkg/reconcile
func (r *AIModelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	log := log.FromContext(ctx)

	// Increment reconcile total counter on entry
	ReconcileTotal.WithLabelValues("total").Inc()

	// Fetch the AIModel instance
	aiModel := &aimodelv1alpha1.AIModel{}
	if err := r.Get(ctx, req.NamespacedName, aiModel); err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "unable to fetch AIModel")
			ReconcileTotal.WithLabelValues("error").Inc() // Increment error metric
			return ctrl.Result{}, err
		}
		// AIModel not found, might have been deleted
		log.Info("AIModel resource not found. Ignoring since object must be deleted")
		ReconcileTotal.WithLabelValues("skipped").Inc() // Increment skipped metric
		return ctrl.Result{}, nil
	}

	// Store old phase for metric updates
	oldPhase := aiModel.Status.Phase

	// Check if the AIModel instance is marked for deletion
	isAIModelMarkedForDeletion := !aiModel.ObjectMeta.DeletionTimestamp.IsZero()

	// The object is not marked for deletion, so ensure it has the finalizer
	if !isAIModelMarkedForDeletion {
		if !controllerutil.ContainsFinalizer(aiModel, aimodelv1alpha1.AIModelFinalizer) {
			log.Info("Adding Finalizer for AIModel")
			controllerutil.AddFinalizer(aiModel, aimodelv1alpha1.AIModelFinalizer)
			if err := r.Update(ctx, aiModel); err != nil {
				log.Error(err, "Failed to add finalizer to AIModel")
				ReconcileTotal.WithLabelValues("error").Inc() // Increment error metric
				return ctrl.Result{}, err
			}
			ReconcileTotal.WithLabelValues("success").Inc() // Increment success metric
			return ctrl.Result{Requeue: true}, nil // Requeue to restart reconciliation
		}
	} else {
		// The object is marked for deletion
		if controllerutil.ContainsFinalizer(aiModel, aimodelv1alpha1.AIModelFinalizer) {
			log.Info("Performing Finalizer Operations for AIModel before deletion - owned resources will be garbage collected by Kubernetes")
			// No explicit cleanup needed here for owned Deployments/Services due to OwnerReference and Kubernetes GC.

			// Once all finalizer operations have been successfully completed, remove the finalizer
			controllerutil.RemoveFinalizer(aiModel, aimodelv1alpha1.AIModelFinalizer)
			if err := r.Update(ctx, aiModel); err != nil {
				log.Error(err, "Failed to remove finalizer from AIModel")
				ReconcileTotal.WithLabelValues("error").Inc() // Increment error metric
				return ctrl.Result{}, err
			}
			AIModelStatusPhase.WithLabelValues(aiModel.Name, aiModel.Namespace, oldPhase).Set(0) // Clear old phase metric
			ReconcileTotal.WithLabelValues("success").Inc() // Increment success metric
			return ctrl.Result{}, nil // Do not requeue, allow Kubernetes to delete the object
		}
		log.Info("AIModel object is in the process of being deleted")
		ReconcileTotal.WithLabelValues("skipped").Inc() // Increment skipped metric
		return ctrl.Result{}, nil
	}

	log.Info("Reconciling AIModel", "modelID", aiModel.Spec.ModelID, "namespace", aiModel.Namespace, "enabled", aiModel.Spec.Enabled)

	// If the model is disabled, ensure resources are deleted
	if !aiModel.Spec.Enabled {
		log.Info("AIModel is disabled, deleting resources")

		// Delete Deployment if it exists
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      aiModel.Name + "-vllm-deployment",
				Namespace: aiModel.Namespace,
			},
		}
		if err := r.Delete(ctx, deployment); err != nil {
			if !errors.IsNotFound(err) {
				log.Error(err, "Failed to delete Deployment", "Deployment.Name", deployment.Name)
				ReconcileTotal.WithLabelValues("error").Inc() // Increment error metric
				return ctrl.Result{}, err
			}
		} else {
			log.Info("Deleted Deployment", "Deployment.Name", deployment.Name)
		}

		// Delete Service if it exists
		service := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      aiModel.Name + "-vllm-service",
				Namespace: aiModel.Namespace,
			},
		}
		if err := r.Delete(ctx, service); err != nil {
			if !errors.IsNotFound(err) {
				log.Error(err, "Failed to delete Service", "Service.Name", service.Name)
				ReconcileTotal.WithLabelValues("error").Inc() // Increment error metric
				return ctrl.Result{}, err
			}
		} else {
			log.Info("Deleted Service", "Service.Name", service.Name)
		}

		// Delete Downloader Job if it exists
		downloaderJob := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      aiModel.Name + "-downloader",
				Namespace: aiModel.Namespace,
			},
		}
		if err := r.Delete(ctx, downloaderJob); err != nil {
			if !errors.IsNotFound(err) {
				log.Error(err, "Failed to delete Downloader Job", "Job.Name", downloaderJob.Name)
				ReconcileTotal.WithLabelValues("error").Inc() // Increment error metric
				return ctrl.Result{}, err
			}
		} else {
			log.Info("Deleted Downloader Job", "Job.Name", downloaderJob.Name)
		}

		// Update Status
		if aiModel.Status.Phase != "Disabled" {
			AIModelStatusPhase.WithLabelValues(aiModel.Name, aiModel.Namespace, oldPhase).Set(0) // Clear old phase metric
			aiModel.Status.Phase = "Disabled"
			AIModelStatusPhase.WithLabelValues(aiModel.Name, aiModel.Namespace, aiModel.Status.Phase).Set(1) // Set new phase metric
			if err := r.Status().Update(ctx, aiModel); err != nil {
				log.Error(err, "Failed to update AIModel status to Disabled")
				ReconcileTotal.WithLabelValues("error").Inc() // Increment error metric
				return ctrl.Result{}, err
			}
		}

		ReconcileTotal.WithLabelValues("success").Inc() // Increment success metric
		return ctrl.Result{}, nil
	}

	// Handle model artifact download lifecycle
	modelArtifactsReady, err := r.IsModelArtifactsReady(ctx, aiModel)
	if err != nil {
		log.Error(err, "Failed to check model artifacts readiness")
		ReconcileTotal.WithLabelValues("error").Inc() // Increment error metric
		return ctrl.Result{}, err
	}

	if !modelArtifactsReady {
		// Model artifacts are not ready, initiate or monitor download job
		downloaderJob := &batchv1.Job{}
		err := r.Get(ctx, client.ObjectKey{Name: aiModel.Name + "-downloader", Namespace: aiModel.Namespace}, downloaderJob)
		
		if err != nil && errors.IsNotFound(err) {
			// Downloader Job does not exist, create it
			desiredJob := generateDownloaderJob(aiModel)
			if err := ctrl.SetControllerReference(aiModel, desiredJob, r.Scheme); err != nil {
				log.Error(err, "Failed to set controller reference for Downloader Job")
				ReconcileTotal.WithLabelValues("error").Inc() // Increment error metric
				return ctrl.Result{}, err
			}
			if err := r.Create(ctx, desiredJob); err != nil {
				log.Error(err, "Failed to create Downloader Job", "Job.Name", desiredJob.Name)
				ReconcileTotal.WithLabelValues("error").Inc() // Increment error metric
				return ctrl.Result{}, err
			}
			log.Info("Created Downloader Job", "Job.Name", desiredJob.Name)

			// Update AIModel status to Downloading
			if aiModel.Status.Phase != "Downloading" {
				AIModelStatusPhase.WithLabelValues(aiModel.Name, aiModel.Namespace, oldPhase).Set(0) // Clear old phase metric
				aiModel.Status.Phase = "Downloading"
				AIModelStatusPhase.WithLabelValues(aiModel.Name, aiModel.Namespace, aiModel.Status.Phase).Set(1) // Set new phase metric
				if err := r.Status().Update(ctx, aiModel); err != nil {
					log.Error(err, "Failed to update AIModel status to Downloading")
					ReconcileTotal.WithLabelValues("error").Inc() // Increment error metric
					return ctrl.Result{}, err
				}
			}
			ReconcileTotal.WithLabelValues("success").Inc() // Increment success metric
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil // Requeue to check job status
		} else if err != nil {
			// Other error fetching the Job
			log.Error(err, "Failed to get Downloader Job")
			ReconcileTotal.WithLabelValues("error").Inc() // Increment error metric
			return ctrl.Result{}, err
		}

		// Downloader Job exists, check its status
		if downloaderJob.Status.Succeeded > 0 {
			log.Info("Downloader Job succeeded", "Job.Name", downloaderJob.Name)
			// Artifacts are ready, next reconciliation will find them ready
			ReconcileTotal.WithLabelValues("success").Inc() // Increment success metric
			return ctrl.Result{Requeue: true}, nil 
		} else if downloaderJob.Status.Failed > 0 {
			log.Error(nil, "Downloader Job failed", "Job.Name", downloaderJob.Name, "Message", downloaderJob.Status.Conditions[0].Message)

			// Update AIModel status to Failed
			if aiModel.Status.Phase != "Failed" {
				AIModelStatusPhase.WithLabelValues(aiModel.Name, aiModel.Namespace, oldPhase).Set(0) // Clear old phase metric
				aiModel.Status.Phase = "Failed"
				AIModelStatusPhase.WithLabelValues(aiModel.Name, aiModel.Namespace, aiModel.Status.Phase).Set(1) // Set new phase metric
				if err := r.Status().Update(ctx, aiModel); err != nil {
					log.Error(err, "Failed to update AIModel status to Failed")
					ReconcileTotal.WithLabelValues("error").Inc() // Increment error metric
					return ctrl.Result{}, err
				}
			}
			ReconcileTotal.WithLabelValues("error").Inc() // Increment error metric
			return ctrl.Result{}, nil // Do not requeue, manual intervention needed
		} else {
			log.Info("Downloader Job still running or pending", "Job.Name", downloaderJob.Name)
			// Requeue to check job status later
			ReconcileTotal.WithLabelValues("skipped").Inc() // Increment skipped metric
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
	}

	// If model artifacts are ready, proceed with Deployment and Service reconciliation
	// Ensure AIModel status is Ready if it was previously Downloading
	if aiModel.Status.Phase == "Downloading" || aiModel.Status.Phase == ""{
		log.Info("Model artifacts are ready, updating AIModel status to Ready")
		AIModelStatusPhase.WithLabelValues(aiModel.Name, aiModel.Namespace, oldPhase).Set(0) // Clear old phase metric
		aiModel.Status.Phase = "Ready"
		AIModelStatusPhase.WithLabelValues(aiModel.Name, aiModel.Namespace, aiModel.Status.Phase).Set(1) // Set new phase metric
		if err := r.Status().Update(ctx, aiModel); err != nil {
			log.Error(err, "Failed to update AIModel status to Ready")
			ReconcileTotal.WithLabelValues("error").Inc() // Increment error metric
			return ctrl.Result{}, err
		}
		ReconcileTotal.WithLabelValues("success").Inc() // Increment success metric
		return ctrl.Result{Requeue: true}, nil // Requeue to continue with deployment
	}



	// Generate desired vLLM Deployment
	desiredDeployment := generateDeployment(aiModel)
	// Set AIModel instance as the owner and controller
	if err := ctrl.SetControllerReference(aiModel, desiredDeployment, r.Scheme); err != nil {
		log.Error(err, "Failed to set controller reference for Deployment")
		ReconcileTotal.WithLabelValues("error").Inc() // Increment error metric
		return ctrl.Result{}, err
	}

	// Create or update the Deployment
	foundDeployment := &appsv1.Deployment{}
	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, foundDeployment, func() error {
		// If the deployment does not exist, initialize it with the desired state
		if foundDeployment.ObjectMeta.CreationTimestamp.IsZero() {
			foundDeployment.ObjectMeta.Labels = desiredDeployment.ObjectMeta.Labels
			foundDeployment.Spec = desiredDeployment.Spec
			return nil
		}
		// Update the deployment fields to match the desired state
		foundDeployment.Spec.Replicas = desiredDeployment.Spec.Replicas
		foundDeployment.Spec.Template.Spec.Containers[0].Image = desiredDeployment.Spec.Template.Spec.Containers[0].Image
		// TODO: Update other fields like env vars, resources, etc. as they are added to AIModelSpec
		return nil
	})
	if err != nil {
		log.Error(err, "Failed to create or update Deployment", "Deployment.Name", desiredDeployment.Name)
		ReconcileTotal.WithLabelValues("error").Inc() // Increment error metric
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		log.Info("Deployment reconciled", "Operation", op, "Deployment.Name", desiredDeployment.Name)
	}

	// Generate desired vLLM Service
	desiredService := generateService(aiModel)
	// Set AIModel instance as the owner and controller
	if err := ctrl.SetControllerReference(aiModel, desiredService, r.Scheme); err != nil {
		log.Error(err, "Failed to set controller reference for Service")
		ReconcileTotal.WithLabelValues("error").Inc() // Increment error metric
		return ctrl.Result{}, err
	}

	// Create or update the Service
	foundService := &corev1.Service{}
	op, err = controllerutil.CreateOrUpdate(ctx, r.Client, foundService, func() error {
		// If the service does not exist, initialize it with the desired state
		if foundService.ObjectMeta.CreationTimestamp.IsZero() {
			foundService.ObjectMeta.Labels = desiredService.ObjectMeta.Labels
			foundService.Spec = desiredService.Spec
			return nil
		}
		// Update the service fields to match the desired state
		// For a simple ClusterIP service, selector is usually the main mutable field
		foundService.Spec.Selector = desiredService.Spec.Selector
		return nil
	})
	if err != nil {
		log.Error(err, "Failed to create or update Service", "Service.Name", desiredService.Name)
		ReconcileTotal.WithLabelValues("error").Inc() // Increment error metric
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		log.Info("Service reconciled", "Operation", op, "Service.Name", desiredService.Name)
	}

	// Update AIModel status
	aiModel.Status.Phase = "Ready"
	aiModel.Status.InferenceEndpoint = fmt.Sprintf("http://%s.%s.svc.cluster.local", foundService.Name, foundService.Namespace)

	if err := r.Status().Update(ctx, aiModel); err != nil {
		log.Error(err, "Failed to update AIModel status")
		ReconcileTotal.WithLabelValues("error").Inc() // Increment error metric
		return ctrl.Result{}, err
	}

	ReconcileTotal.WithLabelValues("success").Inc() // Increment success metric
	return ctrl.Result{}, nil}

// SetupWithManager sets up the controller with the Manager.
func (r *AIModelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&aimodelv1alpha1.AIModel{}).
		Owns(&appsv1.Deployment{}).    // Watch Deployments owned by AIModel
		Owns(&corev1.Service{}).       // Watch Services owned by AIModel
		Owns(&batchv1.Job{}).          // Watch Jobs owned by AIModel
		Complete(r)
}

// IsModelArtifactsReady is a placeholder function to check if model artifacts are ready in S3.
// This will be properly implemented in a later task (T013).
func (r *AIModelReconciler) IsModelArtifactsReady(ctx context.Context, aiModel *aimodelv1alpha1.AIModel) (bool, error) {
	// TODO: Implement actual S3 check logic in T013
	// For now, simulate by always returning true after a short delay (for testing purposes)
	// In a real scenario, this would query S3 to check for model files.
	return true, nil
}
