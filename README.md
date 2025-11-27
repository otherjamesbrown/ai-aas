# AI-as-a-Service Platform

[![CI](https://github.com/otherjamesbrown/ai-aas/actions/workflows/ci.yml/badge.svg)](https://github.com/otherjamesbrown/ai-aas/actions/workflows/ci.yml)

This repository contains the source code for a spec-driven, inference-as-a-service platform built on a Go-based microservices architecture.

## Overview

The platform provides a scalable and reliable way to serve AI models for inference. It is designed to be highly available, secure, and easy to operate. The central component is the `api-router-service`, which acts as a gateway for all inference requests, handling authentication, rate limiting, and routing to the appropriate backend model services.

For a detailed explanation of the system's architecture, please see the [ARCHITECTURE.md](./ARCHITECTURE.md) file.

---

**👋 Note for AI Assistants:**

This project is designed with AI assistance in mind. For a comprehensive guide tailored to AI coding assistants, covering architectural concepts, development workflows, key documents, and specific operational guidelines, please refer to:

➡️ **[AI Assistant Guide](./AI_ASSISTANT_GUIDE.md)**

---

## Getting Started

To get started with the project, follow these steps:

1.  **Clone the repository:**
    ```bash
    git clone git@github.com:otherjamesbrown/ai-aas.git
    cd ai-aas
    ```

2.  **Bootstrap the development environment:**
    ```bash
    ./scripts/setup/bootstrap.sh
    ```

3.  **Start the local development stack:**
    ```bash
    make up
    ```

4.  **Run the checks to ensure everything is working:**
    ```bash
    make check
    ```

For more detailed instructions on setting up your development environment, please see the [Developer Onboarding Guide](./docs/setup/developer-onboarding.md).

## Repository Structure

Here is a high-level overview of the key directories in this repository:

| Path | Description |
|---|
| `Makefile` | The main entry point for all automation (build, test, etc.). |
| `services/` | The source code for each of the microservices. |
| `shared/` | Shared libraries used by multiple services. |
| `docs/` | Detailed documentation, including runbooks and setup guides. |
| `specs/` | The feature specifications and design documents. |
| `infra/` | Infrastructure-as-code for the platform (Terraform, Helm). |
| `AI_ASSISTANT_GUIDE.md` | Comprehensive guide tailored for AI coding assistants. |
| `ARCHITECTURE.md` | High-level overview of the system architecture. |
| `CONTRIBUTING.md` | Guidelines for contributing to the project. |
| `usage-guide/` | Documentation for end-users of the platform, organized by role. |


## CI/CD

Our CI/CD pipeline is powered by GitHub Actions and ArgoCD. For a detailed explanation of the pipeline, please see the [CI/CD Pipeline document](./docs/platform/ci-cd-pipeline.md).

## Contributing

We welcome contributions to the project. Before you start, please read our [CONTRIBUTING.md](./CONTRIBUTING.md) file for guidelines on our development process, coding style, and pull request submission.