# Archived Helm Charts

This directory contains deprecated Helm charts that have been replaced by KServe InferenceServices.

## vllm-deployment

**Status**: Deprecated
**Replaced By**: KServe InferenceService with vLLM runtime
**Migration Date**: November 2025
**Reason**: Migrated to KServe for standardized ML model serving with Knative autoscaling

### Why Archived

The custom vLLM Helm chart was used for deploying vLLM models directly as Kubernetes Deployments.
This approach had several limitations:
- Manual management of each model deployment
- No built-in autoscaling (relied on HPA)
- No scale-to-zero capability
- Inconsistent deployment patterns across models

### New Approach

All models are now deployed using KServe InferenceServices:
- Location: `infra/k8s/kserve/models/`
- Template: `infra/k8s/kserve/templates/inference-service-vllm-template.yaml`
- Benefits:
  - Standardized CRD-based deployment
  - Automatic Knative autoscaling (0→N replicas)
  - Built-in canary deployments and traffic splitting
  - Consistent model serving interface

### Reference

See `docs/runbooks/kserve-migration-deployment.md` for complete migration documentation.
