# API Router Service

## Overview

The `api-router-service` is the main entry point for all API requests. It is responsible for routing, authentication, rate limiting, and more. This service provides the primary entrypoint for inference requests, routing them to appropriate model backends while enforcing authentication, budgets, quotas, and usage tracking.

## Quick Start

To run the service locally, you first need to start the local development environment:

```bash
make up
```

Then, you can run the service with the following command:

```bash
go run ./services/api-router-service/cmd/router
```

## Configuration

The configuration for the `api-router-service` is managed through environment variables. A complete list of these variables can be inferred from the source code and deployment files.

## Testing

To run the tests for this service, use the following command:

```bash
make test SERVICE=api-router-service
```
