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
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// ReconcileTotal is a Prometheus counter that tracks total reconciliations.
	ReconcileTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aimodel_reconcile_total",
			Help: "Total number of AIModel reconciliations",
		},
		[]string{"result"}, // "success", "error"
	)

	// ActiveAIModels is a Prometheus gauge that tracks the number of active (enabled) AIModels.
	ActiveAIModels = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "aimodel_active_count",
			Help: "Number of active AIModel resources",
		},
	)

	// AIModelStatusPhase is a Prometheus gauge that exposes the phase of each AIModel.
	AIModelStatusPhase = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aimodel_status_phase",
			Help: "Current phase of AIModel (1 for current phase, 0 otherwise)",
		},
		[]string{"name", "namespace", "phase"}, // "name" is AIModel.Name, "namespace" is AIModel.Namespace, "phase" is AIModel.Status.Phase
	)
)

func init() {
	// Register custom metrics with the global Prometheus registry
	metrics.Registry.MustRegister(ReconcileTotal, ActiveAIModels, AIModelStatusPhase)
}
