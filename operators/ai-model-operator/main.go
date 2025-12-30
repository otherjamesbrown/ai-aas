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

package main

import (
	"flag"
	"os"
	"time"

	// Import all Kubernetes client auth plugins (e.g., Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	aimodelv1alpha1 "github.com/ai-aas/ai-model-operator/api/v1alpha1"
	"github.com/ai-aas/ai-model-operator/controllers"
	"github.com/ai-aas/ai-model-operator/internal/adminapi"
	"github.com/ai-aas/ai-model-operator/internal/recipe"
	"github.com/ai-aas/ai-model-operator/internal/webhook"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(aimodelv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var leaderElectionID string
	var maxDownloadRetries int
	var initialRetryDelay time.Duration
	var maxRetryDelay time.Duration
	var downloaderImage string
	var defaultRuntime string
	var webhookPort int
	var enableWebhook bool

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&leaderElectionID, "leader-election-id", "98a4bc88.ai-aas.io", "The name of the resource that manages leader election")

	// Retry configuration flags
	flag.IntVar(&maxDownloadRetries, "max-download-retries", 5, "Maximum number of download retry attempts before marking as failed")
	flag.DurationVar(&initialRetryDelay, "initial-retry-delay", 1*time.Minute, "Initial delay before first retry (exponentially increases)")
	flag.DurationVar(&maxRetryDelay, "max-retry-delay", 16*time.Minute, "Maximum retry delay (caps exponential backoff)")

	// Image configuration flags
	flag.StringVar(&downloaderImage, "downloader-image", "python:3.11-slim", "Container image for model downloader job")
	flag.StringVar(&defaultRuntime, "default-runtime", "vllm", "Default runtime to use when not specified in AIModel (vllm, tgi, triton, or custom image)")

	// Webhook configuration flags
	flag.IntVar(&webhookPort, "webhook-port", 9443, "Port for webhook server")
	flag.BoolVar(&enableWebhook, "enable-webhook", true, "Enable validating webhook for AIModel resources")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Configure webhook server port if enabled
	var webhookServerOptions ctrl.Options
	if enableWebhook {
		webhookServerOptions = ctrl.Options{
			Scheme: scheme,
			Metrics: metricsserver.Options{
				BindAddress: metricsAddr,
			},
			WebhookServer: ctrl.NewWebhookServer(ctrl.WebhookServerOptions{
				Port: webhookPort,
			}),
			HealthProbeBindAddress:        probeAddr,
			LeaderElection:                enableLeaderElection,
			LeaderElectionID:              leaderElectionID,
			LeaderElectionReleaseOnCancel: true,
		}
	} else {
		webhookServerOptions = ctrl.Options{
			Scheme: scheme,
			Metrics: metricsserver.Options{
				BindAddress: metricsAddr,
			},
			HealthProbeBindAddress:        probeAddr,
			LeaderElection:                enableLeaderElection,
			LeaderElectionID:              leaderElectionID,
			LeaderElectionReleaseOnCancel: true,
		}
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), webhookServerOptions)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Initialize Admin API client if configured
	var adminAPIClient *adminapi.Client
	adminAPIBaseURL := os.Getenv("ADMIN_API_BASE_URL")
	adminAPIKey := os.Getenv("ADMIN_API_KEY")
	if adminAPIBaseURL != "" && adminAPIKey != "" {
		adminAPIClient = adminapi.NewClient(adminAPIBaseURL, adminAPIKey)
		setupLog.Info("Admin API client configured for deployment sync",
			"baseURL", adminAPIBaseURL)
	} else {
		setupLog.Info("Admin API client not configured - deployment sync disabled",
			"reason", "ADMIN_API_BASE_URL or ADMIN_API_KEY not set")
	}

	// Initialize recipe resolver and validator
	recipeResolver := recipe.NewResolver(mgr.GetClient())
	recipeValidator := recipe.NewValidator()

	if err = (&controllers.AIModelReconciler{
		Client:             mgr.GetClient(),
		Scheme:             mgr.GetScheme(),
		Recorder:           mgr.GetEventRecorderFor("ai-model-operator"),
		MaxDownloadRetries: int32(maxDownloadRetries),
		InitialRetryDelay:  initialRetryDelay,
		MaxRetryDelay:      maxRetryDelay,
		DownloaderImage:    downloaderImage,
		DefaultRuntime:     defaultRuntime,
		AdminAPIClient:     adminAPIClient,
		RecipeResolver:     recipeResolver,
		RecipeValidator:    recipeValidator,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "AIModel")
		os.Exit(1)
	}

	setupLog.Info("AIModel controller configuration",
		"maxDownloadRetries", maxDownloadRetries,
		"initialRetryDelay", initialRetryDelay,
		"maxRetryDelay", maxRetryDelay,
		"downloaderImage", downloaderImage,
		"defaultRuntime", defaultRuntime)

	// Register validating webhook if enabled
	if enableWebhook {
		if err = ctrl.NewWebhookManagedBy(mgr).
			For(&aimodelv1alpha1.AIModel{}).
			WithValidator(&webhook.AIModelValidator{
				Client: mgr.GetClient(),
			}).
			Complete(); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "AIModel")
			os.Exit(1)
		}
		setupLog.Info("AIModel validating webhook registered", "port", webhookPort)
	} else {
		setupLog.Info("AIModel validating webhook disabled")
	}
	//+kubebuilder:scaffold:builder

	if err = mgr.AddHealthzCheck("healthz", healthz.Ping);
		err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err = mgr.AddReadyzCheck("readyz", healthz.Ping);
		err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
