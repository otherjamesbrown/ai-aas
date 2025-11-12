# Manual Verification Steps: User-Org-Service

This document provides step-by-step manual verification procedures for the complete authentication and API key lifecycle flow.

## Prerequisites

1. **Service Running**: User-org-service should be running locally or in development
   ```bash
   make run  # Starts on http://localhost:8081
   ```

2. **Database Migrated**: Ensure migrations are applied
   ```bash
   make migrate
   ```

3. **OAuth Client Credentials**: Set environment variables (or use defaults)
   ```bash
   export OAUTH_CLIENT_ID="test-client-id"
   export OAUTH_CLIENT_SECRET="test-client-secret"
   ```

## Complete Auth Flow Verification

### Step 1: Create Organization

```bash
curl -X POST http://localhost:8081/v1/orgs \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Test Organization",
    "slug": "test-org-manual"
  }'
```

**Expected Response**: `201 Created` with `orgId` in response body

**Verify**:
- Organization ID is a valid UUID
- Status is `active`
- Created timestamp is present

---

### Step 2: Invite User

```bash
ORG_ID="<org-id-from-step-1>"

curl -X POST http://localhost:8081/v1/orgs/$ORG_ID/invites \
  -H 'Content-Type: application/json' \
  -d '{
    "email": "test-user@example.com",
    "roles": ["member"]
  }'
```

**Expected Response**: `202 Accepted` with `inviteId` (user ID) in response body

**Verify**:
- User ID is a valid UUID
- Email matches request
- Status is `pending` or `invited`

**Note**: The invite creates a user with status "invited" and a temporary password. To complete the flow, you would need to:
- Get the temporary password (not currently returned in response)
- Or use a recovery flow to set a password
- Or use a test endpoint that creates active users

---

### Step 3: Activate User (Manual Database Update)

Since the invite flow doesn't return the temporary password, for manual testing you can:

**Option A**: Use the seed command to create an active user
```bash
# Create a user directly via seed command
make seed  # Follow prompts to create active user
```

**Option B**: Update user status via database (for testing only)
```sql
-- Connect to database
psql $DATABASE_URL

-- Update user status to active
UPDATE users SET status = 'active' WHERE email = 'test-user@example.com';
```

---

### Step 4: Login

```bash
curl -X POST http://localhost:8081/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{
    "email": "test-user@example.com",
    "password": "<password>",
    "client_id": "test-client-id",
    "client_secret": "test-client-secret",
    "scope": "openid profile email",
    "org_id": "<org-id>"
  }'
```

**Expected Response**: `200 OK` with OAuth2 tokens:
- `access_token`: Bearer token for API requests
- `refresh_token`: Token for refreshing access
- `expires_in`: Token expiration time
- `token_type`: "Bearer"

**Verify**:
- Access token is present and non-empty
- Token type is "Bearer"
- Expires in is a positive number

---

### Step 5: Enable MFA (Optional)

**Note**: MFA enrollment requires an authenticated user. This step would typically be done via a user profile endpoint or MFA enrollment endpoint.

For manual testing, you can verify MFA is enforced by:
1. Setting `mfa_enrolled: true` on the user
2. Attempting login without MFA code (should fail)
3. Attempting login with MFA code (should succeed)

---

### Step 6: Login with MFA

```bash
curl -X POST http://localhost:8081/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{
    "email": "test-user@example.com",
    "password": "<password>",
    "mfaCode": "<6-digit-totp-code>",
    "client_id": "test-client-id",
    "client_secret": "test-client-secret",
    "scope": "openid profile email",
    "org_id": "<org-id>"
  }'
```

**Expected Response**: `200 OK` with tokens (same as Step 4)

**Verify**:
- Login succeeds with valid MFA code
- Login fails with invalid MFA code
- Login fails without MFA code if MFA is required

---

### Step 7: Create Service Account

```bash
ACCESS_TOKEN="<access-token-from-step-4>"
ORG_ID="<org-id>"

curl -X POST http://localhost:8081/v1/orgs/$ORG_ID/service-accounts \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -d '{
    "name": "test-service-account",
    "description": "Manual test service account"
  }'
```

**Expected Response**: `201 Created` with:
- `serviceAccountId`: UUID of created service account
- `orgId`: Organization ID
- `name`: Service account name
- `status`: "active"
- `createdAt`: Timestamp

**Verify**:
- Service account ID is a valid UUID
- Status is "active"
- Created timestamp is present

---

### Step 8: Issue API Key

```bash
SERVICE_ACCOUNT_ID="<service-account-id-from-step-7>"

curl -X POST http://localhost:8081/v1/orgs/$ORG_ID/service-accounts/$SERVICE_ACCOUNT_ID/api-keys \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -d '{
    "scopes": ["inference:read", "inference:write"],
    "expiresInDays": 90
  }'
```

**Expected Response**: `200 OK` with:
- `apiKeyId`: UUID of the API key
- `secret`: **API key secret (shown only once!)**
- `fingerprint`: SHA-256 fingerprint
- `scopes`: Array of granted scopes
- `expiresAt`: Expiration timestamp

**Verify**:
- Secret is present and non-empty
- Secret is a long random string
- Fingerprint matches SHA-256 hash of secret
- Scopes match requested scopes
- Expires at is in the future

**Important**: Save the `secret` - it won't be shown again!

---

### Step 9: Validate API Key

```bash
API_KEY_SECRET="<secret-from-step-8>"

curl -X POST http://localhost:8081/v1/auth/validate-api-key \
  -H 'Content-Type: application/json' \
  -d '{
    "apiKeySecret": "'$API_KEY_SECRET'",
    "orgId": "'$ORG_ID'"
  }'
```

**Expected Response**: `200 OK` with:
- `valid`: `true`
- `apiKeyId`: UUID of the API key
- `organizationId`: Organization ID
- `principalId`: Service account ID
- `principalType`: "service_account"
- `scopes`: Array of scopes
- `status`: "active"

**Verify**:
- Valid is `true`
- Organization ID matches
- Principal type is "service_account"
- Scopes match issued scopes

---

### Step 10: Revoke API Key

```bash
API_KEY_ID="<api-key-id-from-step-8>"

curl -X DELETE http://localhost:8081/v1/orgs/$ORG_ID/api-keys/$API_KEY_ID \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

**Expected Response**: `200 OK` with:
- `apiKeyId`: UUID of revoked key
- `status`: "revoked"
- `revokedAt`: Timestamp

**Verify**:
- Status is "revoked"
- Revoked at timestamp is present

---

### Step 11: Verify Revoked Key is Rejected

```bash
# Try to validate the revoked key again
curl -X POST http://localhost:8081/v1/auth/validate-api-key \
  -H 'Content-Type: application/json' \
  -d '{
    "apiKeySecret": "'$API_KEY_SECRET'",
    "orgId": "'$ORG_ID'"
  }'
```

**Expected Response**: `200 OK` with:
- `valid`: `false`
- `message`: "API key is revoked" or similar

**Verify**:
- Valid is `false`
- Error message indicates revocation

---

## Quick Verification Checklist

- [ ] Organization creation works
- [ ] User invitation creates user with "invited" status
- [ ] User can be activated (via seed or manual DB update)
- [ ] Login returns OAuth2 tokens
- [ ] MFA can be enabled (if endpoint exists)
- [ ] Login with MFA works
- [ ] Service account creation requires authentication
- [ ] API key issuance returns secret
- [ ] API key validation works for valid keys
- [ ] API key validation rejects invalid keys
- [ ] API key revocation works
- [ ] Revoked keys are rejected during validation

---

## Troubleshooting

### Login Fails

- **User not active**: Check user status in database, update to "active" if needed
- **Wrong password**: Verify password matches what was set
- **OAuth client credentials**: Ensure `OAUTH_CLIENT_ID` and `OAUTH_CLIENT_SECRET` match config
- **User not in org**: Verify user belongs to the organization

### API Key Validation Fails

- **Key not found**: Verify the secret matches exactly (no extra spaces)
- **Key revoked**: Check if key was revoked
- **Key expired**: Check expiration timestamp
- **Org mismatch**: Verify org ID matches

### Service Account Creation Fails

- **Authentication required**: Ensure `Authorization: Bearer <token>` header is present
- **Invalid token**: Verify access token is valid and not expired
- **Org not found**: Verify organization ID is correct

---

## Next Steps

For automated testing, see:
- `make e2e-test` - Run end-to-end tests
- `make smoke-k6` - Run k6 smoke tests
- `docs/e2e-testing.md` - E2E testing guide

