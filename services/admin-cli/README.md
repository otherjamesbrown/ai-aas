# Admin CLI

The `admin-cli` is a command-line interface for administrators to manage the platform.

## Purpose

This CLI provides a convenient way to perform administrative tasks, such as:
- Creating and managing users and organizations
- Managing API keys
- Configuring routing policies for model inference
- Managing model registry entries
- Interacting with the inference API (list models, send requests)
- Performing credential rotation
- Triggering data synchronization
- Exporting analytics data

## Installation

Build the CLI:

```bash
cd services/admin-cli
go build -o ../../bin/admin-cli ./cmd/admin-cli
```

The binary will be available at `bin/admin-cli`.

## Commands

### Routing Policy Management

Manage routing policies that control how API requests are routed to vLLM model backends.

#### Create or Update Policy

Create a global policy (applies to all organizations):

```bash
admin-cli routing policy create \
  --global \
  --model gpt-oss-20b \
  --backends gpt-oss-20b-vllm-deployment:100
```

Create an organization-specific policy:

```bash
admin-cli routing policy create \
  --org-id <org-uuid> \
  --model gpt-4-turbo \
  --backends "backend-1:70,backend-2:30"
```

Options:
- `--global`: Create a global policy (mutually exclusive with `--org-id`)
- `--org-id`: Organization ID for org-specific policy
- `--model`: Model name (required)
- `--backends`: Comma-separated list of `backend_id:weight` pairs (required, weights must sum to 100)
- `--format`: Output format (table, json)
- `--quiet`: Suppress non-error output
- `--dry-run`: Simulate creation without applying changes

#### List Policies

```bash
# List all policies in table format
admin-cli routing policy list

# List in JSON format
admin-cli routing policy list --format json
```

#### Delete Policy

```bash
# Delete global policy
admin-cli routing policy delete --global --model gpt-oss-20b

# Delete org-specific policy
admin-cli routing policy delete --org-id <org-uuid> --model gpt-4-turbo
```

### Inference API

Interact with the inference API to list available models and send chat completion requests.

#### List Available Models

```bash
admin-cli inference get-models --api-key="YOUR_API_KEY"
```

Options:
- `--format`: Output format (`table`, `json`)
- `--inference-endpoint`: Override the inference API endpoint
- `--api-key`: API key for authentication

#### Send a Chat Request

```bash
admin-cli inference send-request "What is 2+2?" --api-key="YOUR_API_KEY"
```

Options:
- `--model`: Model to use (defaults to first available model)
- `--max-tokens`: Maximum tokens in response (default: 256)
- `--temperature`: Sampling temperature 0.0-1.0 (default: 0.7)
- `--system`: System prompt (default: "You are a helpful assistant.")
- `--format`: Output format (`table`, `json`)
- `--inference-endpoint`: Override the inference API endpoint
- `--api-key`: API key for authentication

Example with all options:

```bash
admin-cli inference send-request "Explain quantum computing" \
  --model vllm-gpt-oss-20b \
  --max-tokens 512 \
  --temperature 0.5 \
  --system "You are a physics professor." \
  --api-key="YOUR_API_KEY"
```

Output includes:
- Model used
- Response time (seconds)
- Token usage (prompt, completion, total)
- Model response content

### Other Commands

See full command documentation:

```bash
admin-cli --help
```

## Configuration

The CLI can be configured via:

1. **Configuration file**: `~/.admin-cli/config.yaml`
2. **Environment variables**: Prefix with `ADMIN_CLI_`
3. **Command-line flags**: Override all other sources

### Default Configuration (Minimal Setup Required)

The admin-cli is pre-configured with sensible defaults for the development environment. Most services are accessible via public HTTPS URLs without configuration.

**Default Service Endpoints:**
- **User-Org Service**: `https://user-org.172.232.58.222.nip.io` (public HTTPS, no setup needed)
- **Inference Service (API Router)**: `https://api.172.232.58.222.nip.io` (public HTTPS, no setup needed)
- **Config Service (etcd)**: `localhost:2379` (requires kubectl port-forward)
- **Analytics Service**: `http://localhost:8084` (not yet exposed)

**Setup Requirements:**

For etcd access (required for routing policy management), you need to run one kubectl port-forward:

```bash
kubectl port-forward -n development svc/etcd-service 2379:2379
```

Keep this running in a terminal while using the admin-cli.

**Benefits:**
- User-org-service works from any machine with internet access (no kubectl needed)
- Only one port-forward needed for etcd gRPC access
- HTTPS encryption for all HTTP service communication
- No configuration files or environment variables needed

**Why does etcd need port-forward?**
etcd uses gRPC protocol, which doesn't work well through standard NGINX ingress. While the etcd HTTP gateway is exposed via HTTPS at `https://etcd.172.232.58.222.nip.io`, the admin-cli uses the gRPC API which requires direct access via port-forward.

**When to Override Defaults:**
- Testing against a different environment (staging, production)
- Using a different cluster with a different LoadBalancer IP
- Running services locally for development

### Configuration File Example

If you need to override the defaults, create `~/.admin-cli/config.yaml`:

```yaml
api-endpoints:
  # Override defaults only if needed
  user-org-service: "https://user-org.172.232.58.222.nip.io"
  inference-service: "https://api.172.232.58.222.nip.io"
  analytics-service: "http://localhost:8084"
  config-service: "localhost:2379"  # etcd gRPC endpoint (requires port-forward)

auth:
  api-key: "your-admin-api-key"

database:
  url: "postgres://postgres:postgres@localhost:5432/ai_aas_operational?sslmode=disable"

defaults:
  output-format: "table"
  verbose: false
  quiet: false
```

### Environment Variables

Override specific endpoints as needed:

```bash
export ADMIN_CLI_API_ENDPOINTS_USER_ORG_SERVICE="https://user-org.172.232.58.222.nip.io"
export ADMIN_CLI_API_ENDPOINTS_INFERENCE_SERVICE="https://api.172.232.58.222.nip.io"
export ADMIN_CLI_API_ENDPOINTS_ANALYTICS_SERVICE="http://localhost:8084"
export ADMIN_CLI_API_ENDPOINTS_CONFIG_SERVICE="localhost:2379"
export ADMIN_CLI_AUTH_API_KEY="your-admin-api-key"
export ADMIN_CLI_DATABASE_URL="postgres://..."
```

### Using kubectl Port-Forward for User-Org Service (Optional)

If you prefer to use local port-forwarding for user-org-service instead of the public HTTPS URL:

```bash
# Forward user-org-service
kubectl port-forward -n user-org-service svc/user-org-service-development-user-org-service 8081:8081

# Override the endpoint to use localhost
export ADMIN_CLI_API_ENDPOINTS_USER_ORG_SERVICE="http://localhost:8081"
```

**Note:** The etcd port-forward (port 2379) is always required regardless of this choice, as etcd's gRPC protocol requires direct access.

## Running Tests

To run the tests for this service, use the following command:

```bash
make test SERVICE=admin-cli
```

## Documentation

- [Routing Policy Management Guide](../../docs/routing-policies.md)
- [vLLM Deployment Workflow](../../docs/deployment-workflow.md)