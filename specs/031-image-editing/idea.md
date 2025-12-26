# Idea: Image Editing Support

**Spec Number:** 031
**Created:** 2025-12-17
**Status:** Draft

## Problem

The ai-aas platform currently only supports LLM inference (vLLM, Triton, TensorRT-LLM, TGI). Users want to edit images using AI - upload an image with instructions like "change the sky to sunset" and get back a modified image.

## Discussion

- Explored current architecture - platform supports 4 runtimes, all LLM-focused
- The API router is protocol-agnostic (just forwards HTTP to endpoints)
- Model registry and routing policies are generic - already support arbitrary backends
- Minimal code changes needed in CRD/operator layer

**Target Model**: `nunchaku-tech/nunchaku-qwen-image-edit-2509`
- Image editing model (image + text → edited image)
- 4-bit quantized, runs on 8GB VRAM + 16GB RAM
- Based on Qwen-Image-Edit-2509

**Architecture Decision**: Extend `api-router-service` with image endpoints (Option 1)
- Industry standard pattern (OpenAI, Together AI, Fireworks all do this)
- Single API surface: `/v1/chat/completions` (LLM) + `/v1/images/edits` (image editing)
- Same auth, same routing policy system, unified billing/logging

## Proposed Approach

1. **AIModel CRD** - Add `nunchaku` to runtime enum
2. **Operator Controller** - Add switch case mapping `nunchaku` → KServe ClusterServingRuntime
3. **KServe Runtime** - Create `ClusterServingRuntime` for Nunchaku (new file)
4. **API Router** - Add `/v1/images/edits` endpoint
5. **Backend Config** - Add Nunchaku service endpoints to Helm values

**No changes needed**: Model registry schema, routing policies, database migrations

## Open Questions

- What is Nunchaku's exact API contract? Need to verify OpenAI Image API compatibility
- GPU memory requirements - docs say 8GB VRAM minimum
- Container image for Nunchaku runtime - official or custom build?
- Rate limiting strategy for image editing vs LLM inference?

## Out of Scope

- Text-to-image generation (Stable Diffusion, DALL-E style)
- Video generation/editing
- Vision-language models (image understanding) - that's vLLM territory

## Notes

- Beads issue: ai-aas-4yn2
- Model: https://huggingface.co/nunchaku-tech/nunchaku-qwen-image-edit-2509
- Nunchaku runtime: https://github.com/nunchaku-tech/nunchaku
