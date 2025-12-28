// Package config provides configuration management for the CLI.
package config

import "time"

// Timeout constants for various CLI operations.
//
// These values are chosen based on operational requirements:
//
// - API operations: Fast, should respond within seconds
// - Model downloads: Slow, models can be 10s of GB
// - Model deployments: Medium, includes container startup + health checks
// - Validation: Medium, includes cold start inference test
//
// Production experience shows:
// - 70B models take ~30-45min to download on 1Gbps connection
// - Deployment to ready state takes ~5-15min depending on GPU warmup
// - Inference validation needs 90s to handle cold start latency

const (
	// DefaultTimeout is used for standard API operations like listing models,
	// getting status, or simple CRUD operations. These should be fast.
	DefaultTimeout = 30 * time.Second

	// RegistryTimeout is used for model registration operations that may
	// involve HuggingFace API calls to validate model IDs.
	RegistryTimeout = 60 * time.Second

	// ValidationTimeout is used for model inference validation tests.
	// Needs to accommodate GPU cold start latency (model loading, CUDA init).
	ValidationTimeout = 5 * time.Minute

	// QuickDeployTimeout is used for deploying small models (<7B params)
	// or operations where the model is already cached and warmed up.
	QuickDeployTimeout = 5 * time.Minute

	// MediumDeployTimeout is used for deploying medium models (7B-13B params)
	// or operations involving cache rebuilds.
	MediumDeployTimeout = 10 * time.Minute

	// UpdateTimeout is used for model update operations that may involve
	// rolling restarts and gradual health check validation.
	UpdateTimeout = 15 * time.Minute

	// DeployTimeout is the default timeout for full model deployment operations.
	// Covers: model download, cache extraction, container start, GPU warmup,
	// health check stabilization. Sized for models up to 30B params.
	DeployTimeout = 30 * time.Minute

	// CacheTimeout is used for cache pull operations. Large models (70B+)
	// can be 20-40GB and may take 30-45min on 1Gbps connections.
	// We size this generously to avoid spurious timeouts.
	CacheTimeout = 2 * time.Hour

	// TroubleshootTimeout is used for troubleshooting operations that may
	// involve multiple diagnostic steps (log collection, pod inspection, etc).
	TroubleshootTimeout = 30 * time.Minute

	// SwapTimeout is used for model swap operations that coordinate
	// deployment of a new model and removal of the old one.
	SwapTimeout = 30 * time.Minute
)
