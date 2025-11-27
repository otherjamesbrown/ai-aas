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

## Key Architectural Concepts

- **Model Serving**: The platform uses [KServe](https://kserve.github.io/website/) for model serving. Models are deployed as `InferenceService` custom resources in Kubernetes.
- **GitOps**: All deployments are managed via GitOps using [ArgoCD](https://argo-cd.readthedocs.io/en/stable/). Changes are made by committing manifests to the Git repository.
- **Microservices**: The backend is composed of several Go-based microservices that handle tasks like authentication, routing, and usage tracking.

## Accessing the Development Environment

- **URLs**: A complete list of service URLs for the development and production environments can be found in the [Endpoints and URLs Configuration Guide](./docs/platform/endpoints-and-urls.md).
- **Seeded Data**: Information on the seeded test users, organizations, and the E2E Admin API key is available in the [Seeded Test Data Guide](./docs/seeded-data.md).

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

By familiarizing yourself with these documents and following the core principles, you will be well-equipped to answer questions and perform tasks related to the AI-AAS platform.
