# Documentation

> [!NOTE]
> This directory (`docs/`) is optimized for **AI Coding Assistants** (e.g., `go-services-developer`, `infra-ops-manager`).
>
> **Human-readable documentation** has been moved to **[`../docs-humans/`](../docs-humans/)**.

## Directory Structure

### Core Folders

| Folder | Purpose | Agent |
|--------|---------|-------|
| [`/architecture`](architecture/) | System architecture, diagrams, design decisions | All |
| [`/getting-started`](getting-started/) | Onboarding guides for new users | External users |
| [`/go-services`](go-services/) | Go service development patterns and guides | `go-services-developer` |
| [`/operators`](operators/) | Kubernetes operator documentation | `operator-developer` |
| [`/platform`](platform/) | Infrastructure, deployment, security, CI/CD | `infra-ops-manager` |
| [`/runbooks`](runbooks/) | Operational runbooks and workflows | All |
| [`/troubleshooting`](troubleshooting/) | Troubleshooting guides for common issues | All |
| [`/monitoring`](monitoring/) | Observability stack, alerts, SLOs | `infra-ops-manager` |

### Specialized Folders

| Folder | Purpose |
|--------|---------|
| [`/admin`](admin/) | Admin-specific setup and configuration |
| [`/arch-review`](arch-review/) | Architecture review templates and records |
| [`/testing`](testing/) | Test coverage, E2E testing guides |

## Agent Navigation Maps

* **Go Services Developer**: [`go-services/agent-go-services-developer.md`](go-services/agent-go-services-developer.md)
* **Infrastructure Operations Manager**: [`platform/agent-infra-ops-manager.md`](platform/agent-infra-ops-manager.md)

## Quick Reference

### Finding Documentation

| Looking for... | Go to... |
|----------------|----------|
| How to deploy a service | `/runbooks/deploy-to-environments.md` |
| Environment access/credentials | `/platform/environment-access.md` |
| Debugging AI workflows | `/runbooks/ai-debugging-workflow.md` |
| Go service patterns | `/go-services/` |
| ArgoCD workflows | `/runbooks/argocd-*.md` |
| TLS/SSL setup | `/platform/tls-ssl-setup.md` |
| CI/CD pipeline | `/platform/ci-cd-pipeline.md` |
| vLLM issues | `/troubleshooting/vllm-*.md` |
