package gitops

import (
	"context"
	"fmt"
	"strings"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/kubernetes"
	"gopkg.in/yaml.v3"
)

// DeployOptions configures a GitOps deployment.
type DeployOptions struct {
	ModelName    string
	Environment  string
	AIModelCfg   kubernetes.AIModelConfig
	AutoPush     bool   // If true, automatically push after commit
	CommitPrefix string // Optional prefix for commit message
}

// DeleteOptions configures a GitOps deletion.
type DeleteOptions struct {
	ModelName   string
	Environment string
	AutoPush    bool // If true, automatically push after commit
}

// DeployResult contains the result of a GitOps deployment.
type DeployResult struct {
	ManifestPath string // Relative path to the manifest file
	Branch       string // Git branch used
	Committed    bool   // Whether a commit was created
	Pushed       bool   // Whether the commit was pushed
	CommitMsg    string // The commit message used
}

// Deployer handles GitOps-based model deployments.
type Deployer struct {
	client *Client
}

// NewDeployer creates a new GitOps deployer.
func NewDeployer(client *Client) *Deployer {
	return &Deployer{client: client}
}

// Deploy deploys a model by committing its manifest to the config repository.
func (d *Deployer) Deploy(ctx context.Context, opts DeployOptions) (*DeployResult, error) {
	result := &DeployResult{}

	// Determine the branch for this environment
	branch := BranchForEnvironment(opts.Environment)
	result.Branch = branch

	// Checkout the correct branch
	fmt.Printf("Checking out branch: %s\n", branch)
	if err := d.client.CheckoutBranch(ctx, branch); err != nil {
		return nil, fmt.Errorf("checkout branch: %w", err)
	}

	// Check if manifest already exists
	if d.client.ModelManifestExists(opts.Environment, opts.ModelName) {
		return nil, fmt.Errorf("model %s already exists in %s. Use 'model undeploy' first or update the manifest directly",
			opts.ModelName, opts.Environment)
	}

	// Generate the manifest YAML
	manifest, err := generateAIModelManifest(opts.AIModelCfg)
	if err != nil {
		return nil, fmt.Errorf("generate manifest: %w", err)
	}

	// Write manifest to repository
	manifestPath, err := d.client.WriteModelManifest(opts.Environment, opts.ModelName, manifest)
	if err != nil {
		return nil, fmt.Errorf("write manifest: %w", err)
	}
	result.ManifestPath = manifestPath
	fmt.Printf("Created manifest: %s\n", manifestPath)

	// Stage the file
	if err := d.client.StageFile(ctx, manifestPath); err != nil {
		return nil, fmt.Errorf("stage file: %w", err)
	}

	// Create commit
	commitMsg := fmt.Sprintf("deploy: %s to %s", opts.ModelName, opts.Environment)
	if opts.CommitPrefix != "" {
		commitMsg = fmt.Sprintf("%s %s", opts.CommitPrefix, commitMsg)
	}
	result.CommitMsg = commitMsg

	if err := d.client.Commit(ctx, commitMsg); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	result.Committed = true
	fmt.Printf("Created commit: %s\n", commitMsg)

	// Push if requested
	if opts.AutoPush {
		fmt.Printf("Pushing to origin/%s...\n", branch)
		if err := d.client.Push(ctx); err != nil {
			return nil, fmt.Errorf("push: %w", err)
		}
		result.Pushed = true
		fmt.Println("Pushed successfully.")
	}

	return result, nil
}

// Undeploy removes a model by deleting its manifest from the config repository.
func (d *Deployer) Undeploy(ctx context.Context, opts DeleteOptions) (*DeployResult, error) {
	result := &DeployResult{}

	// Determine the branch for this environment
	branch := BranchForEnvironment(opts.Environment)
	result.Branch = branch

	// Checkout the correct branch
	fmt.Printf("Checking out branch: %s\n", branch)
	if err := d.client.CheckoutBranch(ctx, branch); err != nil {
		return nil, fmt.Errorf("checkout branch: %w", err)
	}

	// Check if manifest exists
	if !d.client.ModelManifestExists(opts.Environment, opts.ModelName) {
		return nil, fmt.Errorf("model %s not found in %s", opts.ModelName, opts.Environment)
	}

	// Delete the manifest
	manifestPath, err := d.client.DeleteModelManifest(opts.Environment, opts.ModelName)
	if err != nil {
		return nil, fmt.Errorf("delete manifest: %w", err)
	}
	result.ManifestPath = manifestPath
	fmt.Printf("Deleted manifest: %s\n", manifestPath)

	// Stage the deletion
	if err := d.client.StageDeleted(ctx, manifestPath); err != nil {
		return nil, fmt.Errorf("stage deletion: %w", err)
	}

	// Create commit
	commitMsg := fmt.Sprintf("undeploy: %s from %s", opts.ModelName, opts.Environment)
	result.CommitMsg = commitMsg

	if err := d.client.Commit(ctx, commitMsg); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	result.Committed = true
	fmt.Printf("Created commit: %s\n", commitMsg)

	// Push if requested
	if opts.AutoPush {
		fmt.Printf("Pushing to origin/%s...\n", branch)
		if err := d.client.Push(ctx); err != nil {
			return nil, fmt.Errorf("push: %w", err)
		}
		result.Pushed = true
		fmt.Println("Pushed successfully.")
	}

	return result, nil
}

// generateAIModelManifest generates YAML for an AIModel custom resource.
// This is similar to the function in deploy.go but includes additional fields
// that match the format used in ai-aas-config.
func generateAIModelManifest(cfg kubernetes.AIModelConfig) ([]byte, error) {
	// Build labels
	labels := map[string]interface{}{
		"app":         "vllm-inference",
		"model":       extractModelFamily(cfg.ModelName),
		"environment": cfg.Environment,
	}
	for k, v := range cfg.Labels {
		labels[k] = v
	}

	// Build spec based on recipe or inline configuration
	spec := map[string]interface{}{}

	if cfg.RecipeName != "" {
		// Recipe-based configuration
		recipeRef := map[string]interface{}{
			"name": cfg.RecipeName,
		}
		if cfg.RecipeNamespace != "" {
			recipeRef["namespace"] = cfg.RecipeNamespace
		} else {
			recipeRef["namespace"] = "ai-model-system"
		}
		spec["recipeRef"] = recipeRef

		// Add overrides if any flags were explicitly set
		overrides := map[string]interface{}{}
		if cfg.OverrideGPUCount || cfg.OverrideMemory {
			resourceOverrides := map[string]interface{}{}
			if cfg.OverrideGPUCount {
				resourceOverrides["gpu"] = map[string]interface{}{
					"count": cfg.GPUCount,
				}
			}
			if cfg.OverrideMemory {
				resourceOverrides["memory"] = map[string]interface{}{
					"requests": fmt.Sprintf("%dGi", cfg.MemoryGB),
				}
			}
			overrides["resources"] = resourceOverrides
		}
		if cfg.OverrideMinReplicas || cfg.OverrideMaxReplicas {
			replicaOverrides := map[string]interface{}{}
			if cfg.OverrideMinReplicas {
				replicaOverrides["min"] = cfg.MinReplicas
			}
			if cfg.OverrideMaxReplicas {
				replicaOverrides["max"] = cfg.MaxReplicas
			}
			overrides["replicas"] = replicaOverrides
		}
		if len(overrides) > 0 {
			spec["overrides"] = overrides
		}
	} else {
		// Inline configuration
		spec["modelName"] = cfg.ModelName
		spec["modelID"] = cfg.ModelID
		spec["enabled"] = cfg.Enabled
		spec["runtime"] = cfg.Runtime

		if cfg.MinReplicas > 0 {
			spec["minReplicas"] = cfg.MinReplicas
		}
		if cfg.MaxReplicas > 0 {
			spec["maxReplicas"] = cfg.MaxReplicas
		}

		// Resources
		resources := map[string]interface{}{
			"requests": map[string]interface{}{
				"cpu":              "4",
				"memory":           fmt.Sprintf("%dGi", cfg.MemoryGB/2),
				"nvidia.com/gpu":   fmt.Sprintf("%d", cfg.GPUCount),
			},
			"limits": map[string]interface{}{
				"cpu":            "8",
				"memory":         fmt.Sprintf("%dGi", cfg.MemoryGB),
				"nvidia.com/gpu": fmt.Sprintf("%d", cfg.GPUCount),
			},
		}
		spec["resources"] = resources

		// Add tolerations for GPU nodes
		spec["tolerations"] = []map[string]interface{}{
			{
				"key":      "nvidia.com/gpu",
				"operator": "Exists",
				"effect":   "NoSchedule",
			},
		}

		// Add runtime args for vLLM
		if cfg.Runtime == "vllm" {
			spec["runtimeArgs"] = []string{
				"--dtype=auto",
				"--max-model-len=4096",
				"--gpu-memory-utilization=0.9",
			}
		}

		if cfg.TrustRemoteCode {
			spec["trustRemoteCode"] = true
		}
	}

	// Build the full manifest
	manifest := map[string]interface{}{
		"apiVersion": "aimodel.ai-aas.io/v1alpha1",
		"kind":       "AIModel",
		"metadata": map[string]interface{}{
			"name":      cfg.Name,
			"namespace": cfg.Namespace,
			"labels":    labels,
		},
		"spec": spec,
	}

	return yaml.Marshal(manifest)
}

// extractModelFamily extracts a model family name from a full model name.
// e.g., "unsloth-gpt-oss-20b" -> "gpt-oss"
func extractModelFamily(name string) string {
	// Remove common prefixes
	name = strings.TrimPrefix(name, "unsloth-")
	name = strings.TrimPrefix(name, "openai-")
	name = strings.TrimPrefix(name, "meta-")

	// Remove size suffix (e.g., "-20b", "-7b", "-8b")
	parts := strings.Split(name, "-")
	if len(parts) > 1 {
		last := parts[len(parts)-1]
		if strings.HasSuffix(last, "b") && len(last) <= 4 {
			// Likely a size like "7b", "20b", "120b"
			parts = parts[:len(parts)-1]
		}
	}

	return strings.Join(parts, "-")
}
