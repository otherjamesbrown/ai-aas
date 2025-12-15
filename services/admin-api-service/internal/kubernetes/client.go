// Package kubernetes provides Kubernetes client operations for querying cluster resources.
package kubernetes

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AIModelGVR is the GroupVersionResource for AIModel CRD
var AIModelGVR = schema.GroupVersionResource{
	Group:    "aimodel.ai-aas.io",
	Version:  "v1alpha1",
	Resource: "aimodels",
}

// Client provides Kubernetes operations
type Client struct {
	dynamicClient dynamic.Interface
}

// NewClient creates a new Kubernetes client
// If kubeconfig is empty, uses in-cluster config
func NewClient(kubeconfig string) (*Client, error) {
	var config *rest.Config
	var err error

	if kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		config, err = rest.InClusterConfig()
	}
	if err != nil {
		return nil, fmt.Errorf("failed to build config: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	return &Client{
		dynamicClient: dynamicClient,
	}, nil
}

// AIModelDeployment represents an AIModel deployment reference
type AIModelDeployment struct {
	Name        string
	Namespace   string
	Environment string
	RecipeName  string
}

// ListAIModelsByRecipe lists all AIModel resources that reference a specific recipe
func (c *Client) ListAIModelsByRecipe(ctx context.Context, recipeName string) ([]AIModelDeployment, error) {
	// List all AIModels across all namespaces
	list, err := c.dynamicClient.Resource(AIModelGVR).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list AIModels: %w", err)
	}

	var deployments []AIModelDeployment

	for _, item := range list.Items {
		// Check if this AIModel has a recipeRef
		recipeRef, found, err := unstructured.NestedMap(item.Object, "spec", "recipeRef")
		if err != nil {
			continue // Skip items with invalid structure
		}
		if !found {
			continue // No recipeRef, skip
		}

		// Check if the recipe name matches
		name, ok := recipeRef["name"].(string)
		if !ok || name != recipeName {
			continue // Recipe name doesn't match
		}

		// Extract environment label or derive from namespace
		environment := ""
		labels := item.GetLabels()
		if labels != nil {
			if env, ok := labels["environment"]; ok {
				environment = env
			}
		}
		// If no environment label, derive from namespace
		if environment == "" {
			ns := item.GetNamespace()
			switch ns {
			case "development", "staging", "production":
				environment = ns
			default:
				environment = "unknown"
			}
		}

		deployments = append(deployments, AIModelDeployment{
			Name:        item.GetName(),
			Namespace:   item.GetNamespace(),
			Environment: environment,
			RecipeName:  recipeName,
		})
	}

	return deployments, nil
}
