package terratest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
)

func TestDevelopmentEnvironmentPlan(t *testing.T) {
	if os.Getenv("RUN_TERRAFORM_TESTS") != "1" {
		t.Skip("set RUN_TERRAFORM_TESTS=1 to execute terraform plan")
	}

	if os.Getenv("LINODE_TOKEN") == "" {
		t.Skip("LINODE_TOKEN not set; skipping plan")
	}

	envDir, err := filepath.Abs(filepath.Join("..", "..", "..", "infra", "terraform", "environments", "development"))
	if err != nil {
		t.Fatalf("failed to resolve terraform directory: %v", err)
	}

	opts := &terraform.Options{
		TerraformDir: envDir,
		NoColor:      true,
	}

	terraform.InitAndPlan(t, opts)
}
