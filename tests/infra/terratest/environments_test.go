package terratest

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
)

func resolveEnvDir(t *testing.T, env string) string {
	t.Helper()
	envDir, err := filepath.Abs(filepath.Join("..", "..", "..", "infra", "terraform", "environments", env))
	if err != nil {
		t.Fatalf("failed to resolve terraform directory: %v", err)
	}
	return envDir
}

func TestDevelopmentEnvironmentPlan(t *testing.T) {
	if os.Getenv("RUN_TERRAFORM_TESTS") != "1" {
		t.Skip("set RUN_TERRAFORM_TESTS=1 to execute terraform plan")
	}

	if os.Getenv("LINODE_TOKEN") == "" {
		t.Skip("LINODE_TOKEN not set; skipping plan")
	}

	envDir := resolveEnvDir(t, "development")
	opts := terraform.WithDefaultRetryableErrors(t, &terraform.Options{
		TerraformDir: envDir,
		NoColor:      true,
		Vars: map[string]interface{}{
			"environment": "development",
			"project":     "ai-aas",
		},
	})

	terraform.InitAndPlan(t, opts)
}

func TestGenerateArtifacts(t *testing.T) {
	if os.Getenv("RUN_TERRAFORM_GENERATE") != "1" {
		t.Skip("set RUN_TERRAFORM_GENERATE=1 to run targeted artifact generation")
	}

	env := os.Getenv("TF_ENV")
	if env == "" {
		env = "development"
	}

	envDir := resolveEnvDir(t, env)
	opts := terraform.WithDefaultRetryableErrors(t, &terraform.Options{
		TerraformDir: envDir,
		NoColor:      true,
		Targets: []string{
			"module.argo_bootstrap.local_file.applicationset",
			"module.data_services.local_file.documentation",
			"module.network.local_file.network_policy",
			"module.network.local_file.firewall",
			"module.observability.local_file.values",
			"module.secrets_bootstrap.local_sensitive_file.sealed_secret",
		},
		Vars: map[string]interface{}{
			"environment": env,
			"project":     "ai-aas",
		},
	})

	terraform.InitAndApply(t, opts)
	defer terraform.Destroy(t, opts)

	generatedDir := filepath.Join(envDir, ".generated")
	expected := []string{
		filepath.Join("argo", fmt.Sprintf("%s-applicationset.yaml", env)),
		filepath.Join("data-services", fmt.Sprintf("%s-endpoints.md", env)),
		filepath.Join("network", fmt.Sprintf("%s-network-policy.yaml", env)),
		filepath.Join("network", fmt.Sprintf("%s-firewall.json", env)),
		filepath.Join("observability", fmt.Sprintf("%s-values.yaml", env)),
		filepath.Join("secrets", fmt.Sprintf("%s-bootstrap.yaml", env)),
	}

	for _, rel := range expected {
		path := filepath.Join(generatedDir, rel)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected generated artifact %s: %v", path, err)
		}
	}
}
