# Quickstart: ai-aas-cli

Get started with model management in 5 minutes.

## Prerequisites

- Access to the AI-AAS platform (API endpoint and admin key)
- HuggingFace account with accepted licenses for gated models
- kubectl configured for your cluster (for troubleshooting)

## Installation

Download the CLI for your platform:

```bash
# macOS (Apple Silicon)
curl -L https://releases.ai-aas.io/cli/latest/ai-aas-cli-darwin-arm64 -o ai-aas-cli
chmod +x ai-aas-cli

# macOS (Intel)
curl -L https://releases.ai-aas.io/cli/latest/ai-aas-cli-darwin-amd64 -o ai-aas-cli
chmod +x ai-aas-cli

# Linux
curl -L https://releases.ai-aas.io/cli/latest/ai-aas-cli-linux-amd64 -o ai-aas-cli
chmod +x ai-aas-cli

# Move to a location in your PATH
sudo mv ai-aas-cli /usr/local/bin/
```

## Initialize the CLI

Run the initialization wizard:

```bash
ai-aas-cli --init
```

The wizard will prompt for:
1. **API Endpoint**: Your platform's admin API URL
2. **API Key**: Your admin API key
3. **Default Environment**: development, staging, or production
4. **HuggingFace Token** (optional): For accessing gated models

### Non-Interactive Setup (CI/CD)

```bash
ai-aas-cli --init \
  --api-key "ak_your_key_here" \
  --endpoint "https://api.ai-aas.example.com" \
  --environment development \
  --hf-token "hf_your_token_here"
```

## Verify Setup

```bash
# Test all connections
ai-aas-cli config test

# Show current configuration
ai-aas-cli config show
```

## Your First Model

### 1. Register a Model

```bash
# Register Llama 3 8B (gated model - requires HF license acceptance)
ai-aas-cli model add meta-llama/Llama-3-8B-Instruct --name llama-3-8b

# For CI/CD with pre-accepted license
ai-aas-cli model add meta-llama/Llama-3-8B-Instruct --name llama-3-8b --accept-license
```

### 2. Set Up Credentials

```bash
# Add HuggingFace token (if not done during init)
ai-aas-cli credentials set hf-token hf_your_token_here

# Test the token
ai-aas-cli credentials test hf-token
```

### 3. Cache the Model

```bash
# Pull model to object storage (may take 10-30 minutes for large models)
ai-aas-cli model pull llama-3-8b

# Check cache status
ai-aas-cli model cache list
```

### 4. Deploy the Model

```bash
# Deploy to development environment
ai-aas-cli model deploy llama-3-8b --environment development

# Watch deployment progress
ai-aas-cli model status llama-3-8b --environment development
```

### 5. Validate the Deployment

```bash
# Run full validation
ai-aas-cli model validate llama-3-8b --environment development

# Quick inference test
ai-aas-cli model test llama-3-8b --prompt "Hello, how are you?"
```

## Common Workflows

### View All Models

```bash
# List all models
ai-aas-cli model list

# List with details
ai-aas-cli model list --format json

# Only deployed models
ai-aas-cli model list --deployed
```

### Library Management (Enable/Disable)

```bash
# View model library
ai-aas-cli model library

# Disable a model (keeps cache, removes deployment)
ai-aas-cli model disable llama-3-8b --environment production

# Re-enable (fast - uses cached model)
ai-aas-cli model enable llama-3-8b --environment production

# Swap models atomically
ai-aas-cli model swap llama-3-8b llama-3-70b --environment production
```

### Troubleshooting

```bash
# View logs
ai-aas-cli model logs llama-3-8b --environment development --tail 100

# View Kubernetes events
ai-aas-cli model events llama-3-8b

# Full deployment details
ai-aas-cli model describe llama-3-8b
```

### Model Updates

```bash
# Check for updates
ai-aas-cli model check-updates

# Update a model (pulls new version, rolling update)
ai-aas-cli model update llama-3-8b

# Pin to prevent updates
ai-aas-cli model pin llama-3-8b --version v1.0.0
```

## Aliases

Create shortcuts for models:

```bash
# Create alias
ai-aas-cli model alias create llama-latest --target llama-3-8b

# Use alias in commands
ai-aas-cli model info llama-latest

# Update alias target
ai-aas-cli model alias update llama-latest --target llama-3-70b
```

## Configuration Reference

Configuration is stored in `~/.ai-aas/config.yaml`:

```yaml
api:
  endpoint: https://api.ai-aas.example.com
  key: ak_xxx...
defaults:
  environment: development
```

### Environment Variables

Override config with environment variables:

```bash
export AI_AAS_API_ENDPOINT="https://api.ai-aas.example.com"
export AI_AAS_API_KEY="ak_xxx..."
export AI_AAS_ENVIRONMENT="production"
```

## Getting Help

```bash
# General help
ai-aas-cli --help

# Command-specific help
ai-aas-cli model --help
ai-aas-cli model deploy --help

# Version info
ai-aas-cli --version
```

## Next Steps

- Read the [full specification](./spec.md) for all features
- Review the [API contracts](./contracts/admin-api.yaml) for integration
- Check the [data model](./data-model.md) for database schema details

