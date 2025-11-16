# Analytics Service

The `analytics-service` is responsible for consuming usage data from Kafka to provide analytics and billing information.

## Purpose

This service provides insights into platform usage and is a key component of the billing system.

## Running the Service

To run the service locally, you first need to start the local development environment:

```bash
make up
```

Then, you can run the service with the following command:

```bash
go run ./services/analytics-service/cmd/server
```

## Running Tests

To run the tests for this service, use the following command:

```bash
make test SERVICE=analytics-service
```