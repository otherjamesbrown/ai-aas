# World Service

The `world-service` is an example service that demonstrates how to build a new service within the platform's architecture.

## Purpose

This service is a template for creating new services. It shows how to use the shared libraries and how to integrate with the platform's infrastructure.

## Running the Service

To run the service locally, you first need to start the local development environment:

```bash
make up
```

Then, you can run the service with the following command:

```bash
go run ./services/world-service/cmd/server
```

## Running Tests

To run the tests for this service, use the following command:

```bash
make test SERVICE=world-service
```