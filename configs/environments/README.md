# Environment Profiles

Environment profiles provide centralized configuration management across different deployment environments (local-dev, remote-dev, production). Each profile defines component locations, ports, connection strings, and environment-specific settings.

## Overview

Environment profiles eliminate manual configuration edits by providing:
- **Centralized configuration**: All environment-specific settings in one place
- **Component tracking**: Clear documentation of where components run and their configurations
- **Easy switching**: Change environments without editing service configs
- **Validation**: Catch configuration errors before they cause runtime issues
- **Secret management**: Secure references to credentials without hardcoding

## Profile Structure

Profiles are YAML files stored in `configs/environments/` following this structure:

```yaml
api_version: v1
kind: EnvironmentProfile
metadata:
  name: local-dev
  description: Local development environment
  extends: base  # optional: inherit from base profile

components:
  postgres:
    host: localhost
    port: 5432
    database: ai_aas
    username: postgres
    password_env: POSTGRES_PASSWORD
    connection_string_template: "postgres://{username}:{password}@{host}:{port}/{database}?sslmode={sslmode}"
    sslmode: disable
    
environment_variables:
  - name: ENVIRONMENT
    value: local-dev
  - name: LOG_LEVEL
    value: debug  # debug, info, warn, error (default: info)
  - name: DATABASE_URL
    component: postgres
    from_template: connection_string_template

secrets:
  - name: POSTGRES_PASSWORD
    source: env_file
    file: .env.local
```

## Available Profiles

### base.yaml
Base profile with common component definitions and templates. Other profiles extend this for shared defaults.

### local-dev.yaml
Local development environment:
- Components run on `localhost`
- Default development ports (5432, 6379, etc.)
- SSL disabled for local connections
- Secrets from `.env.local` file
- `LOG_LEVEL=debug` for verbose logging during development

### remote-dev.yaml
Remote Linode development workspace:
- Components run on private network endpoints
- Production-like networking configuration
- Secrets from `.env.linode` file
- Integration with Linode infrastructure
- `LOG_LEVEL=info` (default, can be overridden for debugging)

### production.yaml
Production environment:
- Production service endpoints
- SSL required for all connections
- Secrets from Vault or production secrets store
- Strict validation and security requirements
- `LOG_LEVEL=info` (default, should not be set to debug in production)

## Usage

### Activate Environment

```bash
# Activate local development environment
make env-activate ENVIRONMENT=local-dev

# Activate remote development
make env-activate ENVIRONMENT=remote-dev

# Activate production (requires credentials)
make env-activate ENVIRONMENT=production
```

### View Configuration

```bash
# Show current environment configuration
make env-show

# Show configuration for specific component
make env-show COMPONENT=user_org_service
```

### Validate Configuration

```bash
# Validate active profile
make env-validate

# Checks:
# - YAML syntax validity
# - Schema compliance
# - Component dependencies
# - Secret availability
# - Port conflicts
```

### Compare Environments

```bash
# Compare two environment configurations
make env-diff ENV1=local-dev ENV2=remote-dev
```

### Export Configuration

```bash
# Export environment variables (shell format)
make env-export FORMAT=env > .env.export

# Export as YAML
make env-export FORMAT=yaml > config.yaml

# Export as JSON
make env-export FORMAT=json > config.json
```

## Component Registry

Components are registered in `configs/components.yaml` which documents:
- Component names and descriptions
- Required ports and protocols
- Dependencies on other components
- Required environment variables

When adding a new component:
1. Register it in `configs/components.yaml`
2. Add component definitions to environment profiles
3. Update service templates to reference the component

See `docs/platform/component-registry.md` for detailed component documentation.

## Profile Inheritance

Profiles can extend a base profile using the `extends` field:

```yaml
metadata:
  extends: base
```

Extended profiles inherit:
- Component definitions
- Environment variable templates
- Connection string templates

Profile-specific values override inherited values.

## Secret Management

Secrets are never hardcoded in profiles. Instead, profiles reference secret sources:

```yaml
secrets:
  - name: POSTGRES_PASSWORD
    source: env_file      # From .env.local or .env.linode
    file: .env.local
    
  - name: VAULT_TOKEN
    source: vault         # From HashiCorp Vault
    path: production/database/token
    
  - name: GITHUB_TOKEN
    source: github        # From GitHub secrets
    environment: production
```

During profile activation:
1. Profile manager reads secret references
2. Fetches secrets from specified sources
3. Merges secrets into generated `.env.*` files
4. Ensures `.gitignore` protection

## Integration with Services

Services use environment profiles via:

1. **Template references**: Service config templates (`configs/dev/<service>.env.tpl`) reference profile variables
2. **Automatic loading**: `scripts/dev/common.sh` sources active profile and exports variables
3. **Generated files**: Profile activation generates `.env.*` files that services load

Example service template:
```bash
# configs/dev/user-org-service.env.tpl
SERVICE_NAME=user-org-service
HTTP_PORT=${USER_ORG_SERVICE_PORT}
DATABASE_URL=${DATABASE_URL}
REDIS_ADDR=${REDIS_ADDR}
```

## Adding New Environments

To add a new environment profile:

1. Create `configs/environments/<name>.yaml`
2. Define components, environment variables, and secrets
3. Extend `base.yaml` if appropriate
4. Update `manage-env.sh` to include new environment in validation
5. Test activation and validation

Example:
```bash
# Create new profile
cat > configs/environments/staging.yaml <<EOF
api_version: v1
kind: EnvironmentProfile
metadata:
  name: staging
  description: Staging environment
  extends: base
components:
  # Define staging-specific component configs
EOF

# Activate and validate
make env-activate ENVIRONMENT=staging
make env-validate
```

## Validation Rules

Profile validation checks:

1. **YAML syntax**: Valid YAML structure
2. **Schema compliance**: Required fields present, types correct
3. **Component dependencies**: All referenced components exist in registry
4. **Secret availability**: Referenced secrets accessible from source
5. **Port conflicts**: No duplicate ports within same environment
6. **Connection strings**: Valid templates with required placeholders
7. **Profile inheritance**: Extended profiles exist and are valid

## Troubleshooting

### Profile not found
```bash
# List available profiles
ls configs/environments/*.yaml

# Check profile name spelling
make env-show
```

### Validation errors
```bash
# Run detailed validation
make env-validate

# Check component registry
cat configs/components.yaml

# Verify secret sources
make secrets-sync
```

### Wrong environment active
```bash
# Check current environment
make env-show

# Switch to correct environment
make env-activate ENVIRONMENT=<correct-env>
```

### Configuration not applied
```bash
# Regenerate .env file from profile
make env-generate-env-file

# Verify environment variables loaded
env | grep -E "DATABASE_URL|REDIS_ADDR"
```

## Environment Variables

### LOG_LEVEL

Controls logging verbosity for all Go services using the `shared/go/logging` package:
- `debug`: Verbose debugging information (use for local development and troubleshooting)
- `info`: General informational messages (default, recommended for production)
- `warn`: Warning messages only
- `error`: Error messages only

**Default values by environment**:
- `base.yaml`: `info` (default for all environments)
- `local-dev.yaml`: `debug` (overrides base for local development)
- `remote-dev.yaml`: `info` (inherits from base, can be overridden)
- `production.yaml`: `info` (inherits from base, should not be changed to debug)

To change log level for a specific environment, override `LOG_LEVEL` in the environment profile:
```yaml
environment_variables:
  - name: LOG_LEVEL
    value: debug  # Override default for this environment
```

## Best Practices

1. **Always validate** before activating a profile
2. **Use profile inheritance** to reduce duplication
3. **Reference components** rather than hardcoding values
4. **Never hardcode secrets** - always use secret references
5. **Update component registry** when adding new components
6. **Test profile switches** in a clean environment before deploying
7. **Document environment-specific quirks** in profile descriptions
8. **Set appropriate LOG_LEVEL**: Use `debug` for local development, `info` for production

## Related Documentation

- Component Registry: `docs/platform/component-registry.md`
- Secrets Management: `docs/runbooks/secrets-management.md`
- Service Configuration: `docs/runbooks/service-dev-connect.md`
- Environment Contracts: `specs/002-local-dev-environment/contracts/dev-environment-contracts.md`

