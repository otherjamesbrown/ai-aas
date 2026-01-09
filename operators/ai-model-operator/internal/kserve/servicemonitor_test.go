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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

func TestServiceMonitorBuilder_Build(t *testing.T) {
	tests := []struct {
		name           string
		builderSetup   func(*ServiceMonitorBuilder)
		wantName       string
		wantNamespace  string
		wantModelName  string
		wantPath       string
		wantPort       string
		wantInterval   string
		wantOwnerRef   bool
		wantOwnerUID   types.UID
	}{
		{
			name: "basic ServiceMonitor with defaults",
			builderSetup: func(b *ServiceMonitorBuilder) {
				b.WithModelName("test-model")
			},
			wantName:      "test-sm",
			wantNamespace: "default",
			wantModelName: "test-model",
			wantPath:      "/metrics",
			wantPort:      "http1",
			wantInterval:  "15s",
			wantOwnerRef:  false,
		},
		{
			name: "ServiceMonitor with custom metrics path",
			builderSetup: func(b *ServiceMonitorBuilder) {
				b.WithModelName("llama-7b").
					WithMetricsPath("/v1/metrics")
			},
			wantName:      "test-sm",
			wantNamespace: "default",
			wantModelName: "llama-7b",
			wantPath:      "/v1/metrics",
			wantPort:      "http1",
			wantInterval:  "15s",
			wantOwnerRef:  false,
		},
		{
			name: "ServiceMonitor with custom port and interval",
			builderSetup: func(b *ServiceMonitorBuilder) {
				b.WithModelName("gpt-oss-20b").
					WithMetricsPort("metrics").
					WithScrapeInterval("30s")
			},
			wantName:      "test-sm",
			wantNamespace: "default",
			wantModelName: "gpt-oss-20b",
			wantPath:      "/metrics",
			wantPort:      "metrics",
			wantInterval:  "30s",
			wantOwnerRef:  false,
		},
		{
			name: "ServiceMonitor with owner reference",
			builderSetup: func(b *ServiceMonitorBuilder) {
				b.WithModelName("mistral-7b").
					WithOwnerReference(&metav1.OwnerReference{
						APIVersion:         "aimodel.ai-aas.io/v1alpha1",
						Kind:               "AIModel",
						Name:               "mistral-7b",
						UID:                types.UID("test-uid-123"),
						Controller:         func() *bool { v := true; return &v }(),
						BlockOwnerDeletion: func() *bool { v := true; return &v }(),
					})
			},
			wantName:      "test-sm",
			wantNamespace: "default",
			wantModelName: "mistral-7b",
			wantPath:      "/metrics",
			wantPort:      "http1",
			wantInterval:  "15s",
			wantOwnerRef:  true,
			wantOwnerUID:  types.UID("test-uid-123"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewServiceMonitorBuilder("test-sm", "default")
			tt.builderSetup(builder)

			obj, err := builder.Build()
			if err != nil {
				t.Fatalf("Build() error = %v", err)
			}

			// Verify GVK
			gvk := obj.GetObjectKind().GroupVersionKind()
			if gvk != ServiceMonitorGVK {
				t.Errorf("GVK = %v, want %v", gvk, ServiceMonitorGVK)
			}

			// Verify metadata
			if obj.GetName() != tt.wantName {
				t.Errorf("Name = %v, want %v", obj.GetName(), tt.wantName)
			}
			if obj.GetNamespace() != tt.wantNamespace {
				t.Errorf("Namespace = %v, want %v", obj.GetNamespace(), tt.wantNamespace)
			}

			// Verify labels
			labels := obj.GetLabels()
			if labels["release"] != "kube-prometheus-stack" {
				t.Errorf("labels[release] = %v, want kube-prometheus-stack", labels["release"])
			}
			if labels["managed-by"] != "ai-model-operator" {
				t.Errorf("labels[managed-by] = %v, want ai-model-operator", labels["managed-by"])
			}

			// Verify selector
			selector, found, err := unstructured.NestedMap(obj.Object, "spec", "selector", "matchLabels")
			if err != nil || !found {
				t.Fatalf("selector.matchLabels not found")
			}
			if selector["serving.kserve.io/inferenceservice"] != tt.wantModelName {
				t.Errorf("selector label = %v, want %v", selector["serving.kserve.io/inferenceservice"], tt.wantModelName)
			}

			// Verify endpoints
			endpoints, found, err := unstructured.NestedSlice(obj.Object, "spec", "endpoints")
			if err != nil || !found || len(endpoints) == 0 {
				t.Fatalf("endpoints not found or empty")
			}

			endpoint, ok := endpoints[0].(map[string]interface{})
			if !ok {
				t.Fatalf("endpoint is not a map")
			}

			if endpoint["port"] != tt.wantPort {
				t.Errorf("endpoint.port = %v, want %v", endpoint["port"], tt.wantPort)
			}
			if endpoint["path"] != tt.wantPath {
				t.Errorf("endpoint.path = %v, want %v", endpoint["path"], tt.wantPath)
			}
			if endpoint["interval"] != tt.wantInterval {
				t.Errorf("endpoint.interval = %v, want %v", endpoint["interval"], tt.wantInterval)
			}

			// Verify owner reference
			ownerRefs := obj.GetOwnerReferences()
			if tt.wantOwnerRef {
				if len(ownerRefs) != 1 {
					t.Fatalf("expected 1 owner reference, got %d", len(ownerRefs))
				}
				if ownerRefs[0].UID != tt.wantOwnerUID {
					t.Errorf("ownerRef.UID = %v, want %v", ownerRefs[0].UID, tt.wantOwnerUID)
				}
			} else {
				if len(ownerRefs) != 0 {
					t.Errorf("expected no owner references, got %d", len(ownerRefs))
				}
			}
		})
	}
}

func TestServiceMonitorGVK(t *testing.T) {
	if ServiceMonitorGVK.Group != "monitoring.coreos.com" {
		t.Errorf("Group = %v, want monitoring.coreos.com", ServiceMonitorGVK.Group)
	}
	if ServiceMonitorGVK.Version != "v1" {
		t.Errorf("Version = %v, want v1", ServiceMonitorGVK.Version)
	}
	if ServiceMonitorGVK.Kind != "ServiceMonitor" {
		t.Errorf("Kind = %v, want ServiceMonitor", ServiceMonitorGVK.Kind)
	}
}
