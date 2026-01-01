# Master Admin Account Setup

This document describes the master admin account for the AI-AAS platform development cluster.

## Overview

The master admin account is the primary administrative account with full platform access. It is used for:
- Platform bootstrapping and initialization
- System-level administration tasks
- Creating and managing organizations
- Emergency access and break-glass procedures
- Admin CLI operations

## Account Details

**IMPORTANT: These credentials are highly sensitive. Never commit them to public repositories or share them publicly.**

### Organization
- **Name**: Master Admin Organization
- **Slug**: `master-admin-org`
- **Org ID**: `eda1ca3a-dbfa-4a48-a591-1b7e7f489e1a`

### User
- **Email**: `master-admin@ai-aas.dev`
- **Display Name**: Master Admin User
- **User ID**: `40a4652c-76aa-4a3c-b42a-34d6ccc72ece`
- **Password**: `master-admin-password` (⚠️ **PLACEHOLDER - MUST BE CHANGED**)
- **Password Hash**: `$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi` (bcrypt, cost 10)

### API Key
- **API Key ID**: `b3a115c6-e4b4-4ed9-823c-250ebed4e3ec`
- **API Key Secret**: `VXDzIauNfwRdmUDowO37plULPXbf1fUBr-69oqSEWEA`
- **Scopes**: `["*"]` (full admin access to all platform resources)
- **Status**: Active
- **Expires**: 2026-11-23 (365 days from creation)
- **Purpose**: Master admin operations
- **Fingerprint**: `zQ3mdSL2A5tyRSMGclO2VBsQ8xbwEwtBO8eCOrXy88Q` (SHA-256 hash)

## Credential Storage

The master admin credentials are stored in:

```
secrets/env/.env
```

This file is encrypted using `git-crypt` and is only accessible to authorized team members with the git-crypt key. All test credentials (master admin, E2E, staging) are consolidated in this single file.

## Usage

### Loading Credentials

```bash
# Load credentials into your shell (after git-crypt unlock)
export $(grep -v '^#' secrets/env/.env | xargs)

# For Go tests, use the testconfig loader (auto-loads from secrets/env/.env)
# See: shared/go/testconfig/loader.go
```

### Using with Admin CLI

The admin CLI is configured to use the master admin API key by default:

```bash
# Configuration file
~/.admin-cli/config.yaml

# List organizations
admin-cli org list

# Bootstrap a new organization
admin-cli bootstrap --email=admin@example.com --org-name="Example Org"
```

### Using with API Calls

```bash
# Example: List organizations via API
curl -H "X-API-Key: $MASTER_ADMIN_API_KEY" \
     https://user-org.dev.otherjamesbrown.com/v1/orgs

# Example: Create a new user
curl -X POST \
     -H "X-API-Key: $MASTER_ADMIN_API_KEY" \
     -H "Content-Type: application/json" \
     -d '{"email":"user@example.com","display_name":"John Doe"}' \
     https://user-org.dev.otherjamesbrown.com/v1/users
```

### Using in Tests

```bash
# Go tests
MASTER_ADMIN_API_KEY=$MASTER_ADMIN_API_KEY go test ./...

# JavaScript/TypeScript tests
MASTER_ADMIN_API_KEY=$MASTER_ADMIN_API_KEY npm test
```

## Creation Process

The master admin account was created on 2025-11-23 using the following process:

1. **Generated API Key**: Used `scripts/create-e2e-admin-key.py` to generate a cryptographically secure API key and SHA-256 fingerprint

2. **Inserted into Database**: Executed SQL to insert:
   - Organization record in `orgs` table
   - User record in `users` table with bcrypt password hash
   - API key record in `api_keys` table with fingerprint

3. **Stored Credentials**: Added credentials to `secrets/env/.env`

4. **Updated Admin CLI**: Updated `~/.admin-cli/config.yaml` with the new API key

5. **Documentation**: Created this documentation file

### Database Records

The following database queries can verify the master admin setup:

```sql
-- Verify organization
SELECT org_id, slug, name, status
FROM orgs
WHERE slug = 'master-admin-org';

-- Verify user
SELECT user_id, email, display_name, status
FROM users
WHERE email = 'master-admin@ai-aas.dev';

-- Verify API key
SELECT api_key_id, fingerprint, scopes, status, expires_at
FROM api_keys
WHERE principal_id = '40a4652c-76aa-4a3c-b42a-34d6ccc72ece';
```

## Password Management

### Changing the Master Admin Password

⚠️ **IMPORTANT**: The default password `master-admin-password` is a placeholder and **MUST** be changed for production use.

#### Option 1: Using the User-Org Service API

```bash
# Change password via API (requires current API key)
curl -X PUT \
     -H "X-API-Key: $MASTER_ADMIN_API_KEY" \
     -H "Content-Type: application/json" \
     -d '{"current_password":"master-admin-password","new_password":"YOUR_STRONG_PASSWORD"}' \
     https://user-org.dev.otherjamesbrown.com/v1/users/me/password
```

#### Option 2: Direct Database Update

If you need to reset the password directly:

1. **Generate a secure password**:
   ```bash
   # Generate a random 32-character password
   openssl rand -base64 32
   ```

2. **Generate bcrypt hash** (using Python):
   ```bash
   python3 -c "import bcrypt; print(bcrypt.hashpw(b'YOUR_PASSWORD_HERE', bcrypt.gensalt(10)).decode())"
   ```

3. **Update the database**:
   ```sql
   UPDATE users
   SET password_hash = '$2a$10$YOUR_BCRYPT_HASH_HERE',
       updated_at = NOW()
   WHERE user_id = '40a4652c-76aa-4a3c-b42a-34d6ccc72ece';
   ```

4. **Update the .env file**:
   ```bash
   # Edit secrets/env/.env
   MASTER_ADMIN_USER_PASSWORD=your_new_secure_password
   ```

#### Option 3: Using htpasswd (if available)

```bash
# Generate bcrypt hash with htpasswd
htpasswd -nbBC 10 admin YOUR_PASSWORD_HERE | cut -d: -f2
```

### Password Requirements

For production environments, the master admin password should:

- Be at least 16 characters long
- Include uppercase and lowercase letters
- Include numbers and special characters
- Not be based on dictionary words
- Be unique and not reused from other systems
- Be stored securely (use a password manager)

### Example Secure Password Generation

```bash
# Generate a cryptographically secure password
python3 << 'EOF'
import secrets
import string

# Generate a 24-character password with mixed case, digits, and symbols
alphabet = string.ascii_letters + string.digits + "!@#$%^&*()-_=+"
password = ''.join(secrets.choice(alphabet) for i in range(24))
print(f"Secure Password: {password}")
EOF
```

## Security Considerations

### Access Control
- The master admin API key has `["*"]` scope - **unlimited access to all platform resources**
- This key should only be used for legitimate administrative tasks
- Never share this key in logs, error messages, or debug output
- Rotate this key immediately if compromised

### Key Rotation
To rotate the master admin API key:

1. Generate a new API key using `scripts/create-e2e-admin-key.py`
2. Update the database record for the existing API key
3. Update `secrets/env/.env`
4. Update `~/.admin-cli/config.yaml`
5. Update any CI/CD pipelines or automation using the key
6. Revoke the old key after migration is complete

### Break-Glass Procedure
If the master admin key is lost or compromised:

1. Connect directly to the database using the database admin credentials
2. Generate a new API key and fingerprint
3. Update the `api_keys` table with a new record or update the existing one
4. Update all references to the old key
5. Document the incident in the security log

## Related Documentation

- [E2E Admin Setup](../e2e/SETUP_E2E_ADMIN_KEY.md) - Similar setup for E2E testing
- [Admin CLI README](../../services/admin-cli/README.md) - Admin CLI usage guide
- [User-Org Service API](../../specs/007-user-org-service/api.md) - API documentation
- [Security Best Practices](../best-practices/security.md) - Platform security guidelines

## Development Cluster Endpoints

The master admin account works with these development cluster endpoints:

- **API Router**: `https://api.dev.otherjamesbrown.com`
- **User-Org Service**: `https://user-org.dev.otherjamesbrown.com`
- **Analytics Service**: `https://analytics.dev.otherjamesbrown.com`
- **ArgoCD**: `https://argocd.dev.otherjamesbrown.com`
- **Grafana**: `https://grafana.dev.otherjamesbrown.com`

## Support

For questions or issues with the master admin account:

1. Check this documentation first
2. Review the admin CLI logs: `~/.admin-cli/audit.log`
3. Check the user-org-service logs in Kubernetes
4. Contact the platform team via the team Slack channel

## Changelog

- **2025-11-23**: Initial creation of master admin account
  - Created organization, user, and API key
  - Configured admin CLI
  - Created documentation
