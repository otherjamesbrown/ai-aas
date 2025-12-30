# AI-AAS CLI

A comprehensive command-line interface for managing the AI-AAS Platform.

## Overview

The AI-AAS CLI (`ai-aas-cli`) provides a unified interface for managing AI model deployments, organizations, users, and platform configuration. It follows GitOps principles and wraps the Admin API for consistent platform operations.

## Installation

### From Source

```bash
# Clone repository
git clone https://github.com/otherjamesbrown/ai-aas.git
cd ai-aas/services/ai-aas-cli

# Build
make build

# Install to system path
sudo make install
```

### Binary Release

```bash
# Download latest release
curl -LO https://github.com/otherjamesbrown/ai-aas/releases/latest/download/ai-aas-cli-linux-amd64

# Make executable
chmod +x ai-aas-cli-linux-amd64

# Move to PATH
sudo mv ai-aas-cli-linux-amd64 /usr/local/bin/ai-aas-cli
```

## Quick Start

```bash
# Configure CLI
ai-aas-cli config set api-endpoint https://api.dev.otherjamesbrown.com
ai-aas-cli config set api-key <your-api-key>

# Test connection
ai-aas-cli status

# List available models
ai-aas-cli model registry list

# Deploy a model
ai-aas-cli model deploy create mistral-7b -e development
```

## Command Overview

### Model Commands

Manage AI model deployments throughout their lifecycle:

```bash
# Registry management
ai-aas-cli model registry add mistralai/Mistral-7B-v0.1 --name mistral-7b
ai-aas-cli model registry list
ai-aas-cli model registry info mistral-7b
ai-aas-cli model registry remove mistral-7b

# Cache management
ai-aas-cli model cache pull mistral-7b
ai-aas-cli model cache list
ai-aas-cli model cache delete mistral-7b
ai-aas-cli model cache gc  # Garbage collect unused models

# Deployment management
ai-aas-cli model deploy create mistral-7b -e development
ai-aas-cli model deploy create mistral-7b -e development --replicas 2
ai-aas-cli model deploy scale mistral-7b -e development --replicas 3
ai-aas-cli model deploy status mistral-7b -e development
ai-aas-cli model deploy delete mistral-7b -e development

# Troubleshooting
ai-aas-cli model troubleshoot logs mistral-7b -e development
ai-aas-cli model troubleshoot events mistral-7b -e development
ai-aas-cli model troubleshoot test mistral-7b -e development

# Version management
ai-aas-cli model version check mistral-7b
ai-aas-cli model version update mistral-7b --version latest
ai-aas-cli model version pin mistral-7b --version v0.1.0

# Organization library
ai-aas-cli model library enable mistral-7b --org acme-corp
ai-aas-cli model library disable mistral-7b --org acme-corp
ai-aas-cli model library swap old-model new-model --org acme-corp
```

## Recipe Commands

Manage model deployment recipes:

```bash
# List available recipes
ai-aas-cli model recipe list
ai-aas-cli model recipe list --runtime vllm

# Show recipe details
ai-aas-cli model recipe show mistral-7b-instruct-v03

# Validate a recipe file
ai-aas-cli model recipe validate my-recipe.yaml

# Deploy using a recipe
ai-aas-cli model deploy create my-model -e development --recipe mistral-7b-instruct-v03

# Override recipe parameters
ai-aas-cli model deploy create my-model -e development \
  --recipe mistral-7b-instruct-v03 \
  --gpu-count 2 \
  --min-replicas 2 \
  --max-replicas 3
```

### Recipes provide:
- **Validated configurations**: Pre-tested resource requirements and runtime settings
- **Quick deployment**: Deploy models with known-good parameters
- **Consistency**: Standardized configurations across environments
- **Flexibility**: Override specific parameters while keeping the base recipe

### Organization Commands

Manage organizations (tenants):

```bash
ai-aas-cli org list
ai-aas-cli org create acme-corp --display-name "ACME Corporation"
ai-aas-cli org update acme-corp --display-name "ACME Corp"
ai-aas-cli org delete acme-corp
```

### User Commands

Manage user accounts:

```bash
# List users
ai-aas-cli user list --org-id acme-corp

# Create user (invite mode - sends email)
ai-aas-cli user create --org-id acme-corp --email john.doe@acme.com

# Create user directly with temporary password
ai-aas-cli user create --org-id acme-corp --email john.doe@acme.com --direct

# Idempotent user creation (won't fail if user exists)
ai-aas-cli user create --org-id acme-corp --email john.doe@acme.com --upsert

# Update user
ai-aas-cli user update john.doe@acme.com --role member

# Delete user
ai-aas-cli user delete john.doe@acme.com
```

#### Idempotent User Creation with --upsert

The `--upsert` flag makes user creation idempotent, which is essential for automation and scripting:

**Problem it solves:**
Without `--upsert`, running `user create` for an existing user returns a 409 Conflict error and fails.

**How it works:**
- If user doesn't exist: Creates the user normally
- If user already exists: Returns the existing user without error

**When to use --upsert:**

1. **Automation scripts** - Ensure users exist without conditional logic:
   ```bash
   # Script doesn't need to check if user exists first
   ai-aas-cli user create --org-id acme --email ops@acme.com --upsert --direct
   ```

2. **CI/CD pipelines** - Idempotent setup for test environments:
   ```bash
   # Safe to run multiple times in deployment scripts
   ai-aas-cli user create --org-id test-org --email ci-test@example.com --upsert
   ```

3. **Configuration management** - Declarative user provisioning:
   ```bash
   # Ansible/Terraform-style idempotent operations
   for email in admin@acme.com dev@acme.com; do
     ai-aas-cli user create --org-id acme --email "$email" --upsert --direct
   done
   ```

**Important notes:**
- Works with both invite mode (default) and direct mode (`--direct`)
- Does NOT update existing users - only skips creation
- Returns existing user details when user already exists
- No password reset for existing users in direct mode

### API Key Commands

Manage API keys for programmatic access:

```bash
ai-aas-cli apikey list
ai-aas-cli apikey create --name "Production API Key" --org acme-corp
ai-aas-cli apikey delete <key-id>
```

### Credential Commands

Manage platform credentials (S3, HuggingFace, etc.):

```bash
ai-aas-cli credentials set s3 --access-key <key> --secret-key <secret>
ai-aas-cli credentials set huggingface --token <token>
ai-aas-cli credentials list
ai-aas-cli credentials test s3
```

### Platform Commands

Platform status and configuration:

```bash
# Check platform health
ai-aas-cli status

# CLI configuration
ai-aas-cli config show
ai-aas-cli config set api-endpoint https://api.prod.otherjamesbrown.com
ai-aas-cli config set api-key <your-api-key>
ai-aas-cli config test
```

## Configuration

The CLI uses configuration files stored in `~/.config/ai-aas-cli/`:

```yaml
# config.yaml
api_endpoint: https://api.dev.otherjamesbrown.com
api_key: sk-xxxxx
default_environment: development
timeout: 30s
```

### Configuration Precedence

1. Command-line flags (`--api-endpoint`, `--api-key`)
2. Environment variables (`AI_AAS_API_ENDPOINT`, `AI_AAS_API_KEY`)
3. Configuration file (`~/.config/ai-aas-cli/config.yaml`)
4. Default values

### Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `AI_AAS_API_ENDPOINT` | Admin API endpoint | `https://api.dev.otherjamesbrown.com` |
| `AI_AAS_API_KEY` | API authentication key | `sk-xxxxx` |
| `AI_AAS_DEFAULT_ENV` | Default environment | `development` |
| `AI_AAS_CONFIG_PATH` | Config file path | `~/.config/ai-aas-cli/config.yaml` |

## Development

### Prerequisites

- Go 1.22+
- Make
- Docker (for testing)

### Building

```bash
# Install dependencies
go mod download

# Build binary
make build

# Run tests
make test

# Run linter
make lint

# Build Docker image
make docker-build
```

### Project Structure

```
services/ai-aas-cli/
├── cmd/                        # Command definitions
│   ├── root.go                 # Root command
│   ├── model/                  # Model commands
│   │   ├── registry.go
│   │   ├── cache.go
│   │   ├── deploy.go
│   │   ├── troubleshoot.go
│   │   └── recipe.go           # Recipe commands
│   ├── org.go                  # Organization commands
│   ├── user.go                 # User commands
│   ├── apikey.go               # API key commands
│   ├── credentials.go          # Credential commands
│   └── config.go               # Config commands
├── internal/                   # Internal packages
│   ├── api/                    # Admin API client
│   ├── registry/               # Model registry client
│   ├── kubernetes/             # Kubernetes client
│   ├── config/                 # Configuration management
│   └── output/                 # Output formatting (table, JSON)
├── tests/                      # Tests
│   ├── unit/                   # Unit tests
│   └── e2e/                    # End-to-end tests
├── main.go                     # CLI entrypoint
├── Makefile                    # Build automation
└── README.md                   # This file
```

## Architecture

### API-First Design

The CLI is a **thin client** that delegates all business logic to the Admin API:

```
ai-aas-cli → Admin API → [Services/Database/Kubernetes]
```

**Key Principles:**
- **No direct database access**: All operations go through the Admin API
- **No business logic**: CLI only handles user input, formatting, and API calls
- **Consistent operations**: All platform operations use the same API endpoints

### Example: Model Deployment

```go
// CORRECT: Use API client
apiClient := api.NewClient(cfg.APIEndpoint, cfg.APIKey)
regClient := registry.NewClient(apiClient)
model, err := regClient.Deploy(ctx, deploymentRequest)

// WRONG: Direct Kubernetes operations
kubectl.Apply(deploymentManifest)  // ❌ Bypasses API layer
```

## Testing

### Unit Tests

```bash
# Run all unit tests
make test

# Run with coverage
make test-coverage

# Run specific package tests
go test ./internal/api -v
```

### End-to-End Tests

```bash
# Start test environment
make test-e2e-setup

# Run e2e tests
make test-e2e

# Cleanup
make test-e2e-cleanup
```

## Troubleshooting

### Connection Issues

```bash
# Verify API endpoint is reachable
curl -k https://api.dev.otherjamesbrown.com/health

# Test CLI configuration
ai-aas-cli config test

# Use verbose mode for debugging
ai-aas-cli --verbose model registry list
```

### Authentication Issues

```bash
# Check API key is valid
ai-aas-cli config show

# Verify API key has correct permissions
ai-aas-cli apikey list
```

### User Already Exists (409 Conflict)

**Error message:**
```
Error: User with email 'user@example.com' already exists in organization 'acme-corp'
API Response: 409 Conflict
```

**Solution:**
Use the `--upsert` flag to make the operation idempotent:

```bash
# Instead of this (fails if user exists):
ai-aas-cli user create --org-id acme-corp --email user@example.com

# Use this (succeeds whether user exists or not):
ai-aas-cli user create --org-id acme-corp --email user@example.com --upsert
```

The `--upsert` flag is especially useful in:
- Automation scripts
- CI/CD pipelines
- Configuration management tools
- Test environment setup

See the [User Commands](#user-commands) section for more details on `--upsert`.

### Command Not Found

```bash
# Verify installation
which ai-aas-cli

# Check PATH
echo $PATH

# Reinstall
sudo make install
```

## Contributing

### Code Style

- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use `gofmt` for formatting
- Run `golangci-lint` before committing
- Add tests for new features

### Pull Request Checklist

- [ ] Tests added/updated
- [ ] Documentation updated
- [ ] Code formatted (`make fmt`)
- [ ] Linter passed (`make lint`)
- [ ] Commit message follows convention

## Documentation

- [AI Assistant Guide](../../AI_ASSISTANT_GUIDE.md) - Platform overview and CLI usage
- [Admin API Documentation](../admin-api-service/README.md) - API reference
- [Model Operator Guide](../../docs/operators/ai-model-operator-guide.md) - Model deployment details

## License

Apache License 2.0 - See LICENSE file for details
