# E2E Test: Model Recipe Deployment

This document describes the end-to-end test for deploying models using ModelRecipes.

## Overview

The `recipe_deploy_test.go` test validates the complete flow of deploying a model using a ModelRecipe, including:

1. Creating a ModelRecipe in the cluster
2. Deploying a model with the `--recipe` flag
3. Verifying the AIModel references the recipe correctly
4. Verifying configuration merging from recipe
5. Verifying InferenceService creation (if operator is running)
6. Testing override functionality

## Prerequisites

### Required Components

- **Kubernetes cluster** with access configured (KUBECONFIG)
- **ai-model-operator** installed in the cluster
- **CustomResourceDefinitions (CRDs)** installed:
  - `ModelRecipe` CRD (`aimodel.ai-aas.io/v1alpha1`)
  - `AIModel` CRD (`aimodel.ai-aas.io/v1alpha1`)
  - `InferenceService` CRD (`serving.kserve.io/v1beta1`) - optional

### Optional Components

- **ai-aas-cli** binary built and available in PATH
- **Operator running** in the cluster (for full E2E validation)
- **KServe** installed (for InferenceService verification)

### Environment Setup

```bash
# Set KUBECONFIG if not using default
export KUBECONFIG=/path/to/kubeconfig-development.yaml

# Verify cluster access
kubectl cluster-info
kubectl get crd modelrecipes.aimodel.ai-aas.io
kubectl get crd aimodels.aimodel.ai-aas.io

# Verify operator is running (optional)
kubectl get pods -n ai-model-system -l app=ai-model-operator
```

## Running the Tests

### Full E2E Test Suite

Run both tests (basic deployment + overrides):

```bash
cd /home/dev/worktrees/model-recipes/tests/e2e

# Run all recipe deployment tests
go test -v ./suites -run TestRecipeDeploy -timeout 10m

# Run with verbose output
go test -v ./suites -run TestRecipeDeploy -timeout 10m -args -test.v
```

### Individual Test Cases

**Test 1: Basic Recipe Deployment**

```bash
go test -v ./suites -run TestRecipeDeployE2E -timeout 5m
```

This test:
- Creates a ModelRecipe with standard Llama 7B configuration
- Deploys a model using the recipe
- Verifies the AIModel has the correct `recipeRef`
- Checks if InferenceService is created (skips if operator not running)

**Test 2: Recipe with Overrides**

```bash
go test -v ./suites -run TestRecipeDeployWithOverrides -timeout 5m
```

This test:
- Creates a ModelRecipe with base configuration
- Deploys with CLI overrides (--gpu-count, --memory)
- Verifies overrides are correctly set in AIModel spec

### Running in Different Modes

**Short mode (skip E2E tests):**
```bash
go test -v ./suites -short
```

**CI/CD mode:**
```bash
go test -v ./suites -run TestRecipeDeploy -timeout 10m -json > test-results.json
```

## Test Scenarios

### Scenario 1: Basic Recipe Deployment

**Test Steps:**

1. **Create ModelRecipe** (`ai-model-system/test-llama-7b-recipe`)
   - ModelID: `meta-llama/Llama-2-7b-hf`
   - Runtime: vLLM
   - Resources: 1 GPU, 16Gi memory
   - Runtime args: dtype=auto, maxModelLen=4096

2. **Deploy AIModel** via CLI (dry-run mode)
   ```bash
   ai-aas-cli model deploy test-llama-7b -e development --recipe test-llama-7b-recipe --dry-run
   ```

3. **Verify AIModel** in cluster
   ```bash
   kubectl get aimodel test-llama-7b-development -n development -o yaml
   ```

   Expected spec:
   ```yaml
   spec:
     recipeRef:
       name: test-llama-7b-recipe
       namespace: ai-model-system
     enabled: true
   ```

4. **Verify merged configuration** (if operator running)
   - Operator should resolve recipe and populate status
   - InferenceService should be created with recipe configuration

### Scenario 2: Recipe with Overrides

**Test Steps:**

1. **Create ModelRecipe** with base configuration (same as above)

2. **Deploy with overrides**
   ```bash
   ai-aas-cli model deploy test-llama-7b-override -e development \
     --recipe test-llama-7b-override-recipe \
     --gpu-count 2 \
     --memory 48 \
     --dry-run
   ```

3. **Verify AIModel overrides**
   ```bash
   kubectl get aimodel test-llama-7b-override-development -n development -o yaml
   ```

   Expected spec:
   ```yaml
   spec:
     recipeRef:
       name: test-llama-7b-override-recipe
       namespace: ai-model-system
     overrides:
       resources:
         gpu:
           count: 2
         memory:
           requests: 48Gi
     enabled: true
   ```

4. **Verify merged configuration**
   - Operator should merge recipe + overrides
   - InferenceService should use 2 GPUs and 48Gi memory

## Test Structure

### Test File Organization

```
tests/e2e/suites/recipe_deploy_test.go
├── TestRecipeDeployE2E              # Main E2E test
│   ├── Step1_CreateModelRecipe
│   ├── Step2_DeployWithRecipe
│   ├── Step3_VerifyAIModelRecipeRef
│   ├── Step4_VerifyMergedConfig
│   └── Step5_VerifyInferenceService
│
└── TestRecipeDeployWithOverrides    # Override test
    ├── Step1_CreateModelRecipe
    ├── Step2_DeployWithRecipeAndOverrides
    └── Step3_VerifyOverridesApplied
```

### Helper Functions

- `setupKubernetesClient()` - Creates dynamic Kubernetes client
- `testCreateModelRecipe()` - Creates test ModelRecipe CR
- `testDeployModelWithRecipe()` - Executes CLI deploy command
- `testVerifyAIModelRecipeRef()` - Validates recipeRef in AIModel
- `testVerifyMergedConfiguration()` - Checks operator merged config
- `testVerifyInferenceService()` - Verifies InferenceService creation
- `cleanupE2EResources()` - Cleanup test resources

## Expected Behavior

### When Operator is Running

✅ **Full E2E validation:**
- ModelRecipe is created
- AIModel is created with recipeRef
- Operator reconciles AIModel
- Operator creates InferenceService with merged configuration
- Status is updated with deployment phase

### When Operator is NOT Running

⚠️ **Partial validation:**
- ModelRecipe is created ✅
- AIModel is created with recipeRef ✅
- No status updates (operator not running) ⚠️
- No InferenceService created ⚠️
- Tests skip optional verification steps

## Cleanup

The test automatically cleans up resources using `t.Cleanup()`:

```bash
# Resources deleted after test:
- AIModel: development/test-llama-7b-development
- ModelRecipe: ai-model-system/test-llama-7b-recipe
- InferenceService: development/test-llama-7b-development (if exists)
```

Manual cleanup (if test is interrupted):

```bash
# Delete test resources
kubectl delete aimodel test-llama-7b-development -n development
kubectl delete aimodel test-llama-7b-override-development -n development
kubectl delete modelrecipe test-llama-7b-recipe -n ai-model-system
kubectl delete modelrecipe test-llama-7b-override-recipe -n ai-model-system

# Delete test namespaces (if needed)
kubectl delete namespace development --ignore-not-found
kubectl delete namespace ai-model-system --ignore-not-found
```

## Debugging

### Enable Verbose Logging

```bash
# Verbose test output
go test -v ./suites -run TestRecipeDeploy

# Kubernetes client debugging
export KUBECTL_DEBUG=true
```

### Check Test Logs

```bash
# View test output
go test -v ./suites -run TestRecipeDeploy 2>&1 | tee test.log

# Check created resources
kubectl get modelrecipes -A
kubectl get aimodels -A
kubectl get inferenceservices -A
```

### Common Issues

**Issue: "Kubernetes cluster not available"**
```bash
# Verify KUBECONFIG
echo $KUBECONFIG
kubectl cluster-info

# Check cluster connectivity
kubectl get nodes
```

**Issue: "CRD not found"**
```bash
# Install CRDs
kubectl apply -f operators/ai-model-operator/config/crd/bases/

# Verify CRDs
kubectl get crd | grep aimodel
```

**Issue: "Operator not running"**
```bash
# Check operator deployment
kubectl get deployment ai-model-operator -n ai-model-system

# View operator logs
kubectl logs -n ai-model-system -l app=ai-model-operator --tail=50
```

**Issue: "CLI command failed"**
- Tests use `--dry-run` to avoid actual deployment
- CLI failures are expected if not fully configured
- Tests verify YAML output structure instead

## Integration with CI/CD

### GitHub Actions Example

```yaml
name: E2E Recipe Deploy Test

on: [push, pull_request]

jobs:
  e2e-recipe:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Set up Kubernetes (kind)
        uses: helm/kind-action@v1

      - name: Install CRDs
        run: |
          kubectl apply -f operators/ai-model-operator/config/crd/bases/

      - name: Run E2E Recipe Tests
        run: |
          cd tests/e2e
          go test -v ./suites -run TestRecipeDeploy -timeout 10m
```

## References

- [Model Recipes Spec](../../specs/025-model-recipes/spec.md)
- [AIModel Operator](../../operators/ai-model-operator/README.md)
- [CLI Deploy Command](../../services/ai-aas-cli/cmd/model/deploy.go)
- [E2E Test Framework](./README.md)

## Contributing

When modifying the test:

1. **Keep tests hermetic** - Create unique resource names per test run
2. **Use t.Cleanup()** - Always clean up resources
3. **Handle missing dependencies gracefully** - Skip tests if operator not running
4. **Add descriptive logs** - Use t.Logf() for debugging
5. **Document expected behavior** - Update this README

## Future Enhancements

Potential improvements for this test:

- [ ] Add test for multiple recipes in same deployment
- [ ] Test recipe versioning and updates
- [ ] Test invalid recipe references (error cases)
- [ ] Test cross-namespace recipe references
- [ ] Add performance benchmarks for recipe resolution
- [ ] Test concurrent deployments with same recipe
- [ ] Add test for recipe deletion with active AIModels
