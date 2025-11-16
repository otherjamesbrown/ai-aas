# User & Organization Service

The `user-org-service` is responsible for managing all data related to users, organizations, API keys, and authentication.

## Purpose

This service provides a centralized way to manage users and organizations, and it is the source of truth for authentication and authorization.

## Running the Service

To run the service locally, you first need to start the local development environment:

```bash
make up
```

Then, you can run the service with the following command:

```bash
go run ./services/user-org-service/cmd/server
```

## Running Tests

To run the tests for this service, use the following command:

```bash
make test SERVICE=user-org-service
```