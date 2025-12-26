# API Router and KServe Integration

**Last Updated**: 2025-11-27
**Status**: Active

## Overview

This document describes the integration between the API Router Service and KServe for serving models on the AI-AAS platform.

## Architecture

The API Router Service acts as the central gateway for all inference requests. It is responsible for authentication, rate limiting, and routing requests to the appropriate KServe `InferenceService`.

```
Internet
   ↓
Istio Ingress Gateway
   ↓
API Router Service
   ↓ (validates API key, gets model from registry)
Model Registry (PostgreSQL)
   ↓ (routes to KServe InferenceService URL)
KServe InferenceService (Istio-injected)
   ↓
vLLM Pod (managed by Knative)
```

### Key Components

- **API Router Service**: The single entry point for all model inference requests. It enforces authentication and authorization and uses the model registry to find the correct backend for a given model.
- **Model Registry**: A PostgreSQL database that stores metadata about each model, including its `backend_type` (`kserve` or `legacy_helm`) and its endpoint URL.
- **KServe `InferenceService`**: The custom resource that defines a model deployment. It manages the model server (e.g., vLLM), autoscaling (via Knative), and networking (via Istio).
- **Istio**: The service mesh that provides ingress, routing, and observability for KServe services.

### Request Flow

1. A client sends an inference request to the API Router's public endpoint.
2. The API Router authenticates the request using the provided API key.
3. It queries the model registry to find the endpoint for the requested model.
4. If the model's `backend_type` is `kserve`, the router forwards the request to the `InferenceService`'s URL. The router also performs protocol translation from the OpenAI-compatible API to KServe's V2 inference protocol.
5. Istio routes the request to a running vLLM pod. If no pod is running (scale-to-zero), Knative will cold-start one.
6. The vLLM pod processes the request and returns the response.
7. The API Router translates the response back to the OpenAI-compatible format and returns it to the client.

## Model Registration

For the API Router to be able to route requests to a KServe `InferenceService`, the model must be registered in the model registry.

See the [Model Registration Workflow](./model-registration-workflow.md) for detailed instructions.

## Testing

To test the integration, you can send a request to the API Router with a valid API key and the name of a KServe-deployed model.

```bash
curl -X POST https://api.dev.otherjamesbrown.com/v1/chat/completions \
  -H 'Authorization: Bearer <your-api-key>' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "my-kserve-model",
    "messages": [{"role": "user", "content": "What is the capital of France?"}]
  }'
```

## Troubleshooting

Refer to the [KServe Deployment Troubleshooting Guide](./troubleshooting/kserve-deployment-troubleshooting.md) for issues related to KServe deployments. For issues with the API Router, check its logs for errors related to model routing or authentication.

```