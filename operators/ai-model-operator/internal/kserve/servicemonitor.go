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

package kserve

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ServiceMonitorGVK is the GroupVersionKind for Prometheus Operator ServiceMonitor
var ServiceMonitorGVK = schema.GroupVersionKind{
	Group:   "monitoring.coreos.com",
	Version: "v1",
	Kind:    "ServiceMonitor",
}

// ServiceMonitorBuilder provides a fluent API for building ServiceMonitor resources
type ServiceMonitorBuilder struct {
	name           string
	namespace      string
	modelName      string // Used for selector labels
	metricsPath    string
	metricsPort    string
	scrapeInterval string
	ownerRef       *metav1.OwnerReference
}

// NewServiceMonitorBuilder creates a new ServiceMonitorBuilder
func NewServiceMonitorBuilder(name, namespace string) *ServiceMonitorBuilder {
	return &ServiceMonitorBuilder{
		name:           name,
		namespace:      namespace,
		metricsPath:    "/metrics",
		metricsPort:    "http1",
		scrapeInterval: "15s",
	}
}

// WithModelName sets the model name used for the selector label
func (b *ServiceMonitorBuilder) WithModelName(name string) *ServiceMonitorBuilder {
	b.modelName = name
	return b
}

// WithMetricsPath sets the path for metrics scraping
func (b *ServiceMonitorBuilder) WithMetricsPath(path string) *ServiceMonitorBuilder {
	b.metricsPath = path
	return b
}

// WithMetricsPort sets the port name for metrics scraping
func (b *ServiceMonitorBuilder) WithMetricsPort(port string) *ServiceMonitorBuilder {
	b.metricsPort = port
	return b
}

// WithScrapeInterval sets the interval for metrics scraping
func (b *ServiceMonitorBuilder) WithScrapeInterval(interval string) *ServiceMonitorBuilder {
	b.scrapeInterval = interval
	return b
}

// WithOwnerReference sets the owner reference for garbage collection
func (b *ServiceMonitorBuilder) WithOwnerReference(ref *metav1.OwnerReference) *ServiceMonitorBuilder {
	b.ownerRef = ref
	return b
}

// Build constructs the unstructured ServiceMonitor resource
func (b *ServiceMonitorBuilder) Build() (*unstructured.Unstructured, error) {
	// Build labels for the ServiceMonitor
	// The "release: kube-prometheus-stack" label is required for Prometheus Operator
	// to discover this ServiceMonitor
	labels := map[string]interface{}{
		"release":    "kube-prometheus-stack",
		"managed-by": "ai-model-operator",
	}

	// Build the selector to match the KServe InferenceService
	// KServe creates a Service with label serving.kserve.io/inferenceservice=<model-name>
	selector := map[string]interface{}{
		"matchLabels": map[string]interface{}{
			"serving.kserve.io/inferenceservice": b.modelName,
		},
	}

	// Build the endpoint configuration
	endpoints := []interface{}{
		map[string]interface{}{
			"port":     b.metricsPort,
			"path":     b.metricsPath,
			"interval": b.scrapeInterval,
		},
	}

	// Build the unstructured object
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": ServiceMonitorGVK.GroupVersion().String(),
			"kind":       ServiceMonitorGVK.Kind,
			"metadata": map[string]interface{}{
				"name":      b.name,
				"namespace": b.namespace,
				"labels":    labels,
			},
			"spec": map[string]interface{}{
				"selector":  selector,
				"endpoints": endpoints,
			},
		},
	}

	// Add owner reference if provided
	if b.ownerRef != nil {
		obj.SetOwnerReferences([]metav1.OwnerReference{*b.ownerRef})
	}

	return obj, nil
}
