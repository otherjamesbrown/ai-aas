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
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	aimodelv1alpha1 "github.com/ai-aas/ai-model-operator/api/v1alpha1"
	"github.com/ai-aas/ai-model-operator/internal/adminapi"
	"github.com/ai-aas/ai-model-operator/internal/kserve"
	"github.com/ai-aas/ai-model-operator/internal/recipe"
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

// Retry configuration for InferenceService deployment failures
const (
	maxDeploymentRetries        = 10               // More retries for transient resource issues
	initialDeploymentRetryDelay = 30 * time.Second // Shorter initial delay for scheduling
	maxDeploymentRetryDelay     = 10 * time.Minute // Max wait between retries
)

// Deployment timeout configuration
const (
	defaultDeploymentTimeout = 30 * time.Minute // Max time for deployment before marking as Failed
)

// sanitizeInferenceServiceName converts an AIModel name to a KServe-compatible name.
// KServe InferenceService names must be DNS-compatible: lowercase alphanumeric and hyphens only,
// must start with a letter and end with alphanumeric. Periods are not allowed.
// Example: "llama-3.1-8b-instruct" -> "llama-3-1-8b-instruct"
func sanitizeInferenceServiceName(name string) string {
	// Replace periods with hyphens (most common issue)
	sanitized := strings.ReplaceAll(name, ".", "-")
	// Replace any consecutive hyphens with single hyphen
	for strings.Contains(sanitized, "--") {
		sanitized = strings.ReplaceAll(sanitized, "--", "-")
	}
	// Trim leading/trailing hyphens
	sanitized = strings.Trim(sanitized, "-")
	return sanitized
}

// deriveExternalName derives the external name for an AIModel.
// If spec.ExternalName is set, use it. Otherwise, derive from ModelID
// by taking the part after the last "/".
// Example: modelID "unsloth/gpt-oss-20b" -> externalName "gpt-oss-20b"
func deriveExternalName(aiModel *aimodelv1alpha1.AIModel) string {
	if aiModel.Spec.ExternalName != "" {
		return aiModel.Spec.ExternalName
	}
	// Derive from ModelID
	modelID := aiModel.Spec.ModelID
	if idx := strings.LastIndex(modelID, "/"); idx >= 0 {
		return modelID[idx+1:]
	}
	return modelID
}

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
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	// Download retry configuration
	MaxDownloadRetries int32
	InitialRetryDelay  time.Duration
	MaxRetryDelay      time.Duration

	// Deployment retry configuration
	MaxDeploymentRetries        int32
	InitialDeploymentRetryDelay time.Duration
	MaxDeploymentRetryDelay     time.Duration

	// Image configuration
	DownloaderImage string // Docker image for model downloader (default: "python:3.11-slim")
	DefaultRuntime  string // Default runtime if not specified in AIModel spec (default: "vllm")

	// Admin API client for deployment sync (optional - sync is disabled if nil)
	AdminAPIClient AdminAPIClient

	// Recipe resolution and validation
	RecipeResolver  *recipe.Resolver
	RecipeValidator *recipe.Validator
}

// AdminAPIClient defines the interface for syncing deployment state with the Admin API
type AdminAPIClient interface {
	CreateDeployment(ctx context.Context, req adminapi.CreateDeploymentRequest) error
	UpdateDeploymentStatus(ctx context.Context, modelName, environment string, status adminapi.DeploymentStatus) error
	DeleteDeployment(ctx context.Context, modelName, environment string) error
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

	// DEBUG: Log trustRemoteCode value as received from API
	log.Info("DEBUG: AIModel fetched from API",
		"name", aiModel.Name,
		"trustRemoteCode", aiModel.Spec.TrustRemoteCode,
		"modelID", aiModel.Spec.ModelID,
		"s3Bucket", aiModel.Spec.S3Bucket)

	// Set default phase if not already set
	if aiModel.Status.Phase == "" {
		aiModel.Status.Phase = "Pending"
		if err := r.Status().Update(ctx, aiModel); err != nil {
			log.Error(err, "unable to update AIModel status")
			reconcileTotal.WithLabelValues("error").Inc()
			return ctrl.Result{}, err
		}
	}

	// Sync modelType from spec to status
	if aiModel.Status.ModelType != aiModel.Spec.ModelType {
		aiModel.Status.ModelType = aiModel.Spec.ModelType
		if err := r.Status().Update(ctx, aiModel); err != nil {
			log.Error(err, "unable to update AIModel status modelType")
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

	// Validate spec: either S3 fields OR trustRemoteCode must be provided
	hasS3Config := aiModel.Spec.S3Bucket != "" && aiModel.Spec.S3Key != ""
	hasTrustRemoteCode := aiModel.Spec.TrustRemoteCode && aiModel.Spec.ModelID != ""
	if !hasS3Config && !hasTrustRemoteCode {
		log.Error(nil, "Invalid AIModel spec: must provide either S3Bucket+S3Key or TrustRemoteCode=true with ModelID",
			"name", aiModel.Name, "s3Bucket", aiModel.Spec.S3Bucket, "s3Key", aiModel.Spec.S3Key,
			"trustRemoteCode", aiModel.Spec.TrustRemoteCode, "modelID", aiModel.Spec.ModelID)
		aiModel.Status.Phase = aimodelv1alpha1.AIModelPhaseFailed
		aiModel.Status.Message = "Invalid spec: must provide either S3Bucket+S3Key for S3-based deployment, or TrustRemoteCode=true with ModelID for HuggingFace-based deployment"
		if err := r.Status().Update(ctx, aiModel); err != nil {
			log.Error(err, "unable to update AIModel status")
			reconcileTotal.WithLabelValues("error").Inc()
			return ctrl.Result{}, err
		}
		reconcileTotal.WithLabelValues("error").Inc()
		return ctrl.Result{}, nil // Don't requeue - user must fix the spec
	}

	// Handle model deletion if Enabled is false
	if !aiModel.Spec.Enabled {
		log.Info("AIModel is disabled, deleting InferenceService to release resources", "name", aiModel.Name)

		// Delete InferenceService to stop all pods and release GPU resources
		isvc := &unstructured.Unstructured{}
		isvc.SetGroupVersionKind(kserve.InferenceServiceGVK)
		err := r.Get(ctx, types.NamespacedName{Name: sanitizeInferenceServiceName(aiModel.Name), Namespace: aiModel.Namespace}, isvc)
		if err == nil {
			// InferenceService exists, delete it
			log.Info("Deleting InferenceService for disabled model", "name", aiModel.Name)
			if err := r.Delete(ctx, isvc); err != nil && !errors.IsNotFound(err) {
				log.Error(err, "Failed to delete InferenceService for disabled model")
				reconcileTotal.WithLabelValues("error").Inc()
				return ctrl.Result{}, err
			}
		} else if !errors.IsNotFound(err) {
			log.Error(err, "Failed to get InferenceService for disabled model")
			reconcileTotal.WithLabelValues("error").Inc()
			return ctrl.Result{}, err
		}

		// Update status to Disabled
		if aiModel.Status.Phase != "Disabled" {
			aiModel.Status.Phase = "Disabled"
			aiModel.Status.Message = "Model is disabled, InferenceService deleted"
			aiModel.Status.InferenceServiceName = ""
			aiModel.Status.InferenceEndpoint = ""
			aiModel.Status.ReadyReplicas = 0
			if err := r.Status().Update(ctx, aiModel); err != nil {
				log.Error(err, "unable to update AIModel status to Disabled")
				reconcileTotal.WithLabelValues("error").Inc()
				return ctrl.Result{}, err
			}
		}
		reconcileTotal.WithLabelValues("success").Inc()
		return ctrl.Result{}, nil
	}

	// 1. Reconcile Model Artifact Downloader Job (only for S3-based deployments)
	// For trustRemoteCode models, skip the downloader as they load directly from HuggingFace
	if aiModel.Spec.TrustRemoteCode && aiModel.Spec.ModelID != "" {
		// Skip downloader job - model will be loaded directly from HuggingFace
		log.Info("Skipping downloader job for trustRemoteCode model",
			"name", aiModel.Name, "modelID", aiModel.Spec.ModelID)
		// Set phase to Deploying to proceed to InferenceService creation
		if aiModel.Status.Phase != aimodelv1alpha1.AIModelPhaseDeploying &&
			aiModel.Status.Phase != aimodelv1alpha1.AIModelPhaseReady {
			setDeployingPhaseWithTimestamp(aiModel)
			if err := r.Status().Update(ctx, aiModel); err != nil {
				log.Error(err, "unable to update AIModel status to Deploying")
				reconcileTotal.WithLabelValues("error").Inc()
				return ctrl.Result{}, err
			}
		}
		// Fall through to InferenceService creation
	} else {
		// S3-based deployment - proceed with downloader job logic
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
			exists, err := r.checkS3ArtifactExists(ctx, aiModel)
			if err != nil {
				// Actual error (credentials, network, etc.) - not just missing artifacts
				log.Error(err, "Failed to check S3 artifacts", "Bucket", aiModel.Spec.S3Bucket, "Key", aiModel.Spec.S3Key)
				aiModel.Status.Phase = aimodelv1alpha1.AIModelPhaseFailed
				aiModel.Status.Message = fmt.Sprintf("S3 check failed: %v", err)
				if statusErr := r.Status().Update(ctx, aiModel); statusErr != nil {
					log.Error(statusErr, "unable to update AIModel status to Failed")
				}
				reconcileTotal.WithLabelValues("error").Inc()
				return ctrl.Result{Requeue: true}, nil
			}

			if exists {
				// Artifacts already exist in S3, skip download phase
				log.Info("S3 artifacts already exist, skipping download", "Bucket", aiModel.Spec.S3Bucket, "Key", aiModel.Spec.S3Key)
				setDeployingPhaseWithTimestamp(aiModel)
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
				if aiModel.Status.Phase == aimodelv1alpha1.AIModelPhaseDownloading || aiModel.Status.Phase == aimodelv1alpha1.AIModelPhasePending {
					setDeployingPhaseWithTimestamp(aiModel)
					// Reset retry tracking on success
					aiModel.Status.RetryCount = 0
					aiModel.Status.LastRetryTime = nil
					aiModel.Status.NextRetryTime = nil
					if err := r.Status().Update(ctx, aiModel); err != nil {
						log.Error(err, "unable to update AIModel status to Deploying")
						reconcileTotal.WithLabelValues("error").Inc()
						return ctrl.Result{}, err
					}
				}
			} else if jobIsFailed(foundJob) {
				log.Error(fmt.Errorf("job failed"), "Model Downloader Job failed", "Job.Name", jobName)

				// Check if we should retry
				maxRetries := r.MaxDownloadRetries
				if maxRetries == 0 {
					maxRetries = maxDownloadRetries // Use default if not configured
				}
				if aiModel.Status.RetryCount < maxRetries {
					// Delete the failed job so a new one can be created
					log.Info("Deleting failed job to prepare for retry", "Job.Name", jobName, "retryCount", aiModel.Status.RetryCount)
					if err := r.Delete(ctx, foundJob, client.PropagationPolicy(metav1.DeletePropagationForeground)); err != nil {
						log.Error(err, "Failed to delete failed Job", "Job.Name", jobName)
						reconcileTotal.WithLabelValues("error").Inc()
						return ctrl.Result{}, err
					}

					// Increment retry count and calculate backoff
					aiModel.Status.RetryCount++
					backoffDuration := r.calculateRetryBackoff(aiModel.Status.RetryCount - 1) // Use previous count for backoff
					now := metav1.Now()
					nextRetry := metav1.NewTime(now.Add(backoffDuration))

					aiModel.Status.Phase = aimodelv1alpha1.AIModelPhaseRetryPending
					aiModel.Status.LastRetryTime = &now
					aiModel.Status.NextRetryTime = &nextRetry
					aiModel.Status.Message = fmt.Sprintf("Download failed, retry %d/%d scheduled in %v",
						aiModel.Status.RetryCount, maxRetries, backoffDuration)

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
						"Job.Name", jobName, "maxRetries", maxRetries)
					aiModel.Status.Phase = aimodelv1alpha1.AIModelPhaseFailed
					aiModel.Status.Message = fmt.Sprintf("Download failed after %d retry attempts. Manual intervention required.", maxRetries)
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
	} // End of S3-based deployment else block

	// 2. Resolve and merge recipe if specified
	var mergedSpec *aimodelv1alpha1.ModelRecipeSpec
	if aiModel.Spec.RecipeRef != nil {
		log.Info("Resolving recipe reference", "name", aiModel.Spec.RecipeRef.Name, "namespace", aiModel.Spec.RecipeRef.Namespace)

		// Resolve the recipe
		modelRecipe, err := r.RecipeResolver.ResolveRecipe(ctx, aiModel.Spec.RecipeRef)
		if err != nil {
			log.Error(err, "Failed to resolve recipe", "recipeRef", aiModel.Spec.RecipeRef.Name)
			aiModel.Status.Phase = aimodelv1alpha1.AIModelPhaseFailed
			aiModel.Status.Message = fmt.Sprintf("Recipe resolution failed: %v", err)
			if statusErr := r.Status().Update(ctx, aiModel); statusErr != nil {
				log.Error(statusErr, "unable to update AIModel status to Failed")
			}
			reconcileTotal.WithLabelValues("error").Inc()
			// Requeue after delay to retry recipe resolution
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}

		// Merge recipe spec with overrides
		merged := recipe.MergeRecipe(modelRecipe.Spec, aiModel.Spec.Overrides)
		mergedSpec = &merged

		// Validate the merged spec
		validationResult := r.RecipeValidator.Validate(mergedSpec)
		if !validationResult.Valid {
			log.Error(nil, "Recipe validation failed", "errors", validationResult.Errors)
			aiModel.Status.Phase = aimodelv1alpha1.AIModelPhaseFailed
			aiModel.Status.Message = fmt.Sprintf("Recipe validation failed: %s", validationResult.ErrorString())
			if statusErr := r.Status().Update(ctx, aiModel); statusErr != nil {
				log.Error(statusErr, "unable to update AIModel status to Failed")
			}
			reconcileTotal.WithLabelValues("error").Inc()
			// Don't requeue - validation errors require user intervention
			return ctrl.Result{}, nil
		}

		log.Info("Recipe resolved and validated successfully", "recipe", aiModel.Spec.RecipeRef.Name)
	}

	// 3. Reconcile KServe InferenceService (replaces vLLM Deployment + Service)
	log.Info("Reconciling InferenceService", "name", aiModel.Name)

	// Create or update the InferenceService
	if err := r.createOrUpdateInferenceService(ctx, aiModel, mergedSpec); err != nil {
		log.Error(err, "Failed to reconcile InferenceService", "name", aiModel.Name)
		// Re-fetch AIModel before status update to avoid conflicts
		if fetchErr := r.Get(ctx, req.NamespacedName, aiModel); fetchErr != nil {
			log.Error(fetchErr, "Failed to re-fetch AIModel before status update")
			reconcileTotal.WithLabelValues("error").Inc()
			return ctrl.Result{}, fetchErr
		}
		aiModel.Status.Phase = aimodelv1alpha1.AIModelPhaseFailed
		aiModel.Status.Message = fmt.Sprintf("Failed to create InferenceService: %v", err)
		if statusErr := r.Status().Update(ctx, aiModel); statusErr != nil {
			log.Error(statusErr, "unable to update AIModel status to Failed")
		}
		reconcileTotal.WithLabelValues("error").Inc()
		return ctrl.Result{}, err
	}

	// Update status from InferenceService (this will check if already Ready and set appropriate phase)
	if err := r.updateStatusFromInferenceService(ctx, aiModel); err != nil {
		log.Error(err, "Failed to update status from InferenceService", "name", aiModel.Name)
		reconcileTotal.WithLabelValues("error").Inc()
		return ctrl.Result{}, err
	}

	// Re-fetch the AIModel to get the latest status after update
	// This is necessary because updateStatusFromInferenceService uses a fresh copy
	if err := r.Get(ctx, req.NamespacedName, aiModel); err != nil {
		log.Error(err, "Failed to re-fetch AIModel after status update", "name", aiModel.Name)
		reconcileTotal.WithLabelValues("error").Inc()
		return ctrl.Result{}, err
	}

	// Check for deployment timeout (only applies to Deploying phase)
	timedOut, err := r.checkDeploymentTimeout(ctx, aiModel)
	if err != nil {
		log.Error(err, "Failed to check deployment timeout", "name", aiModel.Name)
		// Don't fail reconciliation on timeout check error - continue
	}
	if timedOut {
		// Deployment has exceeded timeout - transition to Failed
		aiModel.Status.Phase = aimodelv1alpha1.AIModelPhaseFailed
		aiModel.Status.LastFailureReason = "Deployment timeout exceeded"
		aiModel.Status.Message = fmt.Sprintf("Deployment timed out after %v. Manual intervention required.", defaultDeploymentTimeout)

		// Emit Kubernetes Event
		r.Recorder.Event(aiModel, corev1.EventTypeWarning, "PhaseTimeout",
			fmt.Sprintf("Deployment stuck in Deploying phase for %v, exceeded timeout of %v",
				time.Since(aiModel.Status.DeploymentStartedAt.Time), defaultDeploymentTimeout))

		if err := r.Status().Update(ctx, aiModel); err != nil {
			log.Error(err, "Failed to update AIModel status to Failed after timeout")
			reconcileTotal.WithLabelValues("error").Inc()
			return ctrl.Result{}, err
		}

		log.Info("Deployment timeout - marked as Failed", "name", aiModel.Name, "timeout", defaultDeploymentTimeout)
		reconcileTotal.WithLabelValues("timeout").Inc()
		return ctrl.Result{}, nil // Don't requeue - manual intervention needed
	}

	// Handle deployment retry pending state
	if aiModel.Status.Phase == aimodelv1alpha1.AIModelPhaseRetryPending {
		if aiModel.Status.NextRetryTime != nil {
			now := metav1.Now()
			if now.Time.Before(aiModel.Status.NextRetryTime.Time) {
				// Not yet time to retry, requeue with remaining wait time
				waitDuration := aiModel.Status.NextRetryTime.Time.Sub(now.Time)
				log.Info("Deployment retry not yet due, waiting",
					"waitDuration", waitDuration,
					"retryCount", aiModel.Status.RetryCount)
				reconcileTotal.WithLabelValues("success").Inc()
				return ctrl.Result{RequeueAfter: waitDuration}, nil
			}
			// Time to retry - transition back to Deploying phase
			log.Info("Deployment retry time reached, retrying",
				"retryCount", aiModel.Status.RetryCount)
			setDeployingPhaseWithTimestamp(aiModel)
			aiModel.Status.Message = fmt.Sprintf("Retrying deployment (attempt %d)", aiModel.Status.RetryCount)
			if err := r.Status().Update(ctx, aiModel); err != nil {
				log.Error(err, "unable to update AIModel status to Deploying for retry")
				reconcileTotal.WithLabelValues("error").Inc()
				return ctrl.Result{}, err
			}
			// Requeue immediately to trigger reconciliation in Deploying phase
			reconcileTotal.WithLabelValues("success").Inc()
			return ctrl.Result{Requeue: true}, nil
		}
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

	// Delete deployment record from Admin API
	if err := r.deleteDeploymentFromAdminAPI(ctx, aiModel); err != nil {
		// Log error but don't fail finalization - Admin API sync is best-effort
		log.Error(err, "Failed to delete deployment from Admin API", "name", aiModel.Name)
	}

	// Delete InferenceService
	isvc := &unstructured.Unstructured{}
	isvc.SetGroupVersionKind(kserve.InferenceServiceGVK)
	if err := r.Get(ctx, client.ObjectKey{Name: sanitizeInferenceServiceName(aiModel.Name), Namespace: aiModel.Namespace}, isvc); err == nil {
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
	// Uses multipart upload with retries for resilience against network issues
	downloadScript := `
pip install -q huggingface_hub boto3 &&
python3 -c "
from huggingface_hub import snapshot_download
import boto3
from boto3.s3.transfer import TransferConfig
from botocore.config import Config
import os
import time

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

# Configure boto3 with retries and timeouts
boto_config = Config(
    retries={'max_attempts': 10, 'mode': 'adaptive'},
    connect_timeout=30,
    read_timeout=60,
)

s3_config = {'config': boto_config}
if s3_endpoint:
    s3_config['endpoint_url'] = s3_endpoint
    print(f'Using custom S3 endpoint: {s3_endpoint}')

s3 = boto3.client('s3', **s3_config)

# Configure multipart upload: 8MB chunks, max 4 concurrent uploads
transfer_config = TransferConfig(
    multipart_threshold=8 * 1024 * 1024,  # 8MB
    multipart_chunksize=8 * 1024 * 1024,  # 8MB chunks
    max_concurrency=4,
    use_threads=True,
)

def upload_with_retry(local_path, bucket, key, max_retries=3):
    for attempt in range(max_retries):
        try:
            s3.upload_file(local_path, bucket, key, Config=transfer_config)
            return True
        except Exception as e:
            if attempt < max_retries - 1:
                wait_time = (attempt + 1) * 5
                print(f'    Retry {attempt + 1}/{max_retries} after {wait_time}s: {e}')
                time.sleep(wait_time)
            else:
                raise

for root, dirs, files in os.walk(local_dir):
    for file in files:
        local_path = os.path.join(root, file)
        rel_path = os.path.relpath(local_path, local_dir)
        s3_path = f'{s3_key}/{rel_path}'
        file_size = os.path.getsize(local_path)
        size_mb = file_size / (1024 * 1024)
        print(f'  Uploading {rel_path} ({size_mb:.1f} MB)...')
        upload_with_retry(local_path, s3_bucket, s3_path)
print('Upload complete!')
"
`

	// Determine downloader image (use configured value or default)
	downloaderImage := r.DownloaderImage
	if downloaderImage == "" {
		downloaderImage = modelDownloaderImage
	}

	// Determine command and args based on image type
	// Go-based model-downloader uses entrypoint, Python image needs shell script
	var command []string
	var args []string
	if strings.Contains(downloaderImage, "model-downloader") {
		// Go binary - uses ENTRYPOINT, no command/args needed
		command = nil
		args = nil
	} else {
		// Python image - run embedded script via shell
		command = []string{"/bin/sh", "-c"}
		args = []string{downloadScript}
	}

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
						Name:    "downloader",
						Image:   downloaderImage,
						Command: command,
						Args:    args,
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
					// Add imagePullSecrets for private container registries (GHCR)
					ImagePullSecrets: []corev1.LocalObjectReference{
						{Name: "ghcr-pull-secret"},
					},
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

// calculateRetryBackoff calculates the exponential backoff duration for download retry.
// Returns min(initialRetryDelay * 2^retryCount, maxRetryDelay)
// Uses configured values from the reconciler, falling back to defaults if not set.
func (r *AIModelReconciler) calculateRetryBackoff(retryCount int32) time.Duration {
	initial := r.InitialRetryDelay
	if initial == 0 {
		initial = initialRetryDelay // Use default
	}
	max := r.MaxRetryDelay
	if max == 0 {
		max = maxRetryDelay // Use default
	}

	backoff := initial * time.Duration(1<<retryCount)
	if backoff > max {
		return max
	}
	return backoff
}

// calculateDeploymentRetryBackoff calculates the exponential backoff duration for deployment retry.
// Returns min(initialDeploymentRetryDelay * 2^retryCount, maxDeploymentRetryDelay)
// Uses configured values from the reconciler, falling back to defaults if not set.
func (r *AIModelReconciler) calculateDeploymentRetryBackoff(retryCount int32) time.Duration {
	initial := r.InitialDeploymentRetryDelay
	if initial == 0 {
		initial = initialDeploymentRetryDelay // Use default
	}
	max := r.MaxDeploymentRetryDelay
	if max == 0 {
		max = maxDeploymentRetryDelay // Use default
	}

	backoff := initial * time.Duration(1<<retryCount)
	if backoff > max {
		return max
	}
	return backoff
}

func (r *AIModelReconciler) checkS3ArtifactExists(ctx context.Context, aiModel *aimodelv1alpha1.AIModel) (bool, error) {
	log := log.FromContext(ctx)

	// Read S3 credentials secret to get endpoint URL
	secret := &corev1.Secret{}
	secretName := types.NamespacedName{
		Name:      "s3-credentials",
		Namespace: aiModel.Namespace,
	}
	if err := r.Get(ctx, secretName, secret); err != nil {
		return false, fmt.Errorf("failed to get s3-credentials secret: %w", err)
	}

	// Extract S3 endpoint from secret
	// Try AWS_ENDPOINT_URL_S3 first, then S3_ENDPOINT
	endpointURL := string(secret.Data["AWS_ENDPOINT_URL_S3"])
	if endpointURL == "" {
		endpointURL = string(secret.Data["S3_ENDPOINT"])
	}
	if endpointURL == "" {
		return false, fmt.Errorf("S3 endpoint not found in secret (expected AWS_ENDPOINT_URL_S3 or S3_ENDPOINT)")
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
		return false, fmt.Errorf("S3 credentials not found in secret (expected AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY)")
	}

	// Create static credentials provider
	staticCreds := credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")

	// Load AWS config with static credentials
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(staticCreds),
	)
	if err != nil {
		return false, fmt.Errorf("failed to load AWS config: %w", err)
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
		// Network/connection errors are treated as "unknown - assume missing"
		// This allows the download job to proceed and attempt to fetch the model
		log.Info("Failed to check S3 artifacts, assuming they don't exist", "bucket", aiModel.Spec.S3Bucket, "prefix", prefix, "error", err)
		return false, nil
	}

	if len(result.Contents) == 0 {
		log.Info("S3 artifacts not found, will need to download", "bucket", aiModel.Spec.S3Bucket, "prefix", prefix)
		return false, nil // Missing artifacts is not an error - just return false
	}

	log.Info("S3 artifacts found", "bucket", aiModel.Spec.S3Bucket, "prefix", prefix, "objectCount", *result.KeyCount)
	return true, nil
}

// createOrUpdateInferenceService creates or updates a KServe InferenceService for the AIModel
// If recipeSpec is provided, it uses the recipe configuration; otherwise uses inline AIModel spec
func (r *AIModelReconciler) createOrUpdateInferenceService(ctx context.Context, aiModel *aimodelv1alpha1.AIModel, recipeSpec *aimodelv1alpha1.ModelRecipeSpec) error {
	log := log.FromContext(ctx)

	// Determine runtime to use (from recipe, spec, or default)
	runtime := ""
	if recipeSpec != nil {
		runtime = recipeSpec.Runtime
	} else if aiModel.Spec.Runtime != "" {
		runtime = aiModel.Spec.Runtime
	} else {
		// Use configured default runtime or fallback to "vllm"
		runtime = r.DefaultRuntime
		if runtime == "" {
			runtime = "vllm"
		}
	}

	// Get min/max replicas (from overrides in recipe, or inline spec, or defaults)
	minReplicas := int32(0)
	maxReplicas := int32(1)

	if recipeSpec != nil && aiModel.Spec.Overrides != nil && aiModel.Spec.Overrides.Replicas != nil {
		// Use override values if specified
		if aiModel.Spec.Overrides.Replicas.Min != nil {
			minReplicas = *aiModel.Spec.Overrides.Replicas.Min
		}
		if aiModel.Spec.Overrides.Replicas.Max != nil {
			maxReplicas = *aiModel.Spec.Overrides.Replicas.Max
		}
	} else {
		// Use inline spec values
		if aiModel.Spec.MinReplicas != nil {
			minReplicas = *aiModel.Spec.MinReplicas
		}
		if aiModel.Spec.MaxReplicas != nil {
			maxReplicas = *aiModel.Spec.MaxReplicas
		}
	}

	// Build resources (from recipe or inline spec)
	var resources corev1.ResourceRequirements
	if recipeSpec != nil {
		// Convert recipe resources to Kubernetes ResourceRequirements
		resources = r.convertRecipeResources(&recipeSpec.Resources)
	} else {
		// Use inline spec resources
		resources = aiModel.Spec.Resources
		if len(resources.Requests) == 0 && len(resources.Limits) == 0 {
			// Apply defaults if nothing specified
			resources = corev1.ResourceRequirements{
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
			}
		}
	}

	// Build tolerations (from recipe scheduling or inline spec)
	var tolerations []corev1.Toleration
	if recipeSpec != nil && len(recipeSpec.Scheduling.Tolerations) > 0 {
		tolerations = recipeSpec.Scheduling.Tolerations
	} else if len(aiModel.Spec.Tolerations) > 0 {
		tolerations = aiModel.Spec.Tolerations
	} else {
		// Default tolerations
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

	// Build node selector (from recipe scheduling or inline spec)
	var nodeSelector map[string]string
	if recipeSpec != nil && len(recipeSpec.Scheduling.NodeSelector) > 0 {
		nodeSelector = recipeSpec.Scheduling.NodeSelector
	} else {
		nodeSelector = aiModel.Spec.NodeSelector
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

	// Build the S3 storage URI
	storageUri := fmt.Sprintf("s3://%s/%s", aiModel.Spec.S3Bucket, aiModel.Spec.S3Key)

	// Determine the model format and runtime name based on the runtime
	modelFormat := "vllm"
	runtimeName := "vllm-runtime"

	// Check if a custom runtime name is specified
	if aiModel.Spec.RuntimeName != "" {
		runtimeName = aiModel.Spec.RuntimeName
		// For custom runtime names, use default vllm modelFormat unless specified otherwise
		// Users can override this by explicitly setting the runtime field
	} else {
		// Map runtime to appropriate KServe ClusterServingRuntime name
		switch runtime {
		case "vllm":
			modelFormat = "vllm"
			runtimeName = "vllm-runtime"
		case "tgi":
			modelFormat = "huggingface"
			runtimeName = "tgi-runtime"
		case "triton":
			modelFormat = "tensorrt"
			runtimeName = "kserve-tritonserver"
		case "tensorrt-llm":
			modelFormat = "tensorrt-llm"
			runtimeName = "kserve-tensorrt-llm"
		default:
			// For custom runtimes, use the runtime value as the runtime name
			runtimeName = runtime
		}
	}

	// Build runtime args (from recipe or inline spec)
	var runtimeArgs []string
	if recipeSpec != nil {
		// Convert recipe runtime args to command-line args
		runtimeArgs = r.convertRecipeRuntimeArgs(runtime, &recipeSpec.RuntimeArgs)
	} else {
		// Use inline spec runtime args
		runtimeArgs = aiModel.Spec.RuntimeArgs
		if len(runtimeArgs) == 0 {
			// Use default vLLM args if none specified
			runtimeArgs = []string{
				"--dtype=float16",
				"--max-model-len=4096",
				"--gpu-memory-utilization=0.9",
			}
		}
	}

	// Add --trust-remote-code flag if TrustRemoteCode is enabled in AIModel spec
	// This allows per-deployment override of trust_remote_code even when using recipes
	if aiModel.Spec.TrustRemoteCode {
		// Check if the flag is not already present to avoid duplicates
		hasFlag := false
		for _, arg := range runtimeArgs {
			if arg == "--trust-remote-code" {
				hasFlag = true
				break
			}
		}
		if !hasFlag {
			runtimeArgs = append(runtimeArgs, "--trust-remote-code")
		}
	}

	// Create owner reference
	ownerRef := &metav1.OwnerReference{
		APIVersion:         aiModel.APIVersion,
		Kind:               aiModel.Kind,
		Name:               aiModel.Name,
		UID:                aiModel.UID,
		Controller:         func() *bool { b := true; return &b }(),
		BlockOwnerDeletion: func() *bool { b := true; return &b }(),
	}

	var isvc *unstructured.Unstructured
	var err error

	// For models with trust_remote_code, use container-based deployment
	// that loads directly from HuggingFace instead of S3. This is required
	// because vLLM needs to download and execute custom Python model code
	// Determine update strategy (default to RollingUpdate if not specified)
	updateStrategy := string(aiModel.Spec.UpdateStrategy)
	if updateStrategy == "" {
		updateStrategy = string(aimodelv1alpha1.UpdateStrategyRollingUpdate)
	}

	// Get health check configuration from recipe or set runtime-aware defaults
	livenessPath := "/health"
	readinessPath := "/health"
	probePort := int32(8000)

	// Use recipe health check config if available
	if recipeSpec != nil && recipeSpec.HealthCheck.LivenessPath != "" {
		livenessPath = recipeSpec.HealthCheck.LivenessPath
	}
	if recipeSpec != nil && recipeSpec.HealthCheck.ReadinessPath != "" {
		readinessPath = recipeSpec.HealthCheck.ReadinessPath
	}

	// Set runtime-aware defaults if recipe doesn't specify paths
	if recipeSpec == nil || recipeSpec.HealthCheck.LivenessPath == "" {
		switch runtime {
		case "triton", "tensorrt-llm":
			livenessPath = "/v2/health/live"
			readinessPath = "/v2/health/ready"
			probePort = 8080
		}
	}

	// Determine deployment mode
	deploymentMode := r.determineDeploymentMode(aiModel, recipeSpec)

	// Log the decision for observability
	log.Info("Deployment mode determined",
		"mode", deploymentMode,
		"aimodel", aiModel.Name,
		"runtime", runtime)

	// from the HuggingFace repo, which isn't preserved in S3 storage.
	if aiModel.Spec.TrustRemoteCode && aiModel.Spec.ModelID != "" {
		log.Info("Using container-based deployment for trust_remote_code model",
			"name", aiModel.Name, "modelID", aiModel.Spec.ModelID, "runtime", runtime)

		// Select container image based on runtime
		var containerImage string
		switch runtime {
		case "tgi":
			containerImage = "ghcr.io/huggingface/text-generation-inference:2.4"
		default:
			// Use vLLM image - v0.10.2 has native support for GPT-OSS and other modern architectures
			containerImage = "vllm/vllm-openai:v0.10.2"
		}

		// Determine liveness probe initial delay (default 300s if not specified)
		livenessInitialDelay := int32(300)
		if aiModel.Spec.ProbeConfig != nil && aiModel.Spec.ProbeConfig.InitialDelaySeconds != nil {
			livenessInitialDelay = *aiModel.Spec.ProbeConfig.InitialDelaySeconds
		}

		// Use the HuggingFace model ID as the served name so vLLM accepts requests
		// with the full HF ID (e.g., "unsloth/gpt-oss-20b"). This allows multiple
		// models with the same base name but different sources to coexist.
		isvc, err = kserve.NewInferenceServiceBuilder(sanitizeInferenceServiceName(aiModel.Name), aiModel.Namespace).
			WithContainerImage(containerImage).
			WithContainerRuntime(runtime).
			WithModelID(aiModel.Spec.ModelID).
			WithServedName(aiModel.Spec.ModelID).
			WithDeploymentMode(deploymentMode).
			WithScaling(minReplicas, maxReplicas).
			WithResources(resources).
			WithTolerations(tolerations).
			WithNodeSelector(nodeSelector).
			WithRuntimeArgs(runtimeArgs).
			WithRuntimeEnv(runtimeEnv).
			WithUpdateStrategy(updateStrategy).
			WithLivenessInitialDelay(livenessInitialDelay).
			WithHealthProbes(livenessPath, readinessPath, probePort).
			WithOwnerReference(ownerRef).
			BuildContainerBased()
		if err != nil {
			return fmt.Errorf("failed to build container-based InferenceService: %w", err)
		}
	} else {
		// Build the InferenceService using KServe native model serving
		// This leverages KServe's storage initializer to download from S3
		// Use the HuggingFace model ID as the served name to allow coexistence
		// of models with the same base name from different sources.
		servedName := aiModel.Spec.ModelID
		if servedName == "" {
			servedName = aiModel.Spec.ModelName // Fallback for legacy models
		}
		isvc, err = kserve.NewInferenceServiceBuilder(sanitizeInferenceServiceName(aiModel.Name), aiModel.Namespace).
			WithStorageUri(storageUri).
			WithModelFormat(modelFormat).
			WithServedName(servedName).
			WithRuntime(runtimeName).
			WithDeploymentMode(deploymentMode).
			WithScaling(minReplicas, maxReplicas).
			WithResources(resources).
			WithTolerations(tolerations).
			WithNodeSelector(nodeSelector).
			WithRuntimeArgs(runtimeArgs).
			WithRuntimeEnv(runtimeEnv).
			WithUpdateStrategy(updateStrategy).
			WithHealthProbes(livenessPath, readinessPath, probePort).
			WithOwnerReference(ownerRef).
			Build()
		if err != nil {
			return fmt.Errorf("failed to build InferenceService: %w", err)
		}
	}

	// Check if InferenceService already exists
	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(kserve.InferenceServiceGVK)
	isvcName := sanitizeInferenceServiceName(aiModel.Name)
	err = r.Get(ctx, types.NamespacedName{Name: isvcName, Namespace: aiModel.Namespace}, existing)

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

	// Check probe config version annotation to detect when probe configuration
	// in the operator code has changed. This forces updates even if the AIModel
	// spec hasn't changed, ensuring existing InferenceServices get updated probe config.
	existingAnnotations := existing.GetAnnotations()
	desiredAnnotations := isvc.GetAnnotations()
	existingProbeVersion := existingAnnotations["ai-aas.io/probe-config-version"]
	desiredProbeVersion := desiredAnnotations["ai-aas.io/probe-config-version"]

	probeConfigChanged := existingProbeVersion != desiredProbeVersion
	if probeConfigChanged {
		log.Info("Probe config version changed, forcing update",
			"name", aiModel.Name,
			"existingVersion", existingProbeVersion,
			"desiredVersion", desiredProbeVersion)
	}

	// Compare spec hashes to determine if an update is needed.
	// This approach ignores server-side mutations (like KServe adding default fields)
	// and only triggers updates when the operator's desired spec actually changes.
	specHashMatches := kserve.SpecHashMatches(isvc, existing)

	// Skip update if spec hash matches AND probe config version is the same
	if !probeConfigChanged && specHashMatches {
		log.Info("InferenceService spec hash and probe config unchanged, skipping update",
			"name", aiModel.Name,
			"specHash", kserve.GetSpecHash(isvc))
		return nil
	}

	// Log what changed for debugging
	if !specHashMatches {
		log.Info("InferenceService spec hash changed, update required",
			"name", aiModel.Name,
			"existingHash", kserve.GetSpecHash(existing),
			"desiredHash", kserve.GetSpecHash(isvc))
	}

	// Update existing InferenceService with retry on conflict
	updateReason := "spec changed"
	if probeConfigChanged {
		updateReason = "probe config version changed"
	}
	log.Info("Updating InferenceService", "name", aiModel.Name, "reason", updateReason)

	// Retry loop with exponential backoff to handle race conditions with KServe controller
	backoff := wait.Backoff{
		Steps:    5,                     // Maximum 5 attempts
		Duration: 50 * time.Millisecond, // Initial delay: 50ms
		Factor:   2.0,                   // Exponential: 50ms, 100ms, 200ms, 400ms, 800ms
		Jitter:   0.1,                   // Add 10% jitter to avoid thundering herd
	}

	var lastErr error
	retryCount := 0

	err = wait.ExponentialBackoff(backoff, func() (bool, error) {
		// Fetch the latest version of the InferenceService
		latest := &unstructured.Unstructured{}
		latest.SetGroupVersionKind(kserve.InferenceServiceGVK)
		if err := r.Get(ctx, types.NamespacedName{Name: isvcName, Namespace: aiModel.Namespace}, latest); err != nil {
			// Permanent error - don't retry
			lastErr = fmt.Errorf("failed to get InferenceService: %w", err)
			return false, lastErr
		}

		// Set the resource version from the latest fetch
		isvc.SetResourceVersion(latest.GetResourceVersion())

		// Attempt the update
		if err := r.Update(ctx, isvc); err != nil {
			if errors.IsConflict(err) {
				// Conflict error - retry after backoff
				retryCount++
				log.Info("InferenceService update conflict, retrying",
					"name", aiModel.Name,
					"attempt", retryCount,
					"maxAttempts", backoff.Steps)
				lastErr = err
				return false, nil // Retry
			}
			// Other error - don't retry
			lastErr = fmt.Errorf("failed to update InferenceService: %w", err)
			return false, lastErr
		}

		// Success
		if retryCount > 0 {
			log.Info("InferenceService update succeeded after retries",
				"name", aiModel.Name,
				"attempts", retryCount+1)
		}
		return true, nil
	})

	if err != nil {
		if lastErr != nil {
			if errors.IsConflict(lastErr) {
				return fmt.Errorf("failed to update InferenceService after %d attempts due to conflicts: %w", retryCount, lastErr)
			}
			return lastErr
		}
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
	isvcName := sanitizeInferenceServiceName(aiModel.Name)

	err := r.Get(ctx, types.NamespacedName{Name: isvcName, Namespace: aiModel.Namespace}, isvc)
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

	// Re-fetch AIModel to get latest resourceVersion before status update
	// This prevents optimistic concurrency conflicts when multiple reconciliations happen
	latestAIModel := &aimodelv1alpha1.AIModel{}
	if err := r.Get(ctx, types.NamespacedName{Name: aiModel.Name, Namespace: aiModel.Namespace}, latestAIModel); err != nil {
		return fmt.Errorf("failed to re-fetch AIModel before status update: %w", err)
	}

	// Save original status for comparison to prevent infinite reconcile loops
	originalStatus := latestAIModel.Status.DeepCopy()

	// Update AIModel status fields (use sanitized name for InferenceService)
	latestAIModel.Status.InferenceServiceName = isvcName
	latestAIModel.Status.InferenceEndpoint = status.URL
	latestAIModel.Status.ReadyReplicas = status.ReadyReplicas

	// Update phase based on InferenceService status
	if status.Ready {
		wasNotReady := latestAIModel.Status.Phase != aimodelv1alpha1.AIModelPhaseReady
		if wasNotReady {
			log.Info("InferenceService is ready, transitioning to Ready phase", "name", aiModel.Name, "url", status.URL)
			latestAIModel.Status.Phase = aimodelv1alpha1.AIModelPhaseReady
			latestAIModel.Status.Message = fmt.Sprintf("Model is ready at %s", status.URL)
			// Reset retry tracking and deployment timeout tracking on success
			latestAIModel.Status.RetryCount = 0
			latestAIModel.Status.LastRetryTime = nil
			latestAIModel.Status.NextRetryTime = nil
			latestAIModel.Status.DeploymentStartedAt = nil // Clear deployment start time

			// Sync deployment state to Admin API only on transition to Ready
			// This prevents hammering the API with PUT requests on every reconcile
			if err := r.syncDeploymentToAdminAPI(ctx, latestAIModel, status); err != nil {
				// Log error but don't fail reconciliation - Admin API sync is best-effort
				log.Error(err, "Failed to sync deployment to Admin API", "name", aiModel.Name)
			}
		}
	} else {
		// InferenceService is not ready - determine if it's a transient or permanent failure

		// Check for transient failures (retryable)
		if status.IsTransientFailure() {
			log.Info("InferenceService has transient failure, will retry",
				"name", aiModel.Name,
				"status", status.String(),
				"retryCount", latestAIModel.Status.RetryCount)

			// Get max retries configuration
			maxRetries := r.MaxDeploymentRetries
			if maxRetries == 0 {
				maxRetries = int32(maxDeploymentRetries) // Use default
			}

			// Check if we should retry
			if latestAIModel.Status.RetryCount < maxRetries {
				// Only increment retry count and schedule retry if we're in Deploying phase
				// (not if we're already in RetryPending waiting for the timer)
				if latestAIModel.Status.Phase == aimodelv1alpha1.AIModelPhaseDeploying {
					latestAIModel.Status.RetryCount++
					backoffDuration := r.calculateDeploymentRetryBackoff(latestAIModel.Status.RetryCount - 1)
					now := metav1.Now()
					nextRetry := metav1.NewTime(now.Add(backoffDuration))

					latestAIModel.Status.Phase = aimodelv1alpha1.AIModelPhaseRetryPending
					latestAIModel.Status.LastRetryTime = &now
					latestAIModel.Status.NextRetryTime = &nextRetry
					latestAIModel.Status.Message = fmt.Sprintf("Transient deployment failure (retry %d/%d scheduled in %v): %s",
						latestAIModel.Status.RetryCount, maxRetries, backoffDuration, status.String())

					log.Info("Deployment retry scheduled",
						"retryCount", latestAIModel.Status.RetryCount,
						"backoffDuration", backoffDuration,
						"nextRetryTime", nextRetry.Time)
				} else {
					// Already in RetryPending, keep the existing message
					latestAIModel.Status.Message = fmt.Sprintf("Waiting for retry %d/%d: %s",
						latestAIModel.Status.RetryCount, maxRetries, status.String())
				}
			} else {
				// Max retries exceeded for transient failure
				log.Error(fmt.Errorf("max deployment retries exceeded"),
					"InferenceService failed after maximum retries",
					"name", aiModel.Name,
					"maxRetries", maxRetries,
					"status", status.String())
				latestAIModel.Status.Phase = aimodelv1alpha1.AIModelPhaseFailed
				latestAIModel.Status.Message = fmt.Sprintf("Deployment failed after %d retry attempts: %s. Manual intervention required.",
					maxRetries, status.String())
			}
		} else if status.IsPermanentFailure() {
			// Permanent failure - don't retry
			log.Error(fmt.Errorf("permanent failure detected"),
				"InferenceService has permanent failure",
				"name", aiModel.Name,
				"status", status.String())
			latestAIModel.Status.Phase = aimodelv1alpha1.AIModelPhaseFailed
			latestAIModel.Status.Message = fmt.Sprintf("Permanent deployment failure: %s. Manual intervention required.", status.String())
		} else {
			// Not ready, but not clearly a failure yet (e.g., still creating)
			// Only transition to Deploying if not already in a terminal or retry state
			if latestAIModel.Status.Phase != aimodelv1alpha1.AIModelPhaseDeploying &&
				latestAIModel.Status.Phase != aimodelv1alpha1.AIModelPhaseFailed &&
				latestAIModel.Status.Phase != aimodelv1alpha1.AIModelPhaseRetryPending {
				log.Info("InferenceService not ready yet, setting phase to Deploying",
					"name", aiModel.Name,
					"currentPhase", latestAIModel.Status.Phase,
					"status", status.String())
				setDeployingPhaseWithTimestamp(latestAIModel)
			}
			latestAIModel.Status.Message = status.String()
		}
	}

	// Only update status if something actually changed to prevent infinite reconcile loops
	if equality.Semantic.DeepEqual(originalStatus, &latestAIModel.Status) {
		log.V(1).Info("Status unchanged, skipping update to prevent infinite reconcile loop", "name", aiModel.Name)
		return nil
	}

	log.V(1).Info("Status changed, performing update",
		"name", aiModel.Name,
		"oldPhase", originalStatus.Phase,
		"newPhase", latestAIModel.Status.Phase)

	// Update status with retry on conflict using exponential backoff
	// This is critical - without proper retry, the phase can get stuck
	statusBackoff := wait.Backoff{
		Steps:    5,
		Duration: 50 * time.Millisecond,
		Factor:   2.0,
		Jitter:   0.1,
	}

	var lastStatusErr error
	statusRetryCount := 0

	err = wait.ExponentialBackoff(statusBackoff, func() (bool, error) {
		// On retry, re-fetch the latest AIModel to get the current resourceVersion
		if statusRetryCount > 0 {
			freshAIModel := &aimodelv1alpha1.AIModel{}
			if fetchErr := r.Get(ctx, types.NamespacedName{Name: aiModel.Name, Namespace: aiModel.Namespace}, freshAIModel); fetchErr != nil {
				lastStatusErr = fmt.Errorf("failed to re-fetch AIModel for status retry: %w", fetchErr)
				return false, lastStatusErr // Permanent error - stop retrying
			}

			// Reapply the status changes to the fresh object
			freshAIModel.Status.InferenceServiceName = latestAIModel.Status.InferenceServiceName
			freshAIModel.Status.InferenceEndpoint = latestAIModel.Status.InferenceEndpoint
			freshAIModel.Status.ReadyReplicas = latestAIModel.Status.ReadyReplicas
			freshAIModel.Status.Phase = latestAIModel.Status.Phase
			freshAIModel.Status.Message = latestAIModel.Status.Message
			freshAIModel.Status.RetryCount = latestAIModel.Status.RetryCount
			freshAIModel.Status.LastRetryTime = latestAIModel.Status.LastRetryTime
			freshAIModel.Status.NextRetryTime = latestAIModel.Status.NextRetryTime
			freshAIModel.Status.DeploymentStartedAt = latestAIModel.Status.DeploymentStartedAt
			freshAIModel.Status.LastFailureReason = latestAIModel.Status.LastFailureReason

			latestAIModel = freshAIModel
		}

		if updateErr := r.Status().Update(ctx, latestAIModel); updateErr != nil {
			if errors.IsConflict(updateErr) {
				statusRetryCount++
				log.Info("Status update conflict, retrying",
					"name", aiModel.Name,
					"attempt", statusRetryCount,
					"maxAttempts", statusBackoff.Steps)
				lastStatusErr = updateErr
				return false, nil // Retry
			}
			lastStatusErr = fmt.Errorf("failed to update AIModel status: %w", updateErr)
			return false, lastStatusErr // Permanent error - stop retrying
		}

		// Success
		if statusRetryCount > 0 {
			log.Info("Status update succeeded after retries",
				"name", aiModel.Name,
				"attempts", statusRetryCount+1,
				"oldPhase", originalStatus.Phase,
				"newPhase", latestAIModel.Status.Phase)
		} else {
			log.Info("Status update succeeded",
				"name", aiModel.Name,
				"oldPhase", originalStatus.Phase,
				"newPhase", latestAIModel.Status.Phase)
		}
		return true, nil
	})

	if err != nil {
		if lastStatusErr != nil {
			if errors.IsConflict(lastStatusErr) {
				// All retries exhausted - return conflict error so reconciler retries
				return fmt.Errorf("status update failed after %d attempts due to conflicts: %w", statusRetryCount, lastStatusErr)
			}
			return lastStatusErr
		}
		return fmt.Errorf("status update failed: %w", err)
	}

	return nil
}

// syncDeploymentToAdminAPI syncs the deployment state to the Admin API database.
// This function is best-effort and errors are logged but don't fail reconciliation.
func (r *AIModelReconciler) syncDeploymentToAdminAPI(ctx context.Context, aiModel *aimodelv1alpha1.AIModel, status *kserve.InferenceServiceStatus) error {
	// Skip if Admin API client is not configured
	if r.AdminAPIClient == nil {
		return nil
	}

	log := log.FromContext(ctx)

	// Determine environment from namespace
	// Convention: namespace format is "system" or "ai-aas-<environment>"
	environment := "development"
	if aiModel.Namespace != "system" {
		environment = aiModel.Namespace
	}

	// Map AIModel phase to deployment status
	deploymentStatus := "pending"
	switch aiModel.Status.Phase {
	case aimodelv1alpha1.AIModelPhaseReady:
		deploymentStatus = "ready"
	case aimodelv1alpha1.AIModelPhaseDeploying:
		deploymentStatus = "deploying"
	case aimodelv1alpha1.AIModelPhaseFailed:
		deploymentStatus = "failed"
	case aimodelv1alpha1.AIModelPhaseDisabled:
		deploymentStatus = "disabled"
	}

	// Try to update existing deployment first
	err := r.AdminAPIClient.UpdateDeploymentStatus(ctx, aiModel.Name, environment, adminapi.DeploymentStatus{
		Status:               deploymentStatus,
		ModelID:              aiModel.Spec.ModelID,
		ExternalName:         deriveExternalName(aiModel),
		InferenceServiceName: aiModel.Status.InferenceServiceName,
		Endpoint:             status.URL,
		ReplicasReady:        int(status.ReadyReplicas),
	})

	// If deployment doesn't exist (404), create it
	if err != nil && (contains(err.Error(), "404") || contains(err.Error(), "not found")) {
		log.Info("Deployment record not found in Admin API, creating", "name", aiModel.Name, "environment", environment)

		// Get min replicas for the create request
		replicas := 1
		if aiModel.Spec.MinReplicas != nil {
			replicas = int(*aiModel.Spec.MinReplicas)
		}

		createErr := r.AdminAPIClient.CreateDeployment(ctx, adminapi.CreateDeploymentRequest{
			ModelName:    aiModel.Name,
			ModelID:      aiModel.Spec.ModelID,
			ExternalName: deriveExternalName(aiModel),
			Environment:  environment,
			Namespace:    aiModel.Namespace,
			Replicas:     replicas,
		})
		if createErr != nil {
			return fmt.Errorf("failed to create deployment: %w", createErr)
		}

		// After creating, update with current status
		return r.AdminAPIClient.UpdateDeploymentStatus(ctx, aiModel.Name, environment, adminapi.DeploymentStatus{
			Status:               deploymentStatus,
			ModelID:              aiModel.Spec.ModelID,
			ExternalName:         deriveExternalName(aiModel),
			InferenceServiceName: aiModel.Status.InferenceServiceName,
			Endpoint:             status.URL,
			ReplicasReady:        int(status.ReadyReplicas),
		})
	}

	return err
}

// deleteDeploymentFromAdminAPI removes the deployment record from the Admin API database.
// This function is best-effort and errors are logged but don't fail finalization.
func (r *AIModelReconciler) deleteDeploymentFromAdminAPI(ctx context.Context, aiModel *aimodelv1alpha1.AIModel) error {
	// Skip if Admin API client is not configured
	if r.AdminAPIClient == nil {
		return nil
	}

	// Determine environment from namespace
	environment := "development"
	if aiModel.Namespace != "system" {
		environment = aiModel.Namespace
	}

	return r.AdminAPIClient.DeleteDeployment(ctx, aiModel.Name, environment)
}

// contains is a helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// convertRecipeResources converts recipe resources to Kubernetes ResourceRequirements
func (r *AIModelReconciler) convertRecipeResources(recipeRes *aimodelv1alpha1.RecipeResources) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{},
		Limits:   corev1.ResourceList{},
	}

	// GPU resources
	gpuCount := fmt.Sprintf("%d", recipeRes.GPU.Count)
	gpuResourceName := corev1.ResourceName(fmt.Sprintf("%s.com/gpu", recipeRes.GPU.Vendor))
	if recipeRes.GPU.Vendor == "" || recipeRes.GPU.Vendor == "nvidia" {
		gpuResourceName = "nvidia.com/gpu"
	}
	resources.Requests[gpuResourceName] = resource.MustParse(gpuCount)
	resources.Limits[gpuResourceName] = resource.MustParse(gpuCount)

	// CPU resources
	if recipeRes.CPU.Requests != "" {
		resources.Requests[corev1.ResourceCPU] = resource.MustParse(recipeRes.CPU.Requests)
	}
	if recipeRes.CPU.Limits != "" {
		resources.Limits[corev1.ResourceCPU] = resource.MustParse(recipeRes.CPU.Limits)
	}

	// Memory resources
	if recipeRes.Memory.Requests != "" {
		resources.Requests[corev1.ResourceMemory] = resource.MustParse(recipeRes.Memory.Requests)
	}
	if recipeRes.Memory.Limits != "" {
		resources.Limits[corev1.ResourceMemory] = resource.MustParse(recipeRes.Memory.Limits)
	}

	return resources
}

// convertRecipeRuntimeArgs converts recipe runtime args to command-line arguments
func (r *AIModelReconciler) convertRecipeRuntimeArgs(runtime string, runtimeArgs *aimodelv1alpha1.RuntimeArgsSpec) []string {
	var args []string

	switch runtime {
	case "vllm":
		if runtimeArgs.VLLM != nil {
			vllmArgs := runtimeArgs.VLLM
			if vllmArgs.DType != "" {
				args = append(args, fmt.Sprintf("--dtype=%s", vllmArgs.DType))
			}
			if vllmArgs.MaxModelLen > 0 {
				args = append(args, fmt.Sprintf("--max-model-len=%d", vllmArgs.MaxModelLen))
			}
			if vllmArgs.GPUMemoryUtilization != "" {
				args = append(args, fmt.Sprintf("--gpu-memory-utilization=%s", vllmArgs.GPUMemoryUtilization))
			}
			if vllmArgs.TrustRemoteCode {
				args = append(args, "--trust-remote-code")
			}
			if vllmArgs.TokenizerMode != "" {
				args = append(args, fmt.Sprintf("--tokenizer-mode=%s", vllmArgs.TokenizerMode))
			}
			// Append any extra args
			args = append(args, vllmArgs.ExtraArgs...)
		}
	case "tgi":
		if runtimeArgs.TGI != nil {
			tgiArgs := runtimeArgs.TGI
			if tgiArgs.Quantize != "" {
				args = append(args, fmt.Sprintf("--quantize=%s", tgiArgs.Quantize))
			}
			if tgiArgs.MaxInputLength > 0 {
				args = append(args, fmt.Sprintf("--max-input-length=%d", tgiArgs.MaxInputLength))
			}
			if tgiArgs.MaxTotalTokens > 0 {
				args = append(args, fmt.Sprintf("--max-total-tokens=%d", tgiArgs.MaxTotalTokens))
			}
			if tgiArgs.MaxBatchPrefillTokens > 0 {
				args = append(args, fmt.Sprintf("--max-batch-prefill-tokens=%d", tgiArgs.MaxBatchPrefillTokens))
			}
			if tgiArgs.NumShard > 0 {
				args = append(args, fmt.Sprintf("--num-shard=%d", tgiArgs.NumShard))
			}
			if tgiArgs.DisableFlashAttention {
				args = append(args, "--disable-flash-attention")
			}
		}
	case "triton", "tensorrt-llm":
		if runtimeArgs.Triton != nil {
			tritonArgs := runtimeArgs.Triton

			// Model repository path - can be set via command-line or env var
			// The ClusterServingRuntime sets --model-store=/mnt/models as default
			// For now, we don't override it here as it's typically handled by KServe
			// However, if needed in the future, we could add:
			// if tritonArgs.ModelRepository != "" {
			//     args = append(args, fmt.Sprintf("--model-repository=%s", tritonArgs.ModelRepository))
			// }

			// Backend configuration
			// Triton backends are configured via config.pbtxt in the model repository
			// Dynamic batching, instance groups, and tensor configs are also in config.pbtxt
			// These are not command-line arguments to tritonserver

			// For TensorRT-LLM backend, additional configuration may be needed
			if tritonArgs.Backend == "tensorrt" {
				// TensorRT-LLM specific args would go here if needed
				// Currently, these are handled by the model repository config
			}

			// Note: Most Triton configuration is done through:
			// 1. ClusterServingRuntime (server-level args like --http-port, --grpc-port)
			// 2. Model repository config.pbtxt (model-level config)
			// 3. Environment variables (set via runtimeEnv in the spec)
			// This function returns empty args for Triton, which is correct behavior
		}
	}

	return args
}

// determineDeploymentMode determines the deployment mode for an AIModel.
// Priority:
// 1. AIModel.Spec.DeploymentMode (explicit override)
// 2. Recipe.Spec.DeploymentMode (if recipe is used)
// 3. Runtime-based default:
//   - tensorrt-llm, triton → RawDeployment
//   - vllm, tgi with GPU → RawDeployment
//   - vllm, tgi without GPU → Serverless
//   - default → Serverless
func (r *AIModelReconciler) determineDeploymentMode(
	aimodel *aimodelv1alpha1.AIModel,
	recipe *aimodelv1alpha1.ModelRecipeSpec,
) string {
	// 1. AIModel explicit override
	if aimodel.Spec.DeploymentMode != "" {
		return aimodel.Spec.DeploymentMode
	}

	// 2. Recipe explicit setting
	if recipe != nil && recipe.DeploymentMode != "" {
		return recipe.DeploymentMode
	}

	// 3. Runtime-based defaults
	runtime := aimodel.Spec.Runtime
	if recipe != nil && recipe.Runtime != "" {
		runtime = recipe.Runtime
	}

	switch runtime {
	case "tensorrt-llm", "triton":
		return "RawDeployment"
	case "vllm", "tgi":
		// Check if GPU is requested
		if r.hasGPU(aimodel, recipe) {
			return "RawDeployment"
		}
		return "Serverless"
	default:
		return "Serverless"
	}
}

// hasGPU checks if GPU resources are requested in the AIModel or recipe.
func (r *AIModelReconciler) hasGPU(
	aimodel *aimodelv1alpha1.AIModel,
	recipe *aimodelv1alpha1.ModelRecipeSpec,
) bool {
	// Check AIModel resources (both requests and limits) for any GPU request
	// Supports nvidia.com/gpu, amd.com/gpu, intel.com/gpu, and other vendors
	if hasGPUResource(aimodel.Spec.Resources.Requests) ||
		hasGPUResource(aimodel.Spec.Resources.Limits) {
		return true
	}

	// Check recipe GPU resources
	if recipe != nil && recipe.Resources.GPU.Count > 0 {
		return true
	}

	return false
}

// hasGPUResource checks if a resource list contains a non-zero GPU request.
func hasGPUResource(resources corev1.ResourceList) bool {
	for resourceName, quantity := range resources {
		if strings.Contains(string(resourceName), "gpu") && !quantity.IsZero() {
			return true
		}
	}
	return false
}

// setDeployingPhaseWithTimestamp sets the phase to Deploying and records the start time.
// Only sets the timestamp if transitioning FROM a different phase TO Deploying.
// Allows Ready → Deploying transition when InferenceService becomes unhealthy.
func setDeployingPhaseWithTimestamp(aiModel *aimodelv1alpha1.AIModel) bool {
	// Don't transition if already in Deploying phase
	if aiModel.Status.Phase == aimodelv1alpha1.AIModelPhaseDeploying {
		return false // Already in Deploying phase
	}
	now := metav1.Now()
	aiModel.Status.Phase = aimodelv1alpha1.AIModelPhaseDeploying
	aiModel.Status.DeploymentStartedAt = &now
	return true // Phase changed
}

// checkDeploymentTimeout checks if the deployment has exceeded the timeout.
// Returns true if timeout exceeded, false otherwise.
func (r *AIModelReconciler) checkDeploymentTimeout(ctx context.Context, aiModel *aimodelv1alpha1.AIModel) (bool, error) {
	// Only check timeout when in Deploying phase
	if aiModel.Status.Phase != aimodelv1alpha1.AIModelPhaseDeploying {
		return false, nil
	}

	// No timeout if deploymentStartedAt is not set (shouldn't happen, but be defensive)
	if aiModel.Status.DeploymentStartedAt == nil {
		return false, nil
	}

	// Calculate elapsed time since deployment started
	elapsed := time.Since(aiModel.Status.DeploymentStartedAt.Time)
	timeout := defaultDeploymentTimeout

	// Check if timeout exceeded
	if elapsed > timeout {
		log := log.FromContext(ctx)
		log.Info("Deployment timeout exceeded",
			"name", aiModel.Name,
			"elapsed", elapsed,
			"timeout", timeout,
			"deploymentStartedAt", aiModel.Status.DeploymentStartedAt.Time)
		return true, nil
	}

	return false, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AIModelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&aimodelv1alpha1.AIModel{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
