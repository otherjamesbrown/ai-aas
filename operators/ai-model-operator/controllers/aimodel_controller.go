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
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	aimodelv1alpha1 "github.com/ai-aas/ai-model-operator/api/v1alpha1"
	"github.com/ai-aas/ai-model-operator/internal/kserve"
	// +kubebuilder:scaffold:imports
)

const aiModelFinalizer = "aimodel.ai-aas.io/finalizer"
const modelDownloaderImage = "python:3.11-slim" // Python image for HuggingFace Hub downloads

// Retry configuration for failed download jobs
const (
	maxDownloadRetries = 5
	initialRetryDelay  = 1 * time.Minute
	maxRetryDelay      = 16 * time.Minute
)

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
//+kubebuilder:rbac:groups=serving.kserve.io,resources=inferenceservices,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=serving.kserve.io,resources=inferenceservices/status,verbs=get
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
	} else if err != nil { // Job not found
		// Handle RetryPending phase - check if it's time to retry
		if aiModel.Status.Phase == aimodelv1alpha1.AIModelPhaseRetryPending {
			if aiModel.Status.NextRetryTime != nil {
				now := metav1.Now()
				if now.Time.Before(aiModel.Status.NextRetryTime.Time) {
					// Not yet time to retry, requeue with remaining wait time
					waitDuration := aiModel.Status.NextRetryTime.Time.Sub(now.Time)
					log.Info("Retry not yet due, waiting", "waitDuration", waitDuration)
					reconcileTotal.WithLabelValues("success").Inc()
					return ctrl.Result{RequeueAfter: waitDuration}, nil
				}
				// Time to retry - fall through to job creation logic
				log.Info("Retry time reached, creating new download job", "retryCount", aiModel.Status.RetryCount)
			}
		}

		// Check if S3 artifacts already exist (e.g., from a previous download or manual upload)
		// If they exist, we can skip the downloader job and proceed to InferenceService creation
		if err := r.checkS3ArtifactExists(ctx, aiModel); err == nil {
			// Artifacts already exist in S3, skip download phase
			log.Info("S3 artifacts already exist, skipping download", "Bucket", aiModel.Spec.S3Bucket, "Key", aiModel.Spec.S3Key)
			aiModel.Status.Phase = aimodelv1alpha1.AIModelPhaseDeploying
			// Reset retry count on success
			aiModel.Status.RetryCount = 0
			aiModel.Status.LastRetryTime = nil
			aiModel.Status.NextRetryTime = nil
			if statusErr := r.Status().Update(ctx, aiModel); statusErr != nil {
				log.Error(statusErr, "unable to update AIModel status to Deploying")
				return ctrl.Result{}, statusErr
			}
			// Continue to InferenceService creation (fall through)
		} else {
			// Artifacts not found in S3, create downloader job to fetch from HuggingFace
			log.Info("S3 artifacts not found, creating downloader job", "Bucket", aiModel.Spec.S3Bucket, "Key", aiModel.Spec.S3Key)
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
			aiModel.Status.Phase = aimodelv1alpha1.AIModelPhaseDownloading
			if err := r.Status().Update(ctx, aiModel); err != nil {
				log.Error(err, "unable to update AIModel status to Downloading")
				reconcileTotal.WithLabelValues("error").Inc()
				return ctrl.Result{}, err
			}
			reconcileTotal.WithLabelValues("success").Inc()
			return ctrl.Result{Requeue: true}, nil // Requeue to wait for Job completion
		}
	} else {
		// Job found, check its status
		if jobIsComplete(foundJob) {
			log.Info("Model Downloader Job completed successfully", "Job.Name", jobName)
			if aiModel.Status.Phase == "Downloading" || aiModel.Status.Phase == "Pending" {
				aiModel.Status.Phase = "Downloaded"
				// Reset retry tracking on success
				aiModel.Status.RetryCount = 0
				aiModel.Status.LastRetryTime = nil
				aiModel.Status.NextRetryTime = nil
				if err := r.Status().Update(ctx, aiModel); err != nil {
					log.Error(err, "unable to update AIModel status to Downloaded")
					reconcileTotal.WithLabelValues("error").Inc()
					return ctrl.Result{}, err
				}
			}
		} else if jobIsFailed(foundJob) {
			log.Error(fmt.Errorf("job failed"), "Model Downloader Job failed", "Job.Name", jobName)

			// Check if we should retry
			if aiModel.Status.RetryCount < maxDownloadRetries {
				// Delete the failed job so a new one can be created
				log.Info("Deleting failed job to prepare for retry", "Job.Name", jobName, "retryCount", aiModel.Status.RetryCount)
				if err := r.Delete(ctx, foundJob, client.PropagationPolicy(metav1.DeletePropagationForeground)); err != nil {
					log.Error(err, "Failed to delete failed Job", "Job.Name", jobName)
					reconcileTotal.WithLabelValues("error").Inc()
					return ctrl.Result{}, err
				}

				// Increment retry count and calculate backoff
				aiModel.Status.RetryCount++
				backoffDuration := calculateRetryBackoff(aiModel.Status.RetryCount - 1) // Use previous count for backoff
				now := metav1.Now()
				nextRetry := metav1.NewTime(now.Add(backoffDuration))

				aiModel.Status.Phase = aimodelv1alpha1.AIModelPhaseRetryPending
				aiModel.Status.LastRetryTime = &now
				aiModel.Status.NextRetryTime = &nextRetry
				aiModel.Status.Message = fmt.Sprintf("Download failed, retry %d/%d scheduled in %v",
					aiModel.Status.RetryCount, maxDownloadRetries, backoffDuration)

				if err := r.Status().Update(ctx, aiModel); err != nil {
					log.Error(err, "unable to update AIModel status to RetryPending")
					reconcileTotal.WithLabelValues("error").Inc()
					return ctrl.Result{}, err
				}

				log.Info("Retry scheduled", "retryCount", aiModel.Status.RetryCount,
					"backoffDuration", backoffDuration, "nextRetryTime", nextRetry.Time)
				reconcileTotal.WithLabelValues("retry").Inc()
				return ctrl.Result{RequeueAfter: backoffDuration}, nil
			} else {
				// Max retries exceeded, mark as permanently failed
				log.Error(fmt.Errorf("max retries exceeded"), "Download failed after maximum retries",
					"Job.Name", jobName, "maxRetries", maxDownloadRetries)
				aiModel.Status.Phase = aimodelv1alpha1.AIModelPhaseFailed
				aiModel.Status.Message = fmt.Sprintf("Download failed after %d retry attempts. Manual intervention required.", maxDownloadRetries)
				if err := r.Status().Update(ctx, aiModel); err != nil {
					log.Error(err, "unable to update AIModel status to Failed")
					reconcileTotal.WithLabelValues("error").Inc()
					return ctrl.Result{}, err
				}
				reconcileTotal.WithLabelValues("error").Inc()
				return ctrl.Result{}, nil // Do not requeue - permanent failure
			}
		} else {
			log.Info("Model Downloader Job still running", "Job.Name", jobName)
			reconcileTotal.WithLabelValues("success").Inc()
			return ctrl.Result{Requeue: true}, nil // Requeue to wait for Job completion
		}
	}



	// 2. Reconcile KServe InferenceService (replaces vLLM Deployment + Service)
	log.Info("Reconciling InferenceService", "name", aiModel.Name)

	// Update phase to Deploying if not already set
	if aiModel.Status.Phase == "Downloaded" || aiModel.Status.Phase == "Pending" {
		aiModel.Status.Phase = aimodelv1alpha1.AIModelPhaseDeploying
		if err := r.Status().Update(ctx, aiModel); err != nil {
			log.Error(err, "unable to update AIModel status to Deploying")
			reconcileTotal.WithLabelValues("error").Inc()
			return ctrl.Result{}, err
		}
	}

	// Create or update the InferenceService
	if err := r.createOrUpdateInferenceService(ctx, aiModel); err != nil {
		log.Error(err, "Failed to reconcile InferenceService", "name", aiModel.Name)
		aiModel.Status.Phase = aimodelv1alpha1.AIModelPhaseFailed
		aiModel.Status.Message = fmt.Sprintf("Failed to create InferenceService: %v", err)
		if statusErr := r.Status().Update(ctx, aiModel); statusErr != nil {
			log.Error(statusErr, "unable to update AIModel status to Failed")
		}
		reconcileTotal.WithLabelValues("error").Inc()
		return ctrl.Result{}, err
	}

	// Update status from InferenceService
	if err := r.updateStatusFromInferenceService(ctx, aiModel); err != nil {
		log.Error(err, "Failed to update status from InferenceService", "name", aiModel.Name)
		reconcileTotal.WithLabelValues("error").Inc()
		return ctrl.Result{}, err
	}

	// Requeue if not ready yet
	if aiModel.Status.Phase != aimodelv1alpha1.AIModelPhaseReady {
		log.Info("InferenceService not ready yet, requeuing", "name", aiModel.Name, "phase", aiModel.Status.Phase)
		reconcileTotal.WithLabelValues("success").Inc()
		return ctrl.Result{Requeue: true}, nil
	}

	reconcileTotal.WithLabelValues("success").Inc()
	return ctrl.Result{}, nil
}


func (r *AIModelReconciler) finalizeAIModel(ctx context.Context, aiModel *aimodelv1alpha1.AIModel) error {
	log := log.FromContext(ctx)
	log.Info("Performing finalization for AIModel", "name", aiModel.Name)

	// Delete InferenceService
	isvc := &unstructured.Unstructured{}
	isvc.SetGroupVersionKind(kserve.InferenceServiceGVK)
	if err := r.Get(ctx, client.ObjectKey{Name: aiModel.Name, Namespace: aiModel.Namespace}, isvc); err == nil {
		log.Info("Deleting InferenceService", "name", aiModel.Name)
		if err := r.Delete(ctx, isvc); err != nil && !errors.IsNotFound(err) {
			log.Error(err, "Failed to delete InferenceService", "name", aiModel.Name)
		}
	}

	// Delete legacy vLLM Deployment (for backward compatibility)
	deploymentName := fmt.Sprintf("%s-vllm", aiModel.Name)
	dep := &appsv1.Deployment{}
	if err := r.Get(ctx, client.ObjectKey{Name: deploymentName, Namespace: aiModel.Namespace}, dep); err == nil {
		log.Info("Deleting legacy Deployment", "name", deploymentName)
		if err := r.Delete(ctx, dep); err != nil && !errors.IsNotFound(err) {
			log.Error(err, "Failed to delete Deployment", "name", deploymentName)
		}
	}

	// Delete legacy vLLM Service (for backward compatibility)
	serviceName := fmt.Sprintf("%s-vllm-svc", aiModel.Name)
	svc := &corev1.Service{}
	if err := r.Get(ctx, client.ObjectKey{Name: serviceName, Namespace: aiModel.Namespace}, svc); err == nil {
		log.Info("Deleting legacy Service", "name", serviceName)
		if err := r.Delete(ctx, svc); err != nil && !errors.IsNotFound(err) {
			log.Error(err, "Failed to delete Service", "name", serviceName)
		}
	}

	// Delete Downloader Job
	jobName := fmt.Sprintf("%s-downloader", aiModel.Name)
	job := &batchv1.Job{}
	if err := r.Get(ctx, client.ObjectKey{Name: jobName, Namespace: aiModel.Namespace}, job); err == nil {
		log.Info("Deleting Job", "name", jobName)
		propagationPolicy := metav1.DeletePropagationBackground
		if err := r.Delete(ctx, job, &client.DeleteOptions{
			PropagationPolicy: &propagationPolicy,
		}); err != nil && !errors.IsNotFound(err) {
			log.Error(err, "Failed to delete Job", "name", jobName)
		}
	}

	log.Info("Successfully finalized AIModel", "name", aiModel.Name)
	return nil
}

// modelDownloaderJob creates a Kubernetes Job to download model artifacts from HuggingFace Hub
// and upload them to S3.
func (r *AIModelReconciler) modelDownloaderJob(aiModel *aimodelv1alpha1.AIModel, jobName string) *batchv1.Job {
	// Python script that downloads from HuggingFace Hub and uploads to S3
	downloadScript := `
pip install -q huggingface_hub boto3 &&
python3 -c "
from huggingface_hub import snapshot_download
import boto3
import os

model_id = os.environ['MODEL_ID']
s3_bucket = os.environ['S3_BUCKET']
s3_key = os.environ['S3_KEY']
s3_endpoint = os.environ.get('AWS_ENDPOINT_URL_S3') or os.environ.get('S3_ENDPOINT')

print(f'Downloading {model_id} from HuggingFace Hub...')
local_dir = snapshot_download(
    model_id,
    local_dir='/tmp/model',
    token=os.environ.get('HF_TOKEN'),
)

print(f'Uploading to s3://{s3_bucket}/{s3_key}/')
s3_config = {}
if s3_endpoint:
    s3_config['endpoint_url'] = s3_endpoint
    print(f'Using custom S3 endpoint: {s3_endpoint}')
s3 = boto3.client('s3', **s3_config)
for root, dirs, files in os.walk(local_dir):
    for file in files:
        local_path = os.path.join(root, file)
        rel_path = os.path.relpath(local_path, local_dir)
        s3_path = f'{s3_key}/{rel_path}'
        print(f'  Uploading {rel_path}...')
        s3.upload_file(local_path, s3_bucket, s3_path)
print('Upload complete!')
"
`

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
						Command: []string{"/bin/sh", "-c"},
						Args: []string{downloadScript},
						Env: []corev1.EnvVar{
							{
								Name:  "MODEL_ID",
								Value: aiModel.Spec.ModelID,
							},
							{
								Name:  "S3_BUCKET",
								Value: aiModel.Spec.S3Bucket,
							},
							{
								Name:  "S3_KEY",
								Value: aiModel.Spec.S3Key,
							},
							{
								Name: "HF_TOKEN",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "hf-credentials",
										},
										Key: "token",
										Optional: func() *bool {
											opt := true
											return &opt
										}(),
									},
								},
							},
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
						},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "model-storage",
							MountPath: "/tmp/model",
						}},
					}},
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Volumes: []corev1.Volume{{
						Name: "model-storage",
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
// NOTE: This function is kept for backward compatibility with legacy deployments.
// New deployments should use KServe InferenceService via createOrUpdateInferenceService.
func (r *AIModelReconciler) vllmDeployment(aiModel *aimodelv1alpha1.AIModel, deploymentName string) *appsv1.Deployment {
	replicas := int32(1)
	if aiModel.Spec.Replicas != nil {
		replicas = *aiModel.Spec.Replicas
	}

	// Determine runtime image from spec, with defaults
	image := "vllm/vllm-openai:v0.6.3"
	switch aiModel.Spec.Runtime {
	case "tgi":
		image = "ghcr.io/huggingface/text-generation-inference:latest"
	case "triton":
		image = "nvcr.io/nvidia/tritonserver:latest"
	case "vllm", "":
		image = "vllm/vllm-openai:v0.6.3"
	}

	// Build args: start with required args, then append user-specified RuntimeArgs
	args := []string{
		"--model", fmt.Sprintf("s3://%s/%s", aiModel.Spec.S3Bucket, aiModel.Spec.S3Key),
		"--served-model-name", aiModel.Spec.ModelName,
	}
	if len(aiModel.Spec.RuntimeArgs) > 0 {
		args = append(args, aiModel.Spec.RuntimeArgs...)
	} else {
		// Default args if none specified
		args = append(args,
			"--dtype", "auto",
			"--max-model-len", "4096",
			"--gpu-memory-utilization", "0.9",
			"--trust-remote-code",
		)
	}

	// Build env vars: S3 credentials + user-specified RuntimeEnv
	env := []corev1.EnvVar{
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
	}
	env = append(env, aiModel.Spec.RuntimeEnv...)

	// Use resources from spec if specified, otherwise defaults
	resources := aiModel.Spec.Resources
	if len(resources.Requests) == 0 && len(resources.Limits) == 0 {
		resources = corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				"nvidia.com/gpu": resource.MustParse("1"),
			},
			Requests: corev1.ResourceList{
				"nvidia.com/gpu": resource.MustParse("1"),
			},
		}
	}

	// Use tolerations from spec if specified, otherwise defaults
	tolerations := aiModel.Spec.Tolerations
	if len(tolerations) == 0 {
		tolerations = []corev1.Toleration{
			{
				Key:      "nvidia.com/gpu",
				Operator: corev1.TolerationOpExists,
				Effect:   corev1.TaintEffectNoSchedule,
			},
			{
				Key:      "gpu-workload",
				Operator: corev1.TolerationOpEqual,
				Value:    "true",
				Effect:   corev1.TaintEffectNoSchedule,
			},
		}
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
						Image: image,
						Args:  args,
						Ports: []corev1.ContainerPort{{
							ContainerPort: 8000,
							Name:          "http",
						}},
						Env:       env,
						Resources: resources,
					}},
					Tolerations:  tolerations,
					NodeSelector: aiModel.Spec.NodeSelector,
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

// calculateRetryBackoff calculates the exponential backoff duration for a given retry count.
// Returns min(initialRetryDelay * 2^retryCount, maxRetryDelay)
func calculateRetryBackoff(retryCount int32) time.Duration {
	backoff := initialRetryDelay * time.Duration(1<<retryCount)
	if backoff > maxRetryDelay {
		return maxRetryDelay
	}
	return backoff
}

func (r *AIModelReconciler) checkS3ArtifactExists(ctx context.Context, aiModel *aimodelv1alpha1.AIModel) error {
	log := log.FromContext(ctx)

	// Read S3 credentials secret to get endpoint URL
	secret := &corev1.Secret{}
	secretName := types.NamespacedName{
		Name:      "s3-credentials",
		Namespace: aiModel.Namespace,
	}
	if err := r.Get(ctx, secretName, secret); err != nil {
		return fmt.Errorf("failed to get s3-credentials secret: %w", err)
	}

	// Extract S3 endpoint from secret
	// Try AWS_ENDPOINT_URL_S3 first, then S3_ENDPOINT
	endpointURL := string(secret.Data["AWS_ENDPOINT_URL_S3"])
	if endpointURL == "" {
		endpointURL = string(secret.Data["S3_ENDPOINT"])
	}
	if endpointURL == "" {
		return fmt.Errorf("S3 endpoint not found in secret (expected AWS_ENDPOINT_URL_S3 or S3_ENDPOINT)")
	}

	// Extract S3 region from secret, fallback to us-east-1
	region := string(secret.Data["S3_REGION"])
	if region == "" {
		region = "us-east-1"
	}

	// Extract S3 credentials from secret
	accessKeyID := string(secret.Data["AWS_ACCESS_KEY_ID"])
	secretAccessKey := string(secret.Data["AWS_SECRET_ACCESS_KEY"])
	if accessKeyID == "" || secretAccessKey == "" {
		return fmt.Errorf("S3 credentials not found in secret (expected AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY)")
	}

	// Create static credentials provider
	staticCreds := credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")

	// Load AWS config with static credentials
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(staticCreds),
	)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client with custom endpoint and path-style addressing
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpointURL)
		o.UsePathStyle = true
	})

	prefix := aiModel.Spec.S3Key
	if !strings.HasSuffix(prefix, "/") {
		prefix = prefix + "/"
	}

	result, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  &aiModel.Spec.S3Bucket,
		Prefix:  &prefix,
		MaxKeys: aws.Int32(1), // We only need to know if at least one object exists
	})
	if err != nil {
		return fmt.Errorf("failed to check S3 artifacts: %w", err)
	}

	if len(result.Contents) == 0 {
		log.Info("S3 artifacts not found, will need to download", "bucket", aiModel.Spec.S3Bucket, "prefix", prefix)
		return fmt.Errorf("model artifacts not found at s3://%s/%s", aiModel.Spec.S3Bucket, prefix)
	}

	log.Info("S3 artifacts found", "bucket", aiModel.Spec.S3Bucket, "prefix", prefix, "objectCount", *result.KeyCount)
	return nil
}

// createOrUpdateInferenceService creates or updates a KServe InferenceService for the AIModel
func (r *AIModelReconciler) createOrUpdateInferenceService(ctx context.Context, aiModel *aimodelv1alpha1.AIModel) error {
	log := log.FromContext(ctx)

	// Determine runtime image based on spec
	runtimeImage := "vllm/vllm-openai:v0.6.3" // Default vLLM image
	if aiModel.Spec.Runtime == "tgi" {
		runtimeImage = "ghcr.io/huggingface/text-generation-inference:latest"
	} else if aiModel.Spec.Runtime == "triton" {
		runtimeImage = "nvcr.io/nvidia/tritonserver:latest"
	}

	// Get min/max replicas with defaults
	minReplicas := int32(0)
	if aiModel.Spec.MinReplicas != nil {
		minReplicas = *aiModel.Spec.MinReplicas
	}

	maxReplicas := int32(1)
	if aiModel.Spec.MaxReplicas != nil {
		maxReplicas = *aiModel.Spec.MaxReplicas
	}

	// Build resources with defaults if not specified
	resources := aiModel.Spec.Resources
	if len(resources.Requests) == 0 && len(resources.Limits) == 0 {
		resources = corev1.ResourceRequirements{
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
		}
	}

	// Build tolerations with defaults if not specified
	tolerations := aiModel.Spec.Tolerations
	if len(tolerations) == 0 {
		tolerations = []corev1.Toleration{
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
		}
	}

	// Build runtime args with defaults
	runtimeArgs := aiModel.Spec.RuntimeArgs
	if len(runtimeArgs) == 0 {
		runtimeArgs = []string{
			"--dtype=auto",
			"--max-model-len=4096",
			"--gpu-memory-utilization=0.9",
			"--trust-remote-code",
		}
	}

	// Build runtime env with S3 credentials
	runtimeEnv := aiModel.Spec.RuntimeEnv
	// Add S3 credentials and endpoint configuration
	runtimeEnv = append(runtimeEnv,
		corev1.EnvVar{
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
		corev1.EnvVar{
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
		corev1.EnvVar{
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
		corev1.EnvVar{
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
	)

	// Build the S3 model path
	modelID := fmt.Sprintf("s3://%s/%s", aiModel.Spec.S3Bucket, aiModel.Spec.S3Key)

	// Create owner reference
	ownerRef := &metav1.OwnerReference{
		APIVersion:         aiModel.APIVersion,
		Kind:               aiModel.Kind,
		Name:               aiModel.Name,
		UID:                aiModel.UID,
		Controller:         func() *bool { b := true; return &b }(),
		BlockOwnerDeletion: func() *bool { b := true; return &b }(),
	}

	// Build the InferenceService
	isvc, err := kserve.NewInferenceServiceBuilder(aiModel.Name, aiModel.Namespace).
		WithModelID(modelID).
		WithServedName(aiModel.Spec.ModelName).
		WithRuntime(runtimeImage).
		WithScaling(minReplicas, maxReplicas).
		WithResources(resources).
		WithTolerations(tolerations).
		WithNodeSelector(aiModel.Spec.NodeSelector).
		WithRuntimeArgs(runtimeArgs).
		WithRuntimeEnv(runtimeEnv).
		WithOwnerReference(ownerRef).
		Build()
	if err != nil {
		return fmt.Errorf("failed to build InferenceService: %w", err)
	}

	// Check if InferenceService already exists
	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(kserve.InferenceServiceGVK)
	err = r.Get(ctx, types.NamespacedName{Name: aiModel.Name, Namespace: aiModel.Namespace}, existing)

	if err != nil {
		if errors.IsNotFound(err) {
			// Create new InferenceService
			log.Info("Creating InferenceService", "name", aiModel.Name)
			if err := r.Create(ctx, isvc); err != nil {
				return fmt.Errorf("failed to create InferenceService: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to get InferenceService: %w", err)
	}

	// Update existing InferenceService
	log.Info("Updating InferenceService", "name", aiModel.Name)
	isvc.SetResourceVersion(existing.GetResourceVersion())
	if err := r.Update(ctx, isvc); err != nil {
		return fmt.Errorf("failed to update InferenceService: %w", err)
	}

	return nil
}

// updateStatusFromInferenceService updates the AIModel status based on the InferenceService status
func (r *AIModelReconciler) updateStatusFromInferenceService(ctx context.Context, aiModel *aimodelv1alpha1.AIModel) error {
	log := log.FromContext(ctx)

	// Get the InferenceService
	isvc := &unstructured.Unstructured{}
	isvc.SetGroupVersionKind(kserve.InferenceServiceGVK)

	err := r.Get(ctx, types.NamespacedName{Name: aiModel.Name, Namespace: aiModel.Namespace}, isvc)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("InferenceService not found, skipping status update", "name", aiModel.Name)
			return nil
		}
		return fmt.Errorf("failed to get InferenceService: %w", err)
	}

	// Extract status from InferenceService
	status, err := kserve.GetStatus(isvc)
	if err != nil {
		return fmt.Errorf("failed to extract InferenceService status: %w", err)
	}

	// Update AIModel status
	aiModel.Status.InferenceServiceName = aiModel.Name
	aiModel.Status.InferenceEndpoint = status.URL
	aiModel.Status.ReadyReplicas = status.ReadyReplicas

	// Update phase based on status
	if status.Ready {
		if aiModel.Status.Phase != aimodelv1alpha1.AIModelPhaseReady {
			log.Info("InferenceService is ready", "name", aiModel.Name, "url", status.URL)
			aiModel.Status.Phase = aimodelv1alpha1.AIModelPhaseReady
			aiModel.Status.Message = fmt.Sprintf("Model is ready at %s", status.URL)
		}
	} else {
		if aiModel.Status.Phase == aimodelv1alpha1.AIModelPhaseReady {
			// Transition from Ready to Deploying
			aiModel.Status.Phase = aimodelv1alpha1.AIModelPhaseDeploying
		}
		aiModel.Status.Message = status.String()
	}

	// Update status
	if err := r.Status().Update(ctx, aiModel); err != nil {
		return fmt.Errorf("failed to update AIModel status: %w", err)
	}

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
