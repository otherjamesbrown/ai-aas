# Inference Engine Expansion

> Expanding ai-aas beyond LLMs to support speech-to-text, image generation/editing, and video generation.

## ai-aas-config Restructure

### Configuration Hierarchy

```
Engine
    └── ModelTemplate (base config, known-good settings)
            └── Model (instance with environment-specific overrides)
```

**Engine** - Container image, protocol, base defaults
**ModelTemplate** - Template for a model class on an engine (GPU reqs, runtime args, scaling defaults)
**Model** - Actual deployment using a template + environment-specific overrides

### Proposed Directory Structure

```
ai-aas-config/
├── engines/
│   ├── vllm.yaml
│   ├── vllm.md                   # Engine runbook
│   ├── triton.yaml
│   ├── triton.md
│   ├── faster-whisper.yaml
│   └── faster-whisper.md
├── templates/
│   ├── llm-7b-vllm.yaml          # ModelTemplate
│   ├── llm-7b-vllm.md            # Template-specific notes
│   ├── llm-70b-vllm.yaml
│   ├── sdxl-triton.yaml
│   ├── whisper-large.yaml
│   └── ...
├── environments/
│   ├── development/
│   │   └── models/
│   │       ├── qwen-2.5-7b.yaml  # Model (references template)
│   │       └── whisper-v3.yaml
│   └── production/
│       └── models/
│           └── ...
└── knowledge/                     # Cross-cutting learnings
    ├── README.md                  # Index of all knowledge docs
    ├── model-sources.md          # S3 vs HuggingFace lessons
    ├── instruct-vs-base.md       # Model type differences
    └── version-compatibility.md  # Engine/model version matrix
```

### Configuration Layering

| Config | Level |
|--------|-------|
| Container image, protocol, API endpoints | Engine |
| Tensor parallelism, batch size, memory utilization | ModelTemplate |
| Replicas, env-specific resources | Model |

### API Endpoints by Engine

| Engine | OpenAI-Compatible Endpoints | Notes |
|--------|----------------------------|-------|
| **vLLM** | `/v1/chat/completions`, `/v1/completions`, `/v1/embeddings` | Full OpenAI chat API |
| **vLLM** (vision models) | `/v1/chat/completions` (with image content) | OpenAI vision format |
| **Faster-Whisper** | `/v1/audio/transcriptions`, `/v1/audio/translations` | OpenAI audio API |
| **Triton** (diffusion) | `/v1/images/generations`, `/v1/images/edits` | Custom handler in api-router |
| **Triton** (video) | `/v1/video/generations` | Custom handler in api-router |

### Example YAML Files

**Engine (`engines/vllm.yaml`):**
```yaml
apiVersion: aimodel.ai-aas.io/v1alpha1
kind: Engine
metadata:
  name: vllm
  annotations:
    ai-aas.io/docs: engines/vllm.md
spec:
  container: vllm/vllm-openai
  defaultVersion: "0.6.4"
  protocol: openai
  endpoints:
    - path: /v1/chat/completions
      method: POST
      description: Chat completions (OpenAI-compatible)
    - path: /v1/completions
      method: POST
      description: Text completions (OpenAI-compatible)
    - path: /v1/embeddings
      method: POST
      description: Text embeddings (OpenAI-compatible)
  compatibility:
    - model: gpt-oss-20b
      minVersion: "0.6.0"
      notes: "Requires trust-remote-code"
    - model: qwen-2.5-*
      minVersion: "0.5.0"
```

**Engine (`engines/faster-whisper.yaml`):**
```yaml
apiVersion: aimodel.ai-aas.io/v1alpha1
kind: Engine
metadata:
  name: faster-whisper
  annotations:
    ai-aas.io/docs: engines/faster-whisper.md
spec:
  container: fedirz/faster-whisper-server
  defaultVersion: "latest-cuda"
  protocol: openai
  endpoints:
    - path: /v1/audio/transcriptions
      method: POST
      description: Speech-to-text transcription (OpenAI-compatible)
    - path: /v1/audio/translations
      method: POST
      description: Speech-to-text with translation to English (OpenAI-compatible)
```

**Engine (`engines/triton.yaml`):**
```yaml
apiVersion: aimodel.ai-aas.io/v1alpha1
kind: Engine
metadata:
  name: triton
  annotations:
    ai-aas.io/docs: engines/triton.md
spec:
  container: nvcr.io/nvidia/tritonserver
  defaultVersion: "24.08-py3"
  protocol: kserve-v2
  endpoints:
    # Diffusion models (custom OpenAI-style handler in api-router)
    - path: /v1/images/generations
      method: POST
      description: Image generation
      modelTypes: [diffusion]
    - path: /v1/images/edits
      method: POST
      description: Image editing
      modelTypes: [diffusion]
    # Video models (custom handler in api-router)
    - path: /v1/video/generations
      method: POST
      description: Video generation
      modelTypes: [video]
```

**ModelTemplate (`templates/llm-7b-vllm.yaml`):**
```yaml
apiVersion: aimodel.ai-aas.io/v1alpha1
kind: ModelTemplate
metadata:
  name: llm-7b-vllm
  annotations:
    ai-aas.io/docs: templates/llm-7b-vllm.md
spec:
  engine: vllm
  resources:
    gpu: 1
    memory: 20Gi
  runtimeArgs:
    max-model-len: 8192
    gpu-memory-utilization: 0.9
```

**Model (`environments/development/models/qwen-2.5-7b.yaml`):**
```yaml
apiVersion: aimodel.ai-aas.io/v1alpha1
kind: Model
metadata:
  name: qwen-2.5-7b
spec:
  template: llm-7b-vllm
  modelId: Qwen/Qwen2.5-7B-Instruct
  replicas: 1
  # Override template values as needed
  runtimeArgs:
    max-model-len: 16384
```

## Knowledge Base

Capture operational knowledge for each engine and model to build institutional memory.

### Engine Runbook Example (`engines/vllm.md`)

```markdown
# vLLM Engine Notes

## Version Compatibility

| Model | Min vLLM Version | Notes |
|-------|------------------|-------|
| gpt-oss-20b | 0.6.0+ | Needs --trust-remote-code |
| Qwen 2.5 | 0.5.0+ | Works with default config |
| Llama 3.1 | 0.4.0+ | |

## Known Issues

### S3 Model Loading (2024-11)
**Problem:** Tried loading models directly from S3, hit timeout issues.
**Resolution:** Fall back to HuggingFace download + local cache.
**Ticket:** ai-aas-xyz

### gpt-oss-20b Tokenizer (2024-12)
**Problem:** Model failed to load with vLLM 0.5.x
**Resolution:** Upgrade to vLLM 0.6.0+, requires trust_remote_code: true
**Ticket:** ai-aas-abc

## Best Practices

- Always pin vLLM version in engine config
- Use --download-dir to control model cache location
- Set VLLM_ATTENTION_BACKEND=FLASH_ATTN for better perf
```

### Knowledge Index (`knowledge/README.md`)

```markdown
# AI-AAS Knowledge Base

## Common Gotchas
- [Model Sources: S3 vs HuggingFace](model-sources.md)
- [Instruct vs Base Models](instruct-vs-base.md)
- [Version Compatibility Matrix](version-compatibility.md)

## Engine Runbooks
- [vLLM](../engines/vllm.md)
- [Triton](../engines/triton.md)
- [Faster-Whisper](../engines/faster-whisper.md)

## Template Notes
- [LLM 7B on vLLM](../templates/llm-7b-vllm.md)
- [SDXL on Triton](../templates/sdxl-triton.md)
```

---

## Current State

- **vLLM** - LLM inference (primary)
- **Triton** - LLM inference via TensorRT-LLM

## Proposed Engine Stack

| Engine | Model Types | Why |
|--------|-------------|-----|
| **vLLM** (have) | LLMs, Vision-Language | Best-in-class for text generation |
| **Triton** (have) | Diffusion, Vision, General ML | Multi-framework, TensorRT acceleration |
| **Faster-Whisper Server** (add) | Speech-to-Text | 4x faster than vanilla Whisper, OpenAI-compatible API |

## Model Catalog (from HuggingFace)

### LLMs → vLLM

| Model | HuggingFace ID | Params | VRAM (FP16) | Context | License | Notes |
|-------|----------------|--------|-------------|---------|---------|-------|
| Llama 3.2 3B | `meta-llama/Llama-3.2-3B-Instruct` | 3.2B | ~7GB | 128K | Llama 3.2 | 8 languages, knowledge distilled |
| Qwen 2.5 7B | `Qwen/Qwen2.5-7B-Instruct` | 7.6B | ~16GB | 128K | Apache 2.0 | YaRN for >32K context |
| Mistral 7B | `mistralai/Mistral-7B-Instruct-v0.3` | 7B | ~14GB | 32K | Apache 2.0 | Function calling, 32K vocab |
| Gemma 2 9B | `google/gemma-2-9b-it` | 9B | ~18GB | 8K | Gemma | Requires license acceptance |
| Mixtral 8x7B | `mistralai/Mixtral-8x7B-Instruct-v0.1` | 47B (12B active) | ~90GB / 24GB Q4 | 32K | Apache 2.0 | MoE, use 4-bit quant |
| DeepSeek R1 | `deepseek-ai/DeepSeek-R1` | 671B (37B active) | Multi-GPU | 128K | MIT | Use distilled variants (7B-70B) |

**GPU Targets:**
- RTX 4000 Ada (20GB): Llama 3.2 3B, Qwen 2.5 7B, Mistral 7B, Gemma 2 9B
- RTX 6000 Blackwell (48GB): Mixtral 8x7B (Q4), Qwen 2.5 32B, DeepSeek-R1-Distill-32B

### Vision/Multimodal → vLLM

| Model | HuggingFace ID | Params | VRAM (FP16) | License | Notes |
|-------|----------------|--------|-------------|---------|-------|
| Qwen2.5-VL 7B | `Qwen/Qwen2.5-VL-7B-Instruct` | 8B | ~16GB | Apache 2.0 | Images + video, flash_attn recommended |
| Qwen2.5-VL 72B | `Qwen/Qwen2.5-VL-72B-Instruct` | 72B | Multi-GPU | Apache 2.0 | 1hr+ video support |

**GPU Targets:**
- RTX 4000 Ada: Qwen2.5-VL 7B
- RTX 6000 Blackwell: Qwen2.5-VL 72B (multi-GPU)

### Speech-to-Text → Faster-Whisper Server

| Model | HuggingFace ID | Params | VRAM | License | Notes |
|-------|----------------|--------|------|---------|-------|
| Whisper Large V3 Turbo | `openai/whisper-large-v3-turbo` | 809M | ~2GB | MIT | 4 decoder layers (vs 32), 99 languages |
| Distil-Whisper | `distil-whisper/distil-large-v3` | 756M | ~2GB | MIT | 6.3x faster than large-v3, <1% WER diff |

**GPU Targets:**
- RTX 4000 Ada: Both models easily fit
- Note: Faster-Whisper uses CTranslate2, 4x faster than vanilla

### Image Generation/Editing → Triton

| Model | HuggingFace ID | Params | VRAM | License | Notes |
|-------|----------------|--------|------|---------|-------|
| SDXL Base | `stabilityai/stable-diffusion-xl-base-1.0` | ~3.5B | ~8GB | CreativeML RAIL++ | Use with refiner for best quality |
| FLUX.1 Dev | `black-forest-labs/FLUX.1-dev` | 12B | ~24GB | Non-Commercial | CPU offload available, use BF16 |
| Nunchaku Qwen Edit | `nunchaku-tech/nunchaku-qwen-image-edit-2509` | - | ~16GB | Apache 2.0 | INT4/NVFP4 quant, Nunchaku runtime |

**GPU Targets:**
- RTX 4000 Ada: SDXL (with optimizations)
- RTX 6000 Blackwell: FLUX.1, Nunchaku Qwen Edit

### Video Generation → Triton

| Model | HuggingFace ID | Params | VRAM | Output | License | Notes |
|-------|----------------|--------|------|--------|---------|-------|
| CogVideoX-2B | `THUDM/CogVideoX-2b` | 2B | 4-18GB | 6s @ 720x480 | Apache 2.0 | Entry-level, FP16 recommended |
| LTX-Video 2B | `Lightricks/LTX-Video` (ltxv-2b) | 2B | ~12GB | 30fps @ 1216x704 | LTX License | Distilled version available |
| Mochi 1 | `genmo/mochi-1-preview` | 10B | 22-60GB | 480p | Apache 2.0 | BF16 variant for 22GB |

**GPU Targets:**
- RTX 6000 Blackwell: CogVideoX-2B, LTX-Video 2B
- RTX 6000 Blackwell (multi-GPU): Mochi 1 (full precision)

### Special Requirements Summary

| Model | Requirement |
|-------|-------------|
| Qwen 2.5 7B+ | `transformers>=4.37.0` |
| Qwen2.5-VL | `flash_attention_2` recommended |
| Gemma 2 | License acceptance on HuggingFace |
| DeepSeek R1 | Temperature 0.5-0.7, no system prompt |
| FLUX.1 | `diffusers>=0.30`, BF16 dtype |
| Mochi 1 | FFMPEG for video output |
| CogVideoX | English prompts only, 226 token max |

## API Compatibility

| Engine | KServe Managed | OpenAI-Compatible API |
|--------|----------------|----------------------|
| **vLLM** | Yes | `/v1/chat/completions` |
| **Faster-Whisper** | Yes (custom runtime) | `/v1/audio/transcriptions` |
| **Triton** (diffusion) | Yes | Custom (no OpenAI standard) |
| **Triton** (video) | Yes | Custom (no OpenAI standard) |

### API Routing Strategy

All managed by KServe, routed by api-router-service:

- `/v1/chat/completions` → vLLM (LLMs + vision models)
- `/v1/audio/transcriptions` → Faster-Whisper
- `/v1/images/generate` → Triton (custom handler)
- `/v1/images/edit` → Triton (custom handler)
- `/v1/video/generate` → Triton (custom handler)

## VRAM Reference

| GPU | VRAM | Suitable For |
|-----|------|--------------|
| RTX 4000 Ada | ~20GB | 7B models, SD 1.5/2.1, Whisper |
| RTX 6000 Blackwell | ~48GB+ | 70B quantized, SDXL/FLUX, video models |

## Implementation Notes

### Adding Faster-Whisper Server

1. Register as new inference engine in `inference_engines` table
2. Create KServe `ClusterServingRuntime` for faster-whisper container
3. Add `/v1/audio/transcriptions` endpoint to api-router-service
4. Container: `fedirz/faster-whisper-server:latest-cuda`

### Adding Diffusion Support to Triton

- NVIDIA has [official Stable Diffusion docs](https://docs.nvidia.com/deeplearning/triton-inference-server/user-guide/docs/tutorials/Popular_Models_Guide/StableDiffusion/README.html)
- Use Python backend with TensorRT optimization
- Create adapter in api-router for OpenAI-style image endpoints

### Video Generation Considerations

- Very resource intensive (4+ H100s for larger models)
- Mochi, HunyuanVideo need 60-80GB VRAM each
- Start with smaller models (LTX-Video, CogVideoX-2B)

## References

- [KServe Documentation](https://kserve.github.io/website/)
- [Faster-Whisper](https://github.com/SYSTRAN/faster-whisper)
- [Triton Stable Diffusion Guide](https://docs.nvidia.com/deeplearning/triton-inference-server/user-guide/docs/tutorials/Popular_Models_Guide/StableDiffusion/README.html)
- [vLLM](https://github.com/vllm-project/vllm)

---

## Lessons Learned (from beads)

> Extracted from closed beads to seed the knowledge base.

### vLLM Version Compatibility

| Issue | Resolution | Bead |
|-------|------------|------|
| gpt-oss-20b failed to load on vLLM 0.5.x | Upgrade to vLLM 0.6.0+, requires `trust_remote_code: true` | ai-aas-0to4 |
| Model tokenizer issues | Pin vLLM version in engine config, don't use `latest` | - |

**Best Practice:** Always document minimum vLLM version per model in ModelTemplate.

### Triton/TensorRT-LLM Compatibility

| Issue | Resolution | Bead |
|-------|------------|------|
| TensorRT-LLM 0.12.0 incompatible with Triton image | Updated Triton image to 24.08 | ai-aas-0iia |
| Tensor shape mismatch in postprocessing | Fixed model.py for tensor shape compatibility | ai-aas-0iia |
| Triton TRT-LLM backend not OpenAI-compatible | Requires wrapper, vLLM redeploy, or custom api-router handling | - |

**Best Practice:** TRT-LLM and Triton versions must be paired. Document compatible versions in engine config.

**Important:** Triton with TensorRT-LLM does NOT serve OpenAI-compatible endpoints natively. Options:
1. Add OpenAI-compatible wrapper layer
2. Redeploy model with vLLM instead (simpler, recommended for LLMs)
3. Custom handling in api-router-service for Triton backend translation

For LLMs, prefer vLLM over Triton/TRT-LLM unless you specifically need TensorRT optimizations.

### Model Source: S3 vs HuggingFace

| Issue | Resolution | Bead |
|-------|------------|------|
| S3 model loading hit timeout issues | Fall back to HuggingFace download + local cache | ai-aas-0to4 |
| Model download slow on first pull | Use `--download-dir` to control cache location | - |

**Best Practice:** Default to HuggingFace with local caching. S3 is unreliable for large models.

### KServe Revision Management

| Issue | Resolution | Bead |
|-------|------------|------|
| Hardcoded revision numbers (00001) break when KServe updates | Use revision-independent service endpoints | ai-aas-0qhc |
| Old revisions hold GPU resources (3 GPUs wasted) | Configure Knative GC: `min-non-active-revisions: 1`, `retain-since-last-active-time: 5m` | ai-aas-11c |
| CrashLoopBackOff revision still allocating GPU | Aggressive GC settings clean up failed revisions | ai-aas-11c |

**Best Practice:** Never hardcode revision numbers. Configure aggressive GC for GPU workloads.

### API/URL Configuration

| Issue | Resolution | Bead |
|-------|------------|------|
| GuideLLM doubled path: `/v1/chat/completions/v1/chat/completions` | Use base URL only, clients auto-append paths | ai-aas-0to4 |
| OpenAI clients add `/v1/...` automatically | Configure base URL without path suffix | ai-aas-0to4 |

**Best Practice:** Document whether engine expects base URL or full endpoint path.

### Configuration Drift

| Issue | Resolution | Bead |
|-------|------------|------|
| Env var set but not read by application | Bridge Kubernetes env vars to application config | ai-aas-0to4 |
| API key in env, but code expects YAML config | Fall back to `os.Getenv()` if YAML field empty | ai-aas-0to4 |

**Best Practice:** Follow 12-factor: prefer env vars, fall back gracefully between config sources.

### Go Module Paths

| Issue | Resolution | Bead |
|-------|------------|------|
| `github.com/ai-aas/shared-go` vs `github.com/otherjamesbrown/ai-aas/shared/go` | Standardize on full path with replace directive | ai-aas-0q8a |
| Inconsistent imports across services | Apply same pattern to all services | ai-aas-0q8a |

**Best Practice:** Use consistent module paths. Document the canonical import path.

### CRD API Group Naming

| Issue | Resolution | Bead |
|-------|------------|------|
| `ai.ai-aas.io/v1alpha1` vs `aimodel.ai-aas.io/v1alpha1` mismatch | Standardize on `aimodel.ai-aas.io` | ai-aas-14bz |
| ArgoCD sync failures due to wrong API group | Update all YAML files to correct group | ai-aas-14bz |

**Best Practice:** Document canonical API group. Validate in CI before merge.

### CI/CD Patterns

| Issue | Resolution | Bead |
|-------|------------|------|
| All tests must pass before images build | Allow dev/staging images to build with failing tests | ai-aas-0o5 |
| Blocked development iteration | Separate production gate from dev image builds | ai-aas-0o5 |

**Best Practice:** Fast feedback for dev branches, strict gates for production only.
