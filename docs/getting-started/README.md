# AI-AAS Platform User Guide

Welcome to the AI-AAS (AI as a Service) platform. This guide will help you get started with accessing AI inference capabilities through our OpenAI-compatible API.

## What is AI-AAS?

AI-AAS is a managed platform for deploying and accessing large language models (LLMs). It provides:

- **OpenAI-compatible API** - Use familiar SDKs and tools
- **Multi-tenant isolation** - Organizations, users, and API keys
- **Usage tracking** - Monitor token consumption and costs
- **Rate limiting** - Fair resource allocation

## Architecture at a Glance

```
Your App  ──▶  API Router  ──▶  vLLM Model Backends (GPU)
                   │
                   ├── Auth (User-Org Service)
                   ├── Usage tracking (Analytics)
                   └── Model registry (Admin API)
```

**Core Components**:

| Component | Purpose |
|-----------|---------|
| **API Router** | Gateway - authenticates requests, routes to models |
| **User-Org Service** | Manages organizations, users, and API keys |
| **AI Model Operator** | Kubernetes operator that deploys and manages models |
| **vLLM Backends** | GPU-accelerated inference engines running LLMs |
| **Analytics Service** | Tracks usage for billing and reporting |

For the complete architecture including multi-model deployment, the operator pattern, request flow details, and GitOps workflow, see **[Architecture Deep Dive](./architecture.md)**.

## Quick Links

| Guide | Description | Time |
|-------|-------------|------|
| [Quickstart](./01-quickstart.md) | Make your first API call | 5 min |
| [Architecture](./architecture.md) | How the platform works (operators, multi-model, GitOps) | 15 min |
| [CLI Setup](./02-cli-setup.md) | Install CLI, create org/users/keys | 15 min |
| [Available Models](./03-available-models.md) | See what models you can use | 2 min |
| [Making Requests](./04-making-requests.md) | API examples in curl, Python, Node.js | 10 min |
| [Usage & Limits](./05-usage-and-limits.md) | Monitor tokens, understand rate limits | 5 min |
| [Troubleshooting](./06-troubleshooting.md) | Common errors and solutions | As needed |

## API Endpoint

| Environment | Base URL |
|-------------|----------|
| Development | `https://api.dev.otherjamesbrown.com` |

All endpoints follow OpenAI's API specification under `/v1/`.

## Authentication

All API requests require an API key passed in the `X-API-Key` header:

```bash
curl https://api.dev.otherjamesbrown.com/v1/chat/completions \
  -H "X-API-Key: your-api-key-here" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-oss-20b", "messages": [{"role": "user", "content": "Hello"}]}'
```

## Getting an API Key

1. **Organization Admin**: Use the `ai-aas-cli` to create organizations, users, and API keys
2. **Team Member**: Request an API key from your organization administrator

See [CLI Setup](./02-cli-setup.md) for detailed instructions.

## Support

For issues or questions:
- Check [Troubleshooting](./06-troubleshooting.md) for common solutions
- Contact your platform administrator
