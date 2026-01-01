# AI Assistant Guide

Welcome to the AI-AAS platform! This guide is designed to help you, an AI coding assistant, understand the architecture, find key information, and effectively assist with development tasks.

## Core Principles

1.  **Spec-Driven Development**: This is a spec-driven repository. Always refer to the `specs/` directory for the relevant feature specification before making any changes.
2.  **Makefile is the Entry Point**: All automation is handled through the root `Makefile`. Use `make help` to see a list of available commands.

## Repository Overview

This is a mono-repo for a platform that provides AI as a service, built on a microservices architecture with a strong emphasis on GitOps.

- **`Makefile`**: The main entry point for all automation. Start here.
- **`services/`**: Contains the source code for the Go-based microservices. The `api-router-service` is the main entry point for all API requests.
- **`shared/`**: Contains shared Go libraries used by multiple services.
- **`infra/`**: Contains the infrastructure-as-code, including Terraform for provisioning and Helm charts for deploying services.
- **`specs/`**: Contains the design specifications for all features. **Consult these first.**
- **`docs/`**: Contains all the detailed documentation for the platform, including runbooks and guides.

## Development Workflow

1.  **Bootstrap the environment**: `make bootstrap`
2.  **Start the local stack**: `make up`
3.  **Run checks**: `make check` (format, lint, security)
4.  **Run tests**: `make test SERVICE=<service-name>`

## CLI-First Operations

**IMPORTANT**: Always prefer the `ai-aas-cli` for platform operations over direct API calls or kubectl commands.

### CLI Command Structure

The CLI uses nested subcommands organized by domain:

```bash
# Model Management
ai-aas-cli model registry add/list/info/remove     # Manage model registry
ai-aas-cli model cache pull/list/delete/gc         # Manage model cache
ai-aas-cli model deploy create/delete/scale/status # Manage deployments
ai-aas-cli model troubleshoot logs/events/test     # Debug deployments
ai-aas-cli model version check/update/pin          # Manage versions
ai-aas-cli model library enable/disable/swap       # Organization library

# Organization Management
ai-aas-cli org list/create/update/delete           # Manage organizations
ai-aas-cli user list/create/update/delete          # Manage users
ai-aas-cli apikey list/create/delete               # Manage API keys

# Platform Operations
ai-aas-cli credentials set/list/test               # Manage credentials
ai-aas-cli status                                  # Check platform health
ai-aas-cli config show/set/test                    # Manage CLI config
```

### Example Model Deployment Workflow

```bash
# 1. Register model from HuggingFace
ai-aas-cli model registry add mistralai/Mistral-7B-v0.1 --name mistral-7b

# 2. Cache model to object storage
ai-aas-cli model cache pull mistral-7b

# 3. Deploy model
ai-aas-cli model deploy create mistral-7b -e development

# 4. Check deployment status
ai-aas-cli model deploy status mistral-7b -e development

# 5. Test inference
ai-aas-cli model troubleshoot test mistral-7b -e development
```

### CLI Help

Every command has detailed help with examples and next steps:

```bash
ai-aas-cli --help                         # All commands
ai-aas-cli model --help                   # Model commands
ai-aas-cli model deploy --help            # Deploy subcommands
ai-aas-cli model deploy create --help     # Specific command details
```

### When CLI Doesn't Support an Operation

If you need to perform an operation that the CLI doesn't support:
1. Note it as a potential enhancement
2. Fall back to the Admin API
3. Consider creating a beads issue: `bd create "CLI: Add <command>" --type feature`

## Key Architectural Concepts

- **Model Serving**: The platform uses [KServe](https://kserve.github.io/website/) for model serving. Models are deployed as `InferenceService` custom resources in Kubernetes.
- **GitOps**: All deployments are managed via GitOps using [ArgoCD](https://argo-cd.readthedocs.io/en/stable/). Changes are made by committing manifests to the Git repository.
- **Microservices**: The backend is composed of several Go-based microservices that handle tasks like authentication, routing, and usage tracking.

## Accessing the Development Environment

- **URLs**: A complete list of service URLs for the development and production environments can be found in the [Endpoints and URLs Configuration Guide](./docs/platform/endpoints-and-urls.md).
- **Seeded Data**: Information on the seeded test users, organizations, and the Master Admin API key is available in the [Seeded Test Data Guide](./docs/seeded-data.md).

## Important Documents

Here are the most important documents to read to understand the platform, organized by topic.

### 1. Core Architecture & Concepts

*   **`ARCHITECTURE.md`**: A high-level overview of the system architecture and service interactions.
*   **`specs/` directory**: The single source of truth for all feature designs. Review the relevant spec before starting work.
*   **`docs/platform/infrastructure-overview.md`**: A detailed description of the platform's infrastructure (Kubernetes, networking, etc.).

### 2. Model Serving with KServe

This is the core functionality of the platform.

*   **`specs/016-kserve-migration/spec.md`**: The key specification for understanding the current model serving architecture and the migration to KServe.
*   **`docs/platform/api-router-kserve-integration.md`**: Explains how the API Router integrates with KServe.
*   **`docs/workflows/kserve-deployment-workflow.md`**: The main workflow for deploying models with KServe.
*   **`docs/best-practices/kserve-deployment-best-practices.md`**: Best practices for deploying models with KServe.
*   **`docs/troubleshooting/kserve-deployment-troubleshooting.md`**: A guide for troubleshooting KServe deployments.

### 3. Development & Operations

*   **`docs/runbooks/deploy-to-environments.md`**: The general workflow for deploying services.
*   **`docs/workflows/model-registration-workflow.md`**: How to register a new model so it can be served by the platform.
*   **`docs/guides/kserve-management-guide.md`**: A quick reference for managing KServe deployments.

### 4. Observability & Debugging

The platform has a unified observability stack for logs, traces, metrics, and error tracking:

*   **`docs/architecture/observability-architecture.md`**: Comprehensive architecture documentation for the observability stack (Loki, Tempo, Promtail, OTEL Collector, Grafana, Sentry).
*   **`docs/platform/observability-guide.md`**: Operational guide for logging standards, data flows, retention policies, and accessing dashboards.
*   **`docs/runbooks/ai-debugging-workflow.md`**: Debug workflow for AI assistants with LogQL commands and common scenarios.

**Quick Reference**:
- **Logs**: Access via Grafana (`https://grafana.dev.otherjamesbrown.com`) or kubectl
- **Traces**: Tempo integration with trace-to-logs correlation
- **Frontend Errors**: Sentry captures React errors with session replay
- **vLLM Logs**: Special dashboard for inference backend monitoring with GPU error detection

**Common Log Queries (LogQL)**:
```bash
# Find errors across all services
{namespace="system"} | json | level="error"

# Search by trace ID
{namespace="system"} | json | trace_id="abc123"

# vLLM GPU errors
{namespace=~"ai-models|system|kserve"} | json | gpu_error="true"
```

**Accessing Logs**:
```bash
# Real-time tailing (kubectl)
kubectl logs -n system -l app=api-router -f

# Historical search (Grafana)
# Navigate to Grafana → Explore → Loki datasource → Enter LogQL query
```

By familiarizing yourself with these documents and following the core principles, you will be well-equipped to answer questions and perform tasks related to the AI-AAS platform.
