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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	aimodelv1alpha1 "github.com/ai-aas/ai-model-operator/api/v1alpha1"
	// +kubebuilder:scaffold:imports
)

const aiModelFinalizer = "aimodel.ai-aas.io/finalizer"
const modelDownloaderImage = "curlimages/curl:latest" // Placeholder, replace with actual image

var (
	reconcileTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aimodel_reconcile_total",
			Help: "Total number of AIModel reconciliations executed",
		},
		[]string{"result"},
	)
	reconcileDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "aimodel_reconcile_duration_seconds",
			Help:    "Duration of AIModel reconciliations",
			Buckets: prometheus.DefBuckets,
		},
	)
)

func init() {
	metrics.Registry.MustRegister(reconcileTotal, reconcileDuration)
}

// AIModelReconciler reconciles an AIModel object
type AIModelReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=aimodel.ai-aas.io,resources=aimodels,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=aimodel.ai-aas.io,resources=aimodels/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=aimodel.ai-aas.io,resources=aimodels/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify Reconcile to perform your desired operations with the correct Context.
// For more details, check Reconcile and its Context docs at:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *AIModelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	timer := prometheus.NewTimer(reconcileDuration)
	defer timer.ObserveDuration()

	log := log.FromContext(ctx)

	// Fetch the AIModel instance
	aiModel := &aimodelv1alpha1.AIModel{}
	if err := r.Get(ctx, req.NamespacedName, aiModel); err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "unable to fetch AIModel")
			reconcileTotal.WithLabelValues("error").Inc()
			return ctrl.Result{}, err
		}
		// Request object not found, could have been deleted after reconcile request.
		// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
		// Return and don't requeue
		reconcileTotal.WithLabelValues("success").Inc()
		return ctrl.Result{}, nil
	}

	// Set default phase if not already set
	if aiModel.Status.Phase == "" {
		aiModel.Status.Phase = "Pending"
		if err := r.Status().Update(ctx, aiModel); err != nil {
			log.Error(err, "unable to update AIModel status")
			reconcileTotal.WithLabelValues("error").Inc()
			return ctrl.Result{}, err
		}
	}

	// Examine if the object is being deleted
	isAIModelMarkedToBeDeleted := aiModel.GetDeletionTimestamp() != nil
	if isAIModelMarkedToBeDeleted {
		if controllerutil.ContainsFinalizer(aiModel, aiModelFinalizer) {
			// Run finalization logic. If it fails, don't remove the finalizer
			// so that we can retry during the next reconciliation.
			if err := r.finalizeAIModel(ctx, aiModel); err != nil {
				reconcileTotal.WithLabelValues("error").Inc()
				return ctrl.Result{}, err
			}

			// Remove aiModelFinalizer. Once all finalizers have been removed, the object will be deleted.
			controllerutil.RemoveFinalizer(aiModel, aiModelFinalizer)
			if err := r.Update(ctx, aiModel); err != nil {
				reconcileTotal.WithLabelValues("error").Inc()
				return ctrl.Result{}, err
			}
		}
		reconcileTotal.WithLabelValues("success").Inc()
		return ctrl.Result{}, nil
	}

	// Add finalizer for this CR
	if !controllerutil.ContainsFinalizer(aiModel, aiModelFinalizer) {
		controllerutil.AddFinalizer(aiModel, aiModelFinalizer)
		if err := r.Update(ctx, aiModel); err != nil {
			log.Error(err, "unable to add finalizer to AIModel")
			reconcileTotal.WithLabelValues("error").Inc()
			return ctrl.Result{}, err
		}
	}


	// Handle model deletion if Enabled is false
	if !aiModel.Spec.Enabled {
		log.Info("AIModel is disabled, scaling down/deleting resources", "name", aiModel.Name)
		// TODO: Implement logic to scale down or delete vLLM deployment and service
		// For now, let's just update the status
		if aiModel.Status.Phase != "Disabled" {
			aiModel.Status.Phase = "Disabled"
			if err := r.Status().Update(ctx, aiModel); err != nil {
				log.Error(err, "unable to update AIModel status to Disabled")
				reconcileTotal.WithLabelValues("error").Inc()
				return ctrl.Result{}, err
			}
		}
		reconcileTotal.WithLabelValues("success").Inc()
		return ctrl.Result{}, nil // Requeue if necessary to ensure cleanup
	}

	// 1. Reconcile Model Artifact Downloader Job (if not already successful)
	// TODO: Implement actual downloader job creation and status checking
	// For now, simulate success.
	jobName := fmt.Sprintf("%s-downloader", aiModel.Name)
	foundJob := &batchv1.Job{}
	err := r.Get(ctx, client.ObjectKey{Name: jobName, Namespace: aiModel.Namespace}, foundJob)
	if err != nil && client.IgnoreNotFound(err) != nil {
		log.Error(err, "Failed to get Job", "Job.Name", jobName)
		reconcileTotal.WithLabelValues("error").Inc()
		return ctrl.Result{}, err
	} else if err != nil { // Job not found, create it
		// Validate S3 artifact existence before creating the job
		// This prevents creating jobs that will inevitably fail if the artifact is missing.
		if err := r.checkS3ArtifactExists(ctx, aiModel); err != nil {
			log.Error(err, "S3 artifact check failed", "Bucket", aiModel.Spec.S3Bucket, "Key", aiModel.Spec.S3Key)
			reconcileTotal.WithLabelValues("error").Inc()
			// Update status to indicate failure
			aiModel.Status.Phase = "ArtifactMissing"
			if statusErr := r.Status().Update(ctx, aiModel); statusErr != nil {
				log.Error(statusErr, "unable to update AIModel status to ArtifactMissing")
			}
			return ctrl.Result{}, nil // Stop reconciliation until fixed
		}

		log.Info("Creating a new Model Downloader Job", "Job.Namespace", aiModel.Namespace, "Job.Name", jobName)
		job := r.modelDownloaderJob(aiModel, jobName)
		if err := ctrl.SetControllerReference(aiModel, job, r.Scheme); err != nil {
			log.Error(err, "Failed to set controller reference for Job", "Job.Name", job.Name)
			reconcileTotal.WithLabelValues("error").Inc()
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, job); err != nil {
			log.Error(err, "Failed to create Job", "Job.Name", job.Name)
			reconcileTotal.WithLabelValues("error").Inc()
			return ctrl.Result{}, err
		}
		aiModel.Status.Phase = "Downloading"
		if err := r.Status().Update(ctx, aiModel); err != nil {
			log.Error(err, "unable to update AIModel status to Downloading")
			reconcileTotal.WithLabelValues("error").Inc()
			return ctrl.Result{}, err
		}
		reconcileTotal.WithLabelValues("success").Inc()
		return ctrl.Result{Requeue: true}, nil // Requeue to wait for Job completion
	} else {
		// Job found, check its status
		if jobIsComplete(foundJob) {
			log.Info("Model Downloader Job completed successfully", "Job.Name", jobName)
			if aiModel.Status.Phase == "Downloading" || aiModel.Status.Phase == "Pending" {
				aiModel.Status.Phase = "Downloaded"
				if err := r.Status().Update(ctx, aiModel); err != nil {
					log.Error(err, "unable to update AIModel status to Downloaded")
					reconcileTotal.WithLabelValues("error").Inc()
					return ctrl.Result{}, err
				}
			}
		} else if jobIsFailed(foundJob) {
			log.Error(fmt.Errorf("job failed"), "Model Downloader Job failed", "Job.Name", jobName)
			if aiModel.Status.Phase != "Failed" {
				aiModel.Status.Phase = "Failed"
				if err := r.Status().Update(ctx, aiModel); err != nil {
					log.Error(err, "unable to update AIModel status to Failed")
					reconcileTotal.WithLabelValues("error").Inc()
					return ctrl.Result{}, err
				}
			}
			reconcileTotal.WithLabelValues("error").Inc()
			return ctrl.Result{}, nil // Do not requeue on failed job, manual intervention needed.
		} else {
			log.Info("Model Downloader Job still running", "Job.Name", jobName)
			reconcileTotal.WithLabelValues("success").Inc()
			return ctrl.Result{Requeue: true}, nil // Requeue to wait for Job completion
		}
	}



	// 2. Reconcile vLLM Deployment
	deploymentName := fmt.Sprintf("%s-vllm", aiModel.Name)
	vllmDeployment := &appsv1.Deployment{}
	err = r.Get(ctx, client.ObjectKey{Name: deploymentName, Namespace: aiModel.Namespace}, vllmDeployment)
	if err != nil && client.IgnoreNotFound(err) != nil {
		log.Error(err, "Failed to get vLLM Deployment", "Deployment.Name", deploymentName)
		reconcileTotal.WithLabelValues("error").Inc()
		return ctrl.Result{}, err
	} else if err != nil { // Deployment not found, create it
		log.Info("Creating a new vLLM Deployment", "Deployment.Namespace", aiModel.Namespace, "Deployment.Name", deploymentName)
		dep := r.vllmDeployment(aiModel, deploymentName)
		if err := ctrl.SetControllerReference(aiModel, dep, r.Scheme); err != nil {
			log.Error(err, "Failed to set controller reference for Deployment", "Deployment.Name", dep.Name)
			reconcileTotal.WithLabelValues("error").Inc()
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, dep); err != nil {
			log.Error(err, "Failed to create vLLM Deployment", "Deployment.Name", dep.Name)
			reconcileTotal.WithLabelValues("error").Inc()
			return ctrl.Result{}, err
		}
		aiModel.Status.VLLMDeploymentName = deploymentName
		aiModel.Status.Phase = "Deploying"
		if err := r.Status().Update(ctx, aiModel); err != nil {
			log.Error(err, "unable to update AIModel status to Deploying")
			reconcileTotal.WithLabelValues("error").Inc()
			return ctrl.Result{}, err
		}
		reconcileTotal.WithLabelValues("success").Inc()
		return ctrl.Result{Requeue: true}, nil // Requeue to wait for Deployment readiness
	} else {
		// Deployment found, update if necessary (e.g., replicas, image, etc.)
		expectedReplicas := int32(1)
		if aiModel.Spec.Replicas != nil {
			expectedReplicas = *aiModel.Spec.Replicas
		}

		if *vllmDeployment.Spec.Replicas != expectedReplicas {
			log.Info("Updating vLLM Deployment replicas", "Deployment.Name", deploymentName, "OldReplicas", *vllmDeployment.Spec.Replicas, "NewReplicas", expectedReplicas)
			vllmDeployment.Spec.Replicas = &expectedReplicas
			if err := r.Update(ctx, vllmDeployment); err != nil {
				log.Error(err, "Failed to update vLLM Deployment replicas", "Deployment.Name", deploymentName)
				reconcileTotal.WithLabelValues("error").Inc()
				return ctrl.Result{}, err
			}
			reconcileTotal.WithLabelValues("success").Inc()
			return ctrl.Result{Requeue: true}, nil // Requeue to observe update
		}

		if vllmDeployment.Status.ReadyReplicas == expectedReplicas && aiModel.Status.Phase == "Deploying" {
			log.Info("vLLM Deployment is ready", "Deployment.Name", deploymentName)
			aiModel.Status.Phase = "Ready"
			if err := r.Status().Update(ctx, aiModel); err != nil {
				log.Error(err, "unable to update AIModel status to Ready")
				reconcileTotal.WithLabelValues("error").Inc()
				return ctrl.Result{}, err
			}
		}
	}


	// 3. Reconcile vLLM Service
	serviceName := fmt.Sprintf("%s-vllm-svc", aiModel.Name)
	vllmService := &corev1.Service{}
	err = r.Get(ctx, client.ObjectKey{Name: serviceName, Namespace: aiModel.Namespace}, vllmService)
	if err != nil && client.IgnoreNotFound(err) != nil {
		log.Error(err, "Failed to get vLLM Service", "Service.Name", serviceName)
		reconcileTotal.WithLabelValues("error").Inc()
		return ctrl.Result{}, err
	} else if err != nil { // Service not found, create it
		log.Info("Creating a new vLLM Service", "Service.Namespace", aiModel.Namespace, "Service.Name", serviceName)
		svc := r.vllmService(aiModel, serviceName)
		if err := ctrl.SetControllerReference(aiModel, svc, r.Scheme); err != nil {
			log.Error(err, "Failed to set controller reference for Service", "Service.Name", svc.Name)
			reconcileTotal.WithLabelValues("error").Inc()
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, svc); err != nil {
			log.Error(err, "Failed to create vLLM Service", "Service.Name", svc.Name)
			reconcileTotal.WithLabelValues("error").Inc()
			return ctrl.Result{}, err
		}
		aiModel.Status.VLLMServiceName = serviceName
		if err := r.Status().Update(ctx, aiModel); err != nil {
			log.Error(err, "unable to update AIModel status with Service name")
			reconcileTotal.WithLabelValues("error").Inc()
			return ctrl.Result{}, err
		}
		reconcileTotal.WithLabelValues("success").Inc()
		return ctrl.Result{Requeue: true}, nil // Requeue to ensure service is ready (though services are usually quick)
	}

	reconcileTotal.WithLabelValues("success").Inc()
	return ctrl.Result{}, nil
}


func (r *AIModelReconciler) finalizeAIModel(ctx context.Context, aiModel *aimodelv1alpha1.AIModel) error {
	log := log.FromContext(ctx)
	log.Info("Performing finalization for AIModel", "name", aiModel.Name)

	// TODO(user): Add here the cleanup logic.
	// Delete associated Deployments, Services, Jobs

	log.Info("Successfully finalized AIModel", "name", aiModel.Name)
	return nil
}

// modelDownloaderJob creates a Kubernetes Job to download model artifacts.
func (r *AIModelReconciler) modelDownloaderJob(aiModel *aimodelv1alpha1.AIModel, jobName string) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: aiModel.Namespace,
			Labels:    labelsForAIModel(aiModel.Name),
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "downloader",
						Image: modelDownloaderImage,
						Command: []string{"curl", "-o", fmt.Sprintf("/model/%s", aiModel.Spec.ModelID), fmt.Sprintf("s3://%s/%s", aiModel.Spec.S3Bucket, aiModel.Spec.S3Key)},
						VolumeMounts: []corev1.VolumeMount{{
							Name: "model-storage",
							MountPath: "/model",
						}},
					}},
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Volumes: []corev1.Volume{{
						Name: "model-storage",
						// TODO: Use a persistent volume claim or host path for actual storage
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					}},
				},
			},
		},
	}
}

// vllmDeployment creates a Kubernetes Deployment for the vLLM model server.
func (r *AIModelReconciler) vllmDeployment(aiModel *aimodelv1alpha1.AIModel, deploymentName string) *appsv1.Deployment {
	replicas := int32(1)
	if aiModel.Spec.Replicas != nil {
		replicas = *aiModel.Spec.Replicas
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: aiModel.Namespace,
			Labels:    labelsForAIModel(aiModel.Name),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labelsForAIModel(aiModel.Name),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labelsForAIModel(aiModel.Name),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "vllm",
						// TODO: Use a configurable vLLM image and expose model path
						Image: "vllm/vllm-openai:latest", // Placeholder
						Ports: []corev1.ContainerPort{{
							ContainerPort: 8000,
							Name:          "http",
						}},
						// TODO: Mount model from shared volume, configure VLLM to load it
					}},
				},
			},
		},
	}
}

// vllmService creates a Kubernetes Service for the vLLM Deployment.
func (r *AIModelReconciler) vllmService(aiModel *aimodelv1alpha1.AIModel, serviceName string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: aiModel.Namespace,
			Labels:    labelsForAIModel(aiModel.Name),
		},
		Spec: corev1.ServiceSpec{
			Selector: labelsForAIModel(aiModel.Name),
			Ports: []corev1.ServicePort{{
				Port:       8000,
				TargetPort: intstr.FromInt(8000),
				Protocol:   corev1.ProtocolTCP,
				Name:       "http",
			}},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
}

// labelsForAIModel returns the labels for selecting the resources
// belonging to the given AIModel CR name.
func labelsForAIModel(name string) map[string]string {
	return map[string]string{"app": "aimodel", "aimodel_cr": name}
}

// Helper functions for Job status
func jobIsComplete(job *batchv1.Job) bool {
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobComplete && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func jobIsFailed(job *batchv1.Job) bool {
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}


func (r *AIModelReconciler) checkS3ArtifactExists(ctx context.Context, aiModel *aimodelv1alpha1.AIModel) error {
	// TODO: Implement actual S3 HEAD request using AWS SDK
	// Requires AWS credentials to be configured in the operator's environment.
	// For now, we assume the artifact exists.
	// Example logic:
	// sess := session.Must(session.NewSession())
	// svc := s3.New(sess)
	// _, err := svc.HeadObject(&s3.HeadObjectInput{Bucket: &aiModel.Spec.S3Bucket, Key: &aiModel.Spec.S3Key})
	// return err
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AIModelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&aimodelv1alpha1.AIModel{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
