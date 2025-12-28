# Documentation Standards for docs/platform/

**Last Updated**: 2025-12-08

## Purpose

This directory serves as the primary reference for the **infra-ops-manager** AI sub-agent and human operators. Documents must accurately map the project's infrastructure scope, structure, and design.

## Core Principles

### 1. Reference, Don't Duplicate

Point to source-of-truth files rather than copying content that will drift:

```markdown
<!-- GOOD: Reference the source -->
Backend endpoints are configured in:
`services/api-router-service/deployments/helm/api-router-service/values-development.yaml`

<!-- BAD: Duplicating config that will drift -->
Backend endpoints:
- vllm-gpt-oss-20b: http://gpt-oss-20b-vllm.system.svc.cluster.local:8000
- ...
```

### 2. Source of Truth Locations

| Information Type | Source Location |
|-----------------|-----------------|
| Service configurations | `services/<name>/deployments/helm/<name>/values-*.yaml` |
| ArgoCD applications | `gitops/clusters/<env>/apps/*.yaml` |
| Infrastructure definitions | `infra/terraform/environments/<env>/` |
| GitHub workflows | `.github/workflows/*.yml` |
| Kubernetes resources | Query cluster directly or reference Helm charts |
| Branching strategy | `docs/platform/branching-workflow.md` |
| Credentials/access | `docs/platform/environment-access.md` (paths only, not values) |

### 3. Verification Commands

For infrastructure state documentation, include commands to verify current state:

```markdown
## Ingress Configuration

Configured ingresses can be verified with:
```bash
kubectl get ingress -A
```

See `gitops/clusters/development/apps/` for ArgoCD application definitions.
```

### 4. Document Types

Mark each document with its type in the frontmatter:

| Type | Purpose | Example |
|------|---------|---------|
| `reference` | Quick lookup tables, paths, endpoints | environment-access.md |
| `guide` | How to accomplish tasks | tls-ssl-setup.md |
| `overview` | Architecture/design explanations | infrastructure-overview.md |

## Required Frontmatter

Every document must include:

```yaml
---
title: Document Title
last_updated: YYYY-MM-DD
document_type: reference | guide | overview
---
```

For documents describing infrastructure state, add:

```yaml
---
last_verified: YYYY-MM-DD
verification_command: "kubectl get ingress -A"
---
```

## File Naming

- Use kebab-case: `infrastructure-overview.md`
- Be descriptive: `ci-cd-pipeline.md` not `ci.md`
- Group related docs with common prefixes if needed

## Cross-References

When referencing other documentation:

```markdown
<!-- GOOD: Relative path that can be validated -->
See [Branching Workflow](branching-workflow.md)

<!-- BAD: Absolute URL that may break -->
See https://github.com/org/repo/docs/platform/branching-workflow.md
```

## What NOT to Include

1. **Sensitive values** - Reference paths to secrets, not the secrets themselves
2. **Duplicated config** - Point to Helm values, don't copy them
3. **Volatile state** - Don't hardcode pod names, IPs that change
4. **Historical design docs** - Don't reference `specs/` directory (it contains outdated design specs)

## Review Checklist

Before committing documentation changes:

- [ ] Frontmatter includes `last_updated` date
- [ ] No duplicated configuration (references source files instead)
- [ ] All file path references are valid
- [ ] All cross-document links work
- [ ] Verification commands included where appropriate
- [ ] Document type is accurate (not design doc masquerading as operational)

## Navigation

**For AI agents**: Start with [agent-infra-ops-manager.md](agent-infra-ops-manager.md) - comprehensive document map with quick navigation, source-of-truth locations, and common task guides.

### Quick Task Reference

| Task | Start With |
|------|-----------|
| **Full document map** | [agent-infra-ops-manager.md](agent-infra-ops-manager.md) |
| Understanding infrastructure | [infrastructure-overview.md](infrastructure-overview.md) |
| Accessing environments | [environment-access.md](environment-access.md) |
| Finding endpoints/URLs | [endpoints-and-urls.md](endpoints-and-urls.md) |
| CI/CD pipeline | [ci-cd-pipeline.md](ci-cd-pipeline.md) |
| TLS/certificates | [tls-ssl-setup.md](tls-ssl-setup.md), [certificate-architecture.md](certificate-architecture.md) |
| Observability | [observability-guide.md](observability-guide.md) |
| GitHub Actions | [github-actions-guide.md](github-actions-guide.md) |
| Provisioning/auditing environments | [new-environment-checklist.md](new-environment-checklist.md) |
