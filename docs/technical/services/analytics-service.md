# Analytics Service

## Overview

The `analytics-service` is responsible for consuming usage data from Kafka to provide analytics and billing information. This service provides insights into platform usage and is a key component of the billing system.

## Quick Start

To run the service locally, you first need to start the local development environment:

```bash
make up
```

Then, you can run the service with the following command:

```bash
go run ./services/analytics-service/cmd/server
```

## Configuration

The configuration for the `analytics-service` is managed through environment variables. A complete list of these variables can be inferred from the source code and deployment files.

## Testing

To run the tests for this service, use the following command:

```bash
make test SERVICE=analytics-service
```
