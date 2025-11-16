# Claude Rules for AI-AAS Platform

This document provides a set of rules and guidelines for interacting with the AI-AAS Platform repository.

## Core Principles

1.  **Spec-Driven Development**: This is a spec-driven repository. Always refer to the `specs/` directory for the relevant feature specification before making any changes.
2.  **Makefile is the Entry Point**: All automation is handled through the root `Makefile`. Use `make help` to see a list of available commands.
3.  **Microservices Architecture**: The platform is composed of multiple Go-based microservices located in the `services/` directory. The `api-router-service` is the central gateway.
4.  **Shared Libraries**: Common code is located in the `shared/` directory.

## Key Files and Directories

*   `ARCHITECTURE.md`: A high-level overview of the system architecture.
*   `CONTRIBUTING.md`: Guidelines for contributing to the project.
*   `Makefile`: The main entry point for all automation.
*   `services/`: The source code for each of the microservices.
*   `shared/`: Shared libraries used by multiple services.
*   `docs/`: Detailed documentation, including runbooks and setup guides.
*   `specs/`: The feature specifications and design documents.

## Development Workflow

1.  **Bootstrap the environment**: `make bootstrap`
2.  **Start the local stack**: `make up`
3.  **Run checks**: `make check`
4.  **Run tests**: `make test SERVICE=<service-name>`

## Important Commands

*   `make help`: List all available commands.
*   `make check`: Run all checks (format, lint, security, test).
*   `make build`: Build the services.
*   `make test`: Run unit tests.
*   `make up`: Start the local development environment.
*   `make stop`: Stop the local development environment.
