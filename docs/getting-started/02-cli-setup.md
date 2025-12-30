# CLI Setup: Managing Organizations, Users, and API Keys

The `ai-aas-cli` tool allows organization administrators to manage the platform.

## Installation

### Download the CLI

```bash
# Download the latest release (Linux amd64)
curl -L https://github.com/otherjamesbrown/ai-aas/releases/latest/download/ai-aas-cli-linux-amd64 \
  -o ai-aas-cli

# Make executable
chmod +x ai-aas-cli

# Move to PATH
sudo mv ai-aas-cli /usr/local/bin/
```

### Verify Installation

```bash
ai-aas-cli --version
```

## Initial Configuration

Run the interactive setup wizard:

```bash
ai-aas-cli --init
```

This will prompt you for:
- **Admin API Endpoint**: `https://admin-api.dev.otherjamesbrown.com`
- **API Key**: Your admin API key
- **Environment**: `development`, `staging`, or `production`

### Quick Configuration (Non-Interactive)

```bash
ai-aas-cli --init-domain dev.otherjamesbrown.com
ai-aas-cli --init-api-key your-admin-api-key
ai-aas-cli --init-environment development
```

### Verify Configuration

```bash
ai-aas-cli --init-status
ai-aas-cli config test
```

## Managing Organizations

### List Organizations

```bash
ai-aas-cli org list
```

### Create an Organization

```bash
ai-aas-cli org create --name "ACME Corporation" --slug acme
```

### Update Organization

```bash
ai-aas-cli org update --org-id acme --status active
```

## Managing Users

### List Users in an Organization

```bash
ai-aas-cli user list --org-id acme
```

### Create a User

```bash
# Create user (invite mode - sends email)
ai-aas-cli user create --org-id acme --email alice@example.com --name "Alice Smith"

# Create user with --upsert (idempotent, safe for scripts)
ai-aas-cli user create --org-id acme --email alice@example.com --name "Alice Smith" --upsert
```

**Tip:** Use `--upsert` in automation scripts to avoid 409 errors if the user already exists. See the CLI README for more details on idempotent operations.

## Managing API Keys

### List API Keys

```bash
ai-aas-cli apikey list --org-id acme
```

### Create an API Key

```bash
# Create API key for a user
ai-aas-cli apikey create --org-id acme --user-id u_123

# Create with expiration (90 days)
ai-aas-cli apikey create --org-id acme --user-id u_123 --expires-in-days 90
```

**Important**: The API key secret is shown only once at creation. Save it immediately!

### Delete an API Key

```bash
ai-aas-cli apikey delete --org-id acme --api-key-id ak_123 --confirm
```

## Complete Workflow

Here's a typical workflow for setting up a new team:

```bash
# 1. Create organization
ai-aas-cli org create --name "ACME Corp" --slug acme

# 2. Add team members
ai-aas-cli user create --org-id acme --email alice@example.com --name "Alice"
ai-aas-cli user create --org-id acme --email bob@example.com --name "Bob"

# 3. Create API keys for each user
ai-aas-cli apikey create --org-id acme --user-id <alice-user-id>
ai-aas-cli apikey create --org-id acme --user-id <bob-user-id>

# 4. (Optional) Enable specific models for the organization
ai-aas-cli model library enable gpt-oss-20b --org-id acme
```

## CLI Command Reference

| Command | Description |
|---------|-------------|
| `ai-aas-cli --init` | Run setup wizard |
| `ai-aas-cli --init-status` | Show configuration status |
| `ai-aas-cli config test` | Test API connectivity |
| `ai-aas-cli org list` | List organizations |
| `ai-aas-cli org create` | Create organization |
| `ai-aas-cli user list --org-id <org>` | List users |
| `ai-aas-cli user create` | Create user |
| `ai-aas-cli apikey list --org-id <org>` | List API keys |
| `ai-aas-cli apikey create` | Create API key |
| `ai-aas-cli usage query --org-id <org>` | Query usage data |

## Environment Variables

| Variable | Description |
|----------|-------------|
| `AI_AAS_API_KEY` | API key for authentication |
| `AI_AAS_API_ENDPOINT` | Admin API endpoint URL |
| `AI_AAS_ENVIRONMENT` | Target environment |
| `AI_AAS_PROFILE` | Active profile name |

## Getting Help

```bash
ai-aas-cli --help                    # General help
ai-aas-cli org --help                # Organization commands
ai-aas-cli apikey create --help      # Specific command help
```
