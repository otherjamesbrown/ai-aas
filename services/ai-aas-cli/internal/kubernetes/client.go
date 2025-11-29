// Package kubernetes provides Kubernetes client operations for model deployment.
package kubernetes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Client wraps the Kubernetes client with model management operations
type Client struct {
	clientset *kubernetes.Clientset
	config    *rest.Config
	namespace string
}

// ClientConfig configures the Kubernetes client
type ClientConfig struct {
	Kubeconfig string // Path to kubeconfig file
	Context    string // Kubernetes context to use
	Namespace  string // Default namespace
	InCluster  bool   // Use in-cluster config
}

// NewClient creates a new Kubernetes client
func NewClient(cfg ClientConfig) (*Client, error) {
	var config *rest.Config
	var err error

	if cfg.InCluster {
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("in-cluster config: %w", err)
		}
	} else {
		// Build kubeconfig path
		kubeconfig := cfg.Kubeconfig
		if kubeconfig == "" {
			if home := os.Getenv("HOME"); home != "" {
				kubeconfig = filepath.Join(home, ".kube", "config")
			}
		}

		// Load kubeconfig
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		loadingRules.ExplicitPath = kubeconfig

		configOverrides := &clientcmd.ConfigOverrides{}
		if cfg.Context != "" {
			configOverrides.CurrentContext = cfg.Context
		}

		kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			loadingRules,
			configOverrides,
		)

		config, err = kubeConfig.ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("load kubeconfig: %w", err)
		}

		// Get namespace from kubeconfig if not specified
		if cfg.Namespace == "" {
			namespace, _, err := kubeConfig.Namespace()
			if err == nil && namespace != "" {
				cfg.Namespace = namespace
			}
		}
	}

	// Set default namespace
	if cfg.Namespace == "" {
		cfg.Namespace = "default"
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create clientset: %w", err)
	}

	return &Client{
		clientset: clientset,
		config:    config,
		namespace: cfg.Namespace,
	}, nil
}

// Namespace returns the default namespace
func (c *Client) Namespace() string {
	return c.namespace
}

// SetNamespace sets the default namespace
func (c *Client) SetNamespace(ns string) {
	c.namespace = ns
}

// Clientset returns the underlying Kubernetes clientset
func (c *Client) Clientset() *kubernetes.Clientset {
	return c.clientset
}

// RESTConfig returns the underlying REST config
func (c *Client) RESTConfig() *rest.Config {
	return c.config
}

// Ping tests connectivity to the Kubernetes cluster
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.clientset.Discovery().ServerVersion()
	if err != nil {
		return fmt.Errorf("ping cluster: %w", err)
	}
	return nil
}

// GetServerVersion returns the Kubernetes server version
func (c *Client) GetServerVersion(ctx context.Context) (string, error) {
	version, err := c.clientset.Discovery().ServerVersion()
	if err != nil {
		return "", fmt.Errorf("get server version: %w", err)
	}
	return version.GitVersion, nil
}

