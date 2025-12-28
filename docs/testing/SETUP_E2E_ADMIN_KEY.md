# How to Create an E2E Admin API Key

This guide explains how to create an admin API key for E2E testing on any environment (development, staging, production).

## Prerequisites

1. Access to the target Kubernetes cluster
2. Database credentials for the environment
3. kubectl configured with appropriate KUBECONFIG

## Quick Setup Script

We provide scripts to automate this process:

```bash
# Generate the SQL and API key information
python3 scripts/create-e2e-admin-key.py > /tmp/e2e-admin-setup.sql

# Review the generated SQL (check organization, user, and API key details)
cat /tmp/e2e-admin-setup.sql

# Apply to database (replace with your database connection string)
export DB_URL="postgres://user:pass@host:port/dbname?sslmode=require"
kubectl run psql-client --rm -i --restart=Never --image=postgres:15-alpine -- \
  psql "$DB_URL" -f - < /tmp/e2e-admin-setup.sql

# Extract the API key secret from the output
# Save it securely - it will be shown in the output
```

## Manual Setup (Step-by-Step)

If you prefer to understand each step or need to customize the setup:

### Step 1: Generate API Key and Hashes

The API key system uses:
- **Secret**: A random 32-byte value, base64url-encoded
- **Fingerprint**: SHA-256 hash of the secret, base64url-encoded
- **Password**: Argon2id hash for user login

Generate these values:

```bash
# Generate API key secret and fingerprint
python3 << 'EOF'
import secrets, hashlib, base64

secret_bytes = secrets.token_bytes(32)
secret = base64.urlsafe_b64encode(secret_bytes).decode('utf-8').rstrip('=')
fingerprint_hash = hashlib.sha256(secret.encode('utf-8')).digest()
fingerprint = base64.urlsafe_b64encode(fingerprint_hash).decode('utf-8').rstrip('=')

print(f"API Key Secret: {secret}")
print(f"Fingerprint: {fingerprint}")
EOF

# Generate Argon2id password hash
go run scripts/gen-argon2-hash.go
# Output: $argon2id$v=19$m=65536,t=1,p=4$...$...
```

### Step 2: Create Database Records

Create SQL to insert the organization, user, and API key:

```sql
BEGIN;

-- Create Organization
INSERT INTO orgs (org_id, slug, name, status, declarative_mode, mfa_required_roles, metadata, created_at, updated_at)
VALUES (
    '<org-uuid>',
    'e2e-admin-org',
    'E2E Test Admin Organization',
    'active',
    'disabled',
    '[]'::jsonb,
    '{}'::jsonb,
    NOW(),
    NOW()
)
ON CONFLICT (slug) DO UPDATE SET status = 'active';

-- Create User
INSERT INTO users (user_id, org_id, email, display_name, password_hash, status, mfa_enrolled, mfa_methods, recovery_tokens, metadata, created_at, updated_at)
VALUES (
    '<user-uuid>',
    '<org-uuid>',
    'e2e-admin@ai-aas.dev',
    'E2E Admin User',
    '<argon2id-hash>',
    'active',
    false,
    '[]'::jsonb,
    '[]'::jsonb,
    '{}'::jsonb,
    NOW(),
    NOW()
)
ON CONFLICT (org_id, email) DO UPDATE SET status = 'active', password_hash = '<argon2id-hash>';

-- Create API Key
INSERT INTO api_keys (api_key_id, org_id, principal_type, principal_id, fingerprint, status, scopes, issued_at, expires_at, annotations, created_at, updated_at)
VALUES (
    '<apikey-uuid>',
    '<org-uuid>',
    'user',
    '<user-uuid>',
    '<fingerprint>',
    'active',
    '["*"]'::jsonb,
    NOW(),
    NOW() + INTERVAL '365 days',
    '{"purpose": "e2e-tests", "created_by": "script"}'::jsonb,
    NOW(),
    NOW()
)
ON CONFLICT (org_id, fingerprint) DO UPDATE SET status = 'active', expires_at = NOW() + INTERVAL '365 days';

COMMIT;
```

### Step 3: Execute SQL

```bash
# Get database URL from Kubernetes secret
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
export DB_URL=$(kubectl get secret -n user-org-service user-org-service-db-secret \
  -o jsonpath='{.data.database-url}' | base64 -d)

# Execute SQL via kubectl run (one-time pod)
cat your-setup.sql | kubectl run psql-client --rm -i --restart=Never \
  --image=postgres:15-alpine -- psql "$DB_URL" -f -
```

### Step 4: Verify API Key

Test the API key validation:

```bash
# Port-forward to user-org-service
kubectl port-forward -n user-org-service svc/user-org-service-development-user-org-service 8081:8081 &

# Validate the API key
curl -X POST http://localhost:8081/v1/auth/validate-api-key \
  -H "Content-Type: application/json" \
  -d '{"apiKeySecret":"<your-api-key-secret>"}'
```

Expected response with `"valid": true`.

### Step 5: Document the Key

Save the following information securely:

- Organization ID and slug
- User ID and email
- API Key ID and secret
- Password (if needed for OAuth login)

## Environment Variables for Tests

Add to your test environment or CI/CD:

```bash
export E2E_API_KEY="<api-key-secret>"
export E2E_ORG_ID="<org-uuid>"
export E2E_USER_ID="<user-uuid>"
export E2E_USER_EMAIL="e2e-admin@ai-aas.dev"
export E2E_USER_PASSWORD="e2e-admin-password"
```

## Security Considerations

1. **Scope**: The API key has `["*"]` scope, granting full admin access
2. **Environment**: Only use for non-production environments or isolated test orgs
3. **Rotation**: Rotate the key periodically (set expiration to 1 year)
4. **Storage**: Never commit API keys to version control
5. **Secrets Management**: Use Kubernetes secrets or secret management tools in CI/CD

## Troubleshooting

### API Key Validation Fails

- Verify the fingerprint matches: `echo -n "<secret>" | sha256sum`
- Check the API key status in database: `SELECT * FROM api_keys WHERE fingerprint = '<fingerprint>'`
- Ensure the key hasn't expired

### Password Login Fails

- The service expects Argon2id password hashes
- Common error: `parse argon hash: unexpected format` means bcrypt was used instead
- Regenerate with: `go run scripts/gen-argon2-hash.go`

### Database Connection Issues

- Verify KUBECONFIG is set correctly
- Check the database URL secret exists
- Test connection: `psql "$DB_URL" -c "SELECT 1"`

## See Also

- [E2E Admin API Key Documentation](./E2E_ADMIN_API_KEY.md)
- [Admin CLI Bootstrap Guide](../services/admin-cli/README.md)
- [User-Org Service API Reference](../../specs/005-user-org-service/spec.md)
