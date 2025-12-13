# Model Recipes Specification

## Overview

A centralized configuration system for AI model deployment "recipes" that captures the specific requirements, runtime configurations, and resource needs for each model type. This enables consistent, repeatable deployments across environments while supporting diverse model types (LLMs, vision models, embedding models) and runtimes (vLLM, Triton, TGI).

## Problem Statement

Currently, each AIModel CR contains all deployment configuration inline:
- Resource requirements (CPU, memory, GPU)
- Runtime arguments (max-model-len, dtype, gpu-memory-utilization)
- Tolerations and node selectors
- Runtime-specific settings

This leads to:
1. **Duplication**: Same model deployed across environments has duplicate configs
2. **Inconsistency**: Easy to have config drift between dev/staging/prod
3. **No knowledge capture**: Learned settings (e.g., "mistral needs tokenizer-mode=mistral") are lost
4. **Hard to manage**: Adding new models requires knowing all the right settings

## Proposed Solution

### Model Recipe CRD

A new `ModelRecipe` Custom Resource that captures the deployment "recipe" for a model:

```yaml
apiVersion: ai.ai-aas.io/v1alpha1
kind: ModelRecipe
metadata:
  name: mistral-7b-instruct-v03
  namespace: ai-model-system  # Cluster-wide recipes
  labels:
    ai.ai-aas.io/model-family: mistral
    ai.ai-aas.io/model-size: 7b
    ai.ai-aas.io/task: text-generation
spec:
  # Model identification
  modelID: mistralai/Mistral-7B-Instruct-v0.3
  displayName: "Mistral 7B Instruct v0.3"
  description: "Mistral AI's instruction-tuned 7B parameter model"

  # Runtime configuration
  runtime: vllm  # vllm | triton | tgi | custom

  # Container image (optional - defaults based on runtime)
  image: vllm/vllm-openai:latest

  # Resource requirements (can be overridden per-environment)
  resources:
    gpu:
      type: nvidia  # nvidia | amd | intel
      count: 1
      minMemoryGB: 16  # Minimum VRAM required
    cpu:
      requests: "4"
      limits: "8"
    memory:
      requests: "16Gi"
      limits: "32Gi"

  # Runtime-specific arguments
  runtimeArgs:
    vllm:
      dtype: auto
      maxModelLen: 8192
      gpuMemoryUtilization: 0.9
      trustRemoteCode: true
      tokenizerMode: mistral  # Important for Mistral models!
      extraArgs:
        - --enable-chunked-prefill
    triton:
      # Triton-specific config would go here
      backend: python
      instanceGroup:
        - kind: GPU
          count: 1

  # Scheduling requirements
  scheduling:
    tolerations:
      - key: nvidia.com/gpu
        operator: Exists
        effect: NoSchedule
      - key: gpu-workload
        operator: Equal
        value: "true"
        effect: NoSchedule
    nodeSelector: {}  # Optional
    affinity: {}      # Optional

  # Health check configuration
  healthCheck:
    startupProbeSeconds: 300  # 5 min for large models
    livenessPath: /health
    readinessPath: /health

  # Model-specific metadata
  metadata:
    parameters: "7B"
    contextLength: 32768
    architecture: "MistralForCausalLM"
    license: "Apache-2.0"
    sourceURL: "https://huggingface.co/mistralai/Mistral-7B-Instruct-v0.3"
```

### Updated AIModel CRD

The AIModel CR becomes simpler - it references a recipe:

```yaml
apiVersion: ai.ai-aas.io/v1alpha1
kind: AIModel
metadata:
  name: mistral-7b-instruct-v03
  namespace: staging
spec:
  # Reference the recipe
  recipeRef:
    name: mistral-7b-instruct-v03
    namespace: ai-model-system  # Optional, defaults to same namespace

  # OR inline recipe (for custom/one-off deployments)
  # recipe:
  #   modelID: ...
  #   runtime: ...

  # Environment-specific overrides
  overrides:
    replicas:
      min: 1
      max: 3
    resources:
      # Override just what's different for this env
      gpu:
        count: 1  # Maybe use 2 GPUs in production
    runtimeArgs:
      vllm:
        gpuMemoryUtilization: 0.85  # More conservative in production

  # Deployment control
  enabled: true
```

### Recipe Library Structure

```
infra/model-recipes/
├── README.md
├── llm/
│   ├── mistral/
│   │   ├── mistral-7b-instruct-v03.yaml
│   │   ├── mistral-7b-instruct-v02.yaml
│   │   └── mixtral-8x7b-instruct.yaml
│   ├── llama/
│   │   ├── llama-2-7b-chat.yaml
│   │   ├── llama-2-13b-chat.yaml
│   │   └── llama-3-8b-instruct.yaml
│   └── openai/
│       └── gpt-oss-20b.yaml
├── vision/
│   ├── florence/
│   │   └── florence-2-large.yaml
│   ├── sam/
│   │   └── sam-vit-huge.yaml
│   └── yolo/
│       └── yolov8x.yaml
├── embedding/
│   ├── bge-large-en.yaml
│   └── e5-mistral-7b-instruct.yaml
└── multimodal/
    ├── llava-1.5-7b.yaml
    └── qwen-vl-chat.yaml
```

### Triton Runtime Support

For vision models and other non-LLM use cases:

```yaml
apiVersion: ai.ai-aas.io/v1alpha1
kind: ModelRecipe
metadata:
  name: florence-2-large
  labels:
    ai.ai-aas.io/task: image-captioning
spec:
  modelID: microsoft/Florence-2-large
  displayName: "Florence 2 Large"
  runtime: triton

  # Triton-specific configuration
  runtimeArgs:
    triton:
      backend: python  # python | tensorrt | onnxruntime | pytorch
      modelRepository: s3://ai-aas-models/triton/florence-2-large
      instanceGroup:
        - kind: GPU
          count: 1
      dynamicBatching:
        maxBatchSize: 8
        maxQueueDelayMicroseconds: 100000
      inputConfig:
        - name: image
          dataType: TYPE_UINT8
          dims: [-1, -1, 3]  # Dynamic H, W, 3 channels
        - name: prompt
          dataType: TYPE_STRING
          dims: [1]
      outputConfig:
        - name: caption
          dataType: TYPE_STRING
          dims: [1]

  resources:
    gpu:
      type: nvidia
      count: 1
      minMemoryGB: 8
    cpu:
      requests: "2"
      limits: "4"
    memory:
      requests: "8Gi"
      limits: "16Gi"
```

### Recipe Validation

The operator validates recipes before deployment:

```go
type RecipeValidator interface {
    // Validate checks if a recipe is valid
    Validate(recipe *ModelRecipe) error

    // ValidateForRuntime checks runtime-specific requirements
    ValidateForRuntime(recipe *ModelRecipe, runtime string) error

    // CheckResourceAvailability verifies cluster can support the recipe
    CheckResourceAvailability(recipe *ModelRecipe, cluster *ClusterInfo) error
}
```

### CLI Integration

```bash
# List available recipes
ai-aas-cli model recipe list
ai-aas-cli model recipe list --task text-generation
ai-aas-cli model recipe list --runtime triton

# Show recipe details
ai-aas-cli model recipe show mistral-7b-instruct-v03

# Validate a recipe
ai-aas-cli model recipe validate ./my-custom-recipe.yaml

# Deploy using a recipe
ai-aas-cli model deploy create --recipe mistral-7b-instruct-v03 -e staging

# Create AIModel from recipe with overrides
ai-aas-cli model deploy create \
  --recipe mistral-7b-instruct-v03 \
  --override replicas.min=2 \
  --override resources.gpu.count=2 \
  -e production
```

### Admin API Integration

```
GET  /api/v1/recipes                    # List all recipes
GET  /api/v1/recipes/{name}             # Get recipe details
POST /api/v1/recipes                    # Create recipe
PUT  /api/v1/recipes/{name}             # Update recipe
DELETE /api/v1/recipes/{name}           # Delete recipe

GET  /api/v1/recipes/{name}/deployments # List deployments using this recipe
POST /api/v1/recipes/{name}/validate    # Validate recipe
```

## Implementation Phases

### Phase 1: Recipe CRD and Basic Support
- Define ModelRecipe CRD
- Update AI Model Operator to read recipes
- Create initial recipe library for existing models
- Basic CLI commands

### Phase 2: Triton Runtime Support
- Add Triton InferenceService builder
- Create vision model recipes (Florence, SAM, YOLO)
- Model repository management for Triton

### Phase 3: Advanced Features
- Recipe versioning and rollback
- Recipe inheritance (base recipes + variants)
- Automatic recipe generation from HuggingFace model cards
- Performance profiling data in recipes

## Benefits

1. **Knowledge Capture**: Learned settings (tokenizer-mode, memory tuning) are preserved
2. **Consistency**: Same recipe across all environments
3. **Flexibility**: Easy overrides for environment-specific needs
4. **Multi-Runtime**: Single abstraction for vLLM, Triton, TGI, etc.
5. **Discoverability**: Browse available recipes, see what's supported
6. **Validation**: Catch misconfigurations before deployment

## Migration Path

1. Create recipes from existing AIModel configs
2. Update AIModels to reference recipes
3. Deprecate inline configs (keep for backwards compatibility)
4. New models must use recipes

## Open Questions

1. Should recipes be namespaced or cluster-scoped?
2. How to handle recipe versioning (semver? git-based?)
3. Should we auto-generate recipes from HuggingFace model cards?
4. How to handle confidential/private model recipes?
