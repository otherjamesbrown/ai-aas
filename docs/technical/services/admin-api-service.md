# Admin API Service

## Overview

The Admin API Service is an internal HTTP API that provides administrative functionalities for the AI-AAS platform. It is used by the `admin-cli` to manage the model registry, organizations, and routing policies.

## Quick Start

To run the service locally, follow these steps:

1.  **Configure Environment:**
    Copy the example environment file and edit it with your database credentials.

    ```bash
    cp services/admin-api-service/config.example.env .env
    ```

2.  **Run the Service:**
    ```bash
    go run ./services/admin-api-service/cmd/admin-api
    ```

3.  **Build the Service:**
    ```bash
    go build -o admin-api ./services/admin-api-service/cmd/admin-api
    ```

## API Endpoints

The API is versioned under `/v1`.

### Model Registry

*   **POST /v1/registry/models**
    *   **Description:** Register a new model.
    *   **Request Body:** Model object in JSON format.
*   **GET /v1/registry/models**
    *   **Description:** List all registered models.
*   **GET /v1/registry/models/{name}**
    *   **Description:** Get a model by its name.
*   **PATCH /v1/registry/models/{name}**
    *   **Description:** Update a model's information.
    *   **Request Body:** Partial model object in JSON format.
*   **DELETE /v1/registry/models/{name}**
    *   **Description:** Delete a model from the registry.

### Organizations

*   **POST /v1/organizations**
    *   **Description:** Create a new organization.
    *   **Request Body:** Organization object in JSON format.
*   **GET /v1/organizations**
    *   **Description:** List all organizations.
*   **GET /v1/organizations/{id}**
    *   **Description:** Get an organization by its ID.
*   **PATCH /v1/organizations/{id}**
    *   **Description:** Update an organization's information.
    *   **Request Body:** Partial organization object in JSON format.

### Routing Policies

*   **POST /v1/routing/policies**
    *   **Description:** Create a new routing policy.
    *   **Request Body:** Policy object in JSON format.
*   **GET /v1/routing/policies**
    *   **Description:** List all routing policies.
*   **GET /v1/routing/policies/{id}**
    *   **Description:** Get a policy by its ID.
*   **PATCH /v1/routing/policies/{id}**
    *   **Description:** Update a policy.
    *   **Request Body:** Partial policy object in JSON format.
*   **DELETE /v1/routing/policies/{id}**
    *   **Description:** Delete a policy.
*   **POST /v1/routing/policies/{id}/activate**
    *   **Description:** Activate a policy.
*   **POST /v1/routing/policies/{id}/deactivate**
    *   **Description:** Deactivate a policy.
*   **GET /v1/routing/policies/sync**
    *   **Description:** Sync endpoint for the `api-router-service`.
*   **POST /v1/routing/policies/validate**
    *   **Description:** Validate a policy configuration.

### Audit & System

*   **GET /v1/audit-logs**
    *   **Description:** Query audit logs.
*   **GET /healthz**
    *   **Description:** Liveness probe for health checks.
*   **GET /readyz**
    *   **Description:** Readiness probe for health checks.
*   **GET /metrics**
    *   **Description:** Prometheus metrics endpoint.

## Authentication

All endpoints under `/v1/` require API key authentication. The API key must be provided in the `X-API-Key` header of the request.

## Configuration

For a complete list of configuration options, please refer to the `config.example.env` file in the service's root directory.

## Kubernetes Deployment

To deploy the service to a Kubernetes cluster, apply the manifests located in the `k8s` directory:

```bash
kubectl apply -f services/admin-api-service/k8s/
```
