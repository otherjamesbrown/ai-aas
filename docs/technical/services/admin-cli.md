# Admin CLI

## Overview

The `admin-cli` is a command-line interface for administrators to manage the AI-AAS platform. It provides a convenient way to perform administrative tasks, such as managing users, organizations, API keys, routing policies, and model registry entries.

## Installation

To build the CLI:

```bash
cd services/admin-cli
go build -o ../../bin/admin-cli ./cmd/admin-cli
```

The binary will be available at `bin/admin-cli`.

## Commands

The `admin-cli` supports a variety of commands to manage the platform. For a full list of commands and their options, use the `--help` flag:

```bash
admin-cli --help
```

### Routing Policy Management

Manage routing policies that control how API requests are routed to vLLM model backends.

#### Create or Update Policy

*   **Global Policy:**
    ```bash
    admin-cli routing policy create \
      --global \
      --model gpt-oss-20b \
      --backends gpt-oss-20b-vllm-deployment:100
    ```

*   **Organization-Specific Policy:**
    ```bash
    admin-cli routing policy create \
      --org-id <org-uuid> \
      --model gpt-4-turbo \
      --backends "backend-1:70,backend-2:30"
    ```

**Options:**

*   `--global`: Create a global policy (mutually exclusive with `--org-id`).
*   `--org-id`: Organization ID for an organization-specific policy.
*   `--model`: Model name (required).
*   `--backends`: Comma-separated list of `backend_id:weight` pairs (required, weights must sum to 100).
*   `--format`: Output format (`table`, `json`).
*   `--quiet`: Suppress non-error output.
*   `--dry-run`: Simulate creation without applying changes.

#### List Policies

```bash
# List all policies in table format
admin-cli routing policy list

# List in JSON format
admin-cli routing policy list --format json
```

#### Delete Policy

```bash
# Delete a global policy
admin-cli routing policy delete --global --model gpt-oss-20b

# Delete an organization-specific policy
admin-cli routing policy delete --org-id <org-uuid> --model gpt-4-turbo
```

## Configuration

The CLI can be configured via a configuration file, environment variables, or command-line flags.

*   **Configuration File:** `~/.admin-cli/config.yaml`
*   **Environment Variables:** Prefix with `ADMIN_CLI_`
*   **Command-Line Flags:** Override all other sources.

### Default Configuration

The `admin-cli` is pre-configured with sensible defaults for the development environment. For etcd access, which is required for routing policy management, you need to run `kubectl port-forward`:

```bash
kubectl port-forward -n development svc/etcd-service 2379:2379
```

### Configuration File Example

If you need to override the defaults, create `~/.admin-cli/config.yaml`:

```yaml
api-endpoints:
  user-org-service: "https://user-org.172.232.58.222.nip.io"
  analytics-service: "http://localhost:8084"
  config-service: "localhost:2379"

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

You can also override specific settings using environment variables:

```bash
export ADMIN_CLI_API_ENDPOINTS_USER_ORG_SERVICE="https://user-org.172.232.58.222.nip.io"
export ADMIN_CLI_AUTH_API_KEY="your-admin-api-key"
```

## Testing

To run the tests for this service, use the following command:

```bash
make test SERVICE=admin-cli
```
