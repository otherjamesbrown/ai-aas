---
title: Operator Developer Agent Guide
last_updated: 2025-12-10
owner: operator-developer
---

# Operator Developer Agent Guide

This document serves as the primary reference for the `operator-developer` agent. It provides navigation to all operator-related documentation and establishes patterns for Kubernetes operator development.

## Document Index

| Document | Purpose |
|----------|---------|
| [AI Model Operator](./ai-model-operator.md) | AIModel CRD, reconciliation flow, S3/HuggingFace integration |
| [Operator Patterns](./operator-patterns.md) | Common patterns for controller-runtime operators |
| [CRD Development](./crd-development.md) | How to add/modify Custom Resource Definitions |
| [Testing Operators](./testing-operators.md) | Unit and integration testing strategies |

## Operators Inventory

| Operator | Namespace | CRDs | Helm Chart |
|----------|-----------|------|------------|
| ai-model-operator | ai-model-system | AIModel | `operators/ai-model-operator/deployments/helm/ai-model-operator` |

## Quick Reference

### Source Locations

```
operators/
└── ai-model-operator/
    ├── api/v1alpha1/           # CRD type definitions
    │   └── aimodel_types.go    # AIModel spec/status
    ├── controllers/            # Reconciliation logic
    │   └── aimodel_controller.go
    ├── internal/kserve/        # KServe integration
    ├── config/
    │   ├── crd/bases/          # Generated CRD YAML
    │   └── rbac/               # RBAC manifests
    ├── deployments/helm/       # Helm chart
    └── Makefile                # Build/generate commands
```

### Common Commands

```bash
# Navigate to operator
cd operators/ai-model-operator

# Regenerate after type changes
make generate    # DeepCopy functions
make manifests   # CRD YAML

# Build and test
make build
make test
go test -race ./...

# Lint
golangci-lint run

# Update Helm chart CRD
cp config/crd/bases/*.yaml deployments/helm/ai-model-operator/crds/
```

### AIModel Phases

| Phase | Description |
|-------|-------------|
| `Pending` | Initial state, waiting for processing |
| `Downloading` | Downloader job is running |
| `Downloaded` | Model artifacts uploaded to S3 |
| `Deploying` | InferenceService being created |
| `Ready` | InferenceService is ready |
| `Failed` | Reconciliation failed |
| `Disabled` | spec.enabled is false |
| `RetryPending` | Waiting to retry after failure (planned) |

## Related Agents

| Agent | When to Hand Off |
|-------|------------------|
| **infra-ops-manager** | Deployment issues, Helm chart problems, ArgoCD sync, pod crashes |
| **go-services-developer** | REST API services (admin-api, api-router, analytics, user-org) |
| **cli-developer** | ai-aas-cli bugs or features |

## Handoff Examples

### To infra-ops-manager

```bash
# Operator pod crashing due to RBAC or resource limits
bd create "Fix ai-model-operator RBAC for secrets access" --type bug --priority 1
bd label add <issue-id> agent:infra-ops-manager
```

### To go-services-developer

```bash
# Admin API needs endpoint for operator status
bd create "Add /api/v1/operators/status endpoint to admin-api" --type feature --priority 2
bd label add <issue-id> agent:go-services-developer
```

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                     AI Model Operator                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────┐     ┌──────────────┐     ┌──────────────┐    │
│  │   AIModel    │────▶│  Downloader  │────▶│   KServe     │    │
│  │     CRD      │     │     Job      │     │InferenceService   │
│  └──────────────┘     └──────────────┘     └──────────────┘    │
│         │                    │                    │             │
│         │                    │                    │             │
│         ▼                    ▼                    ▼             │
│  ┌──────────────┐     ┌──────────────┐     ┌──────────────┐    │
│  │    Status    │     │  HuggingFace │     │    vLLM      │    │
│  │   Updates    │     │   + S3       │     │   Runtime    │    │
│  └──────────────┘     └──────────────┘     └──────────────┘    │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Reconciliation Flow

1. **AIModel Created** → Phase: `Pending`
2. **Check S3 for existing artifacts**
   - If found → Skip to step 4
   - If not found → Continue to step 3
3. **Create Downloader Job** → Phase: `Downloading`
   - Downloads from HuggingFace Hub
   - Uploads to S3
4. **Job Completes** → Phase: `Downloaded`
5. **Create/Update InferenceService** → Phase: `Deploying`
6. **InferenceService Ready** → Phase: `Ready`

### Error Handling

- Job failure → Phase: `Failed` (currently no auto-retry, see ai-aas-evb)
- InferenceService failure → Phase: `Failed`
- Transient errors → Requeue with backoff
