# AI-as-a-Service Platform

[![CI](https://github.com/otherjamesbrown/ai-aas/actions/workflows/ci.yml/badge.svg)](https://github.com/otherjamesbrown/ai-aas/actions/workflows/ci.yml)

This repository contains the source code for a spec-driven, inference-as-a-service platform built on a Go-based microservices architecture.

## Overview

The platform provides a scalable and reliable way to serve AI models for inference. It is designed to be highly available, secure, and easy to operate. The central component is the `api-router-service`, which acts as a gateway for all inference requests, handling authentication, rate limiting, and routing to the appropriate backend model services.

For a detailed explanation of the system's architecture, please see the [ARCHITECTURE.md](./ARCHITECTURE.md) file.

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
|---|---|
| `ARCHITECTURE.md` | A high-level overview of the system architecture. |
| `CONTRIBUTING.md` | Guidelines for contributing to the project. |
| `Makefile` | The main entry point for all automation (build, test, etc.). |
| `services/` | The source code for each of the microservices. |
| `shared/` | Shared libraries used by multiple services. |
| `docs/` | Detailed documentation, including runbooks and setup guides. |
| `specs/` | The feature specifications and design documents. |
| `usage-guide/` | Documentation for end-users of the platform, organized by role. |

## CI/CD

Our CI/CD pipeline is powered by GitHub Actions and ArgoCD. For a detailed explanation of the pipeline, please see the [CI/CD Pipeline document](./docs/platform/ci-cd-pipeline.md).

## Key Commands

Here are some of the most common commands you will use during development:

*   `make help`: Display all available `make` targets.
*   `make check`: Run all checks (format, lint, security, test).
*   `make build`: Build the services.
*   `make test`: Run unit tests.
*   `make up`: Start the local development environment.
*   `make stop`: Stop the local development environment.

## Contributing

We welcome contributions to the project. Before you start, please read our [CONTRIBUTING.md](./CONTRIBUTING.md) file for guidelines on our development process, coding style, and pull request submission.