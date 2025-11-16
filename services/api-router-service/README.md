# API Router Service

The `api-router-service` is the main entry point for all API requests. It is responsible for routing, authentication, rate limiting, and more.

## Purpose

This service provides the primary entrypoint for inference requests, routing them to appropriate model backends while enforcing authentication, budgets, quotas, and usage tracking.

## Running the Service

To run the service locally, you first need to start the local development environment:

```bash
make up
```

Then, you can run the service with the following command:

```bash
go run ./services/api-router-service/cmd/router
```

## Running Tests

To run the tests for this service, use the following command:

```bash
make test SERVICE=api-router-service
```