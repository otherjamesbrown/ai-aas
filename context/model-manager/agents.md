# Model Manager Context

> **Inherits**: context/agents.md | **Verified**: 2026-01-07 | **Commit**: 923f2ed9

---

## Domain

You own:
- `ai-aas-config/library/` - Model library definitions
- `ai-aas-config/environments/*/models/` - Environment model deployments
- `infra/k8s/kserve/base/` - ClusterServingRuntimes
- `infra/model-recipes/` - ModelRecipe templates
- `docs/runbooks/build-tensorrt-*.md` - TRT-LLM build runbooks
- `docs/runbooks/*-model-*.md` - Model-related runbooks
- `docs/platform/vllm-*.md` - vLLM configuration
- `docs/platform/model-*.md` - Model platform docs
- `docs/architecture/model-*.md` - Model architecture
- `docs/architecture/routing-*.md` - Routing architecture

Hand off to:
- Operator reconciliation bugs → `operator-developer`
- API Router code changes → `go-services-developer`
- CLI model command bugs → `cli-developer`
- ArgoCD/GitOps issues → `infra-ops-manager`
- E2E model tests → `test-developer`

---

## Key Patterns

```yaml
patterns:
  golden_thread:
    rule: spec.modelID is the single source of truth
    do:
      - All names (modelName, externalName) derive from modelID
      - Verify alignment before deployment
    never:
      - Use display names in routing policies
      - Create naming mismatches between registry and InferenceService

  gpu_architecture:
    rule: Engine files are GPU-specific and non-portable
    ada:
      compute: sm_89
      vram: 20GB
      container: "24.10"
      trtllm: "0.14.0"
      quantization: FP8
    blackwell:
      compute: sm_120
      vram: 96GB
      container: "25.06+"
      trtllm: "0.20.0+"
      quantization: BF16
    h100:
      compute: sm_90
      vram: 80GB

  serving_mode:
    triton:
      structure: ensemble (preprocessing/ tensorrt_llm/ postprocessing/ ensemble/)
      protocol: v2 (Triton inference protocol)
      model_name: "always 'ensemble' for TRT-LLM"
      chat_api: "/v2/models/ensemble/generate"
      health: "/v2/health/ready"
      features: ["dynamic batching", "gRPC", "metrics"]
    trtllm_serve:
      structure: simple (rank0.engine, config.json, tokenizer files)
      protocol: openai (/v1/completions)
      known_issues:
        - "/v1/chat/completions broken (GitHub #5648)"
      health: "/v1/models"
    vllm:
      structure: HuggingFace model weights
      protocol: openai (/v1/chat/completions works)
      benefits: ["no compilation", "portable"]
      tradeoffs: ["slower", "no TensorRT optimization"]

  container_version:
    rule: Engine must match container TRT-LLM version
    mapping:
      "24.10": "0.14.0"
      "25.06": "0.20.0"
      "25.08": "0.21.0"
    why: "Version mismatch causes immediate crash on model load"

  model_caching:
    rule: Models must be cached to S3 before deployment
    flow:
      - "ai-aas-cli model registry add <hf-model-id>"
      - "ai-aas-cli model cache pull <model>"
      - "ai-aas-cli model deploy <model> -e <env>"

  routing_policy:
    rule: Models need routing policies for API access
    backends:
      triton: "requires tokenizer for token counting"
      openai: "default, no tokenizer needed"
```

---

## Anti-patterns

```yaml
# WRONG: Building engine with wrong container version
# Ada engine built with 25.06 container - architecture mismatch
container: nvcr.io/nvidia/tritonserver:25.06-trtllm-python-py3
s3Key: models/ada/llama-3.1-8b-instruct/trtllm-v1  # Ada needs 24.10

# WRONG: Using display name in routing policy
ai-aas-cli routing policy create --model "llama-3-8b"  # Should be HF path

# WRONG: Using user-facing model name for Triton TRT-LLM
grpcReq.ModelName = policy.Model  # Always use "ensemble"

# WRONG: Deploying without S3 cache
ai-aas-cli model deploy llama-7b -e development  # No cache pull first

# WRONG: Missing routing policy after deployment
# Model deploys but receives no traffic - returns 404 to clients
```

---

## Commands

```bash
# Registration
ai-aas-cli model registry add <hf-model-id> --name <name>
ai-aas-cli model registry list

# Caching
ai-aas-cli model cache pull <model>
ai-aas-cli model cache status <model>

# Deployment
ai-aas-cli model deploy <model> -e <env> --gpu-count N
ai-aas-cli model status <model> -e <env>
ai-aas-cli model pipeline <model> -e <env>  # Full pipeline status

# Routing
ai-aas-cli routing policy create --global --model <model> --backends <name>:100
ai-aas-cli routing policy list

# Troubleshooting
ai-aas-cli model troubleshoot test <model> -e <env>

# Benchmarking (via ai-aas-org)
ai-aas-org benchmark target add <model>
ai-aas-org benchmark run trigger <scenario>
```

---

## Sources

| What | Where |
|------|-------|
| Model naming | `docs/architecture/model-naming-guide.md` |
| Deployment pipeline | `docs/platform/model-deployment-pipeline.md` |
| Library schema | `ai-aas-config/library/README.md` |
| TRT-LLM builds | `docs/runbooks/build-tensorrt-llm-engine.md` |
| vLLM config | `docs/platform/vllm-best-practices.md` |
| AIModel CRD | `docs/operators/ai-model-operator.md` |
| Inference routing | `docs/architecture/inference-routing.md` |
| Library examples | `ai-aas-config/library/models/*.yaml` |
| ClusterServingRuntimes | `infra/k8s/kserve/base/cluster-serving-runtime-*.yaml` |

---

## Checklist

Before completing work:
- [ ] Verified modelID alignment (golden thread)
- [ ] Checked GPU architecture compatibility
- [ ] Updated library entry if new model
- [ ] Tested inference endpoint
- [ ] Created/updated routing policy
- [ ] Created/updated beads issue
- [ ] Ran `bd sync` to commit beads changes
