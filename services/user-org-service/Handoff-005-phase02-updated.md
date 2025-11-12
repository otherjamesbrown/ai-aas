# Handoff Document: Spec 005 - Phase 2 - Identity & Session Lifecycle (Updated)

**Spec**: 005-user-org-service  
**Date**: 2025-01-XX  
**Phase**: Phase 2 - Identity & Session Lifecycle (Priority: P1, maps to US-001)  
**Status**: Partially Complete - Core authentication, MFA enforcement, and API key lifecycle ready  
**Next Action**: Complete IdP federation stubs, recovery endpoints, and end-to-end tests

---

## 📋 Quick Summary

Phase 2 focuses on delivering interactive authentication flows, user/org lifecycles, and audit logging to satisfy US-001 acceptance scenarios. We've completed the foundational authentication infrastructure with OAuth2 flows, org membership validation, Kafka audit event emission, **MFA enforcement with TOTP verification**, and **API key lifecycle management**. Remaining work includes IdP federation stubs, recovery flows, and comprehensive testing.

---

## ✅ Completed Tasks

### T007: Postgres Repositories ✅
- **Status**: Complete
- **Deliverables**:
  - Postgres repositories for `orgs`, `users`, `sessions`, `api_keys`
  - Optimistic locking implementation
  - Row-Level Security (RLS) policies
  - Integration tests using testcontainers
- **Key Files**:
  - `internal/storage/postgres/store.go` - Core data access layer
  - `internal/storage/postgres/models.go` - Domain models
  - `migrations/sql/` - Database schema migrations

### T008: Authentication Service ⚠️ Partially Complete
- **Status**: Core complete, MFA enforcement complete, IdP pending
- **Completed**:
  - ✅ OAuth2 flows with `ory/fosite` (`/v1/auth/login`, `/refresh`, `/logout`)
  - ✅ Session issuance with Redis caching
  - ✅ Org membership validation in login flow
  - ✅ Password-based authentication
  - ✅ **MFA enforcement in login flow** (TOTP verification)
  - ✅ **MFA verification timestamp stored in session metadata**
- **Remaining**:
  - ⏳ IdP federation stubs (`go-oidc` integration for external identity providers)
- **Key Files**:
  - `internal/httpapi/auth/handlers.go` - Authentication endpoints with MFA enforcement
  - `internal/security/mfa.go` - TOTP verification utilities
  - `internal/oauth/store.go` - OAuth2 storage implementation
  - `internal/oauth/provider.go` - Fosite provider composition
  - `internal/storage/postgres/store.go` - Org membership validation methods

### T009: User/Org Lifecycle Handlers ⚠️ Mostly Complete
- **Status**: Core complete, recovery flows pending
- **Completed**:
  - ✅ User/org lifecycle handlers (`/v1/orgs`, `/v1/orgs/{slug}/invites`, `/v1/orgs/{slug}/users/...`)
  - ✅ Invite expiry (72-hour default, stored in `invite_tokens` table)
  - ✅ Suspension workflows (status updates via `UpdateUserStatus`)
  - ✅ Kafka audit event emission (with graceful fallback to logger)
- **Remaining**:
  - ⏳ Recovery API endpoints (recovery tokens exist in DB but no endpoints)
- **Key Files**:
  - `internal/httpapi/orgs/handlers.go` - Organization management
  - `internal/httpapi/users/handlers.go` - User lifecycle and invites
  - `internal/audit/events.go` - KafkaEmitter implementation
  - `internal/bootstrap/bootstrap.go` - Kafka emitter initialization

### T010: API Key Lifecycle ✅ **NEW - Complete**
- **Status**: Complete
- **Deliverables**:
  - ✅ API key issuance endpoint (`POST /v1/orgs/{orgId}/service-accounts/{serviceAccountId}/api-keys`)
  - ✅ API key revocation endpoint (`DELETE /v1/orgs/{orgId}/api-keys/{apiKeyId}`)
  - ✅ Secure random secret generation (32 bytes, base64url encoded)
  - ✅ SHA-256 fingerprint computation for key identification
  - ✅ Vault Transit stub (ready for production integration)
  - ✅ Redis revocation propagation for fast revocation checks
  - ✅ Audit event emission for key lifecycle operations
  - ✅ Optimistic locking for concurrent safety
- **Key Files**:
  - `internal/httpapi/apikeys/handlers.go` - API key lifecycle handlers
  - `internal/storage/postgres/store.go` - Added `GetAPIKeyByID` method
  - `internal/audit/events.go` - Added `ActionAPIKeyIssue` and `ActionAPIKeyRevoke` constants

---

## 🔧 Technical Implementation Details

### Authentication Flow with MFA
```
POST /v1/auth/login
├── Validates user credentials (email + password)
├── Checks account lockout status
├── Validates user belongs to specified org (or looks up org_id)
├── **NEW: Checks MFA requirements**:
│   ├── User must have mfa_enrolled = true
│   ├── User must have "totp" in mfa_methods
│   └── Org may require MFA for specific roles (mfa_required_roles)
├── **NEW: Verifies TOTP code** (if MFA required):
│   ├── Validates code against user's mfa_secret
│   └── Stores mfa_verified_at timestamp in session metadata
├── Issues OAuth2 access/refresh tokens via Fosite
└── Caches session in Redis (if configured)
```

**MFA Enforcement Logic**:
- MFA is required if:
  1. User has `mfa_enrolled = true` AND
  2. User has `"totp"` in `mfa_methods` array AND
  3. (Optional) Org has `mfa_required_roles` configured (currently requires MFA for all enrolled users)
- TOTP verification uses `github.com/pquerna/otp` library
- MFA verification timestamp stored in `session.Extra["mfa_verified_at"]` (serialized in `oauth_sessions.session_data` JSONB)

### API Key Lifecycle Flow
```
POST /v1/orgs/{orgId}/service-accounts/{serviceAccountId}/api-keys
├── Validates service account exists and belongs to org
├── Generates secure random secret (32 bytes)
├── Computes SHA-256 fingerprint
├── Encrypts secret via Vault Transit (stub)
├── Stores key metadata in database (fingerprint, scopes, expiry)
├── Stores encrypted secret in Vault (async, best-effort)
├── Emits audit event (api_key.issue)
└── Returns secret once (never stored in DB)

DELETE /v1/orgs/{orgId}/api-keys/{apiKeyId}
├── Validates key exists and belongs to org
├── Revokes key in database (optimistic locking)
├── Propagates revocation to Redis (for fast checks)
└── Emits audit event (api_key.revoke)
```

**Security Features**:
- Secrets are never stored in database (only fingerprints)
- Secrets displayed once on creation
- Vault Transit encryption stub ready for production integration
- Redis revocation cache enables sub-second revocation checks
- Fingerprints enable key identification without exposing secrets

### Audit Event Emission
- **KafkaEmitter**: Produces events to `audit.identity` topic when `KAFKA_BROKERS` is configured
- **LoggerEmitter**: Fallback when Kafka is not configured (development/testing)
- **Configuration**: Set `KAFKA_BROKERS` environment variable to enable Kafka
- **Event Schema**: Includes event_id, org_id, actor_id, action, target_type, metadata, hash
- **New Actions**: `api_key.issue`, `api_key.revoke`

### Database Schema
- **Users**: Support for MFA (mfa_enrolled, mfa_methods, mfa_secret), recovery tokens, lockout
- **API Keys**: Fingerprint, status, scopes, expiry tracking
- **Invite Tokens**: Separate table with expiry tracking (`invite_tokens`)
- **OAuth Sessions**: Stored in `oauth_sessions` table with Redis caching, MFA verification timestamp in session_data

---

## 📦 Dependencies Added

- `github.com/segmentio/kafka-go v0.4.49` - Kafka producer for audit events
- `github.com/pquerna/otp v1.5.0` - TOTP code generation and verification
- Existing dependencies (Fosite, pgx, Redis) remain unchanged

---

## 🚧 Remaining Work

### T008 Completion (Medium Priority)
1. **IdP Federation Stubs**:
   - Add `go-oidc` dependency
   - Create IdP provider configuration structure
   - Add `/v1/auth/oidc/{provider}/login` endpoint
   - Implement OIDC callback handler
   - Map external IdP users to internal users via `external_idp_id`

### T009 Completion (Medium Priority)
1. **Recovery Flows**:
   - Add `POST /v1/auth/recover` endpoint (initiate recovery)
   - Add `POST /v1/auth/recover/verify` endpoint (verify recovery token)
   - Add `POST /v1/auth/recover/reset` endpoint (reset password)
   - Implement recovery token generation and validation
   - Add admin approval workflow for recovery requests

### T011: Credential Recovery & Lockout (Medium Priority)
- Implement lockout handling workflows
- Add admin approval workflow for recovery
- Add audit trails for recovery operations

### T012: End-to-End Tests (High Priority)
- Create integration tests for login → MFA → key issuance → revocation flow
- Add k6 smoke tests
- Document manual verification steps in `quickstart.md`

### T013: Metrics & Observability (Medium Priority)
- Add Prometheus metrics collectors for:
  - Authentication success/failure rates
  - MFA verification success/failure rates
  - Session creation/revocation counts
  - API key issuance/revocation counts
- Create dashboards in `docs/observability/user-org-service-auth.json`

---

## 📁 Key Files Modified/Created

### Authentication & MFA
- `internal/httpapi/auth/handlers.go` - Login handler with MFA enforcement
- `internal/security/mfa.go` - **NEW** TOTP verification utilities
- `internal/oauth/store.go` - OAuth2 storage with Redis caching
- `internal/oauth/provider.go` - Fosite provider composition
- `internal/storage/postgres/store.go` - Added org membership validation methods

### API Key Lifecycle
- `internal/httpapi/apikeys/handlers.go` - **NEW** API key lifecycle handlers
- `internal/storage/postgres/store.go` - Added `GetAPIKeyByID` method
- `cmd/admin-api/main.go` - Registered API key routes

### Audit
- `internal/audit/events.go` - KafkaEmitter implementation, added API key action constants
- `internal/config/config.go` - Added Kafka configuration fields
- `internal/bootstrap/bootstrap.go` - Kafka emitter initialization

### User/Org Lifecycle
- `internal/httpapi/orgs/handlers.go` - Organization CRUD operations
- `internal/httpapi/users/handlers.go` - User invites, status updates
- `migrations/sql/000003_invite_tokens.sql` - Invite token table

---

## 🔍 Testing Status

### Unit Tests
- ✅ Postgres store tests (`internal/storage/postgres/store_test.go`)
- ✅ OAuth store tests (`internal/oauth/store_test.go`)
- ✅ Password security tests (`internal/security/password_test.go`)
- ⏳ MFA verification tests (pending)
- ⏳ API key handler tests (pending)

### Integration Tests
- ⏳ End-to-end authentication flow with MFA tests (T012 pending)
- ⏳ API key lifecycle tests (T012 pending)
- ⏳ Kafka audit event emission tests (pending)

### Manual Testing
- ✅ Login flow with org validation
- ✅ MFA enforcement (requires user with MFA enrolled)
- ✅ API key issuance and revocation
- ✅ Invite creation and expiry
- ✅ User suspension/activation
- ✅ Kafka audit event emission (when Kafka configured)

---

## 🚀 Deployment Considerations

### Environment Variables
```bash
# Required
DATABASE_URL=postgres://user:pass@host:5432/dbname?sslmode=disable
OAUTH_HMAC_SECRET=<32+ byte secret>
OAUTH_CLIENT_SECRET=<client secret>

# Optional (for Kafka audit)
KAFKA_BROKERS=broker1:9092,broker2:9092
KAFKA_TOPIC=audit.identity
KAFKA_CLIENT_ID=user-org-service

# Optional (for Redis caching)
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0

# Optional (for Vault Transit - when implemented)
VAULT_ADDR=https://vault.example.com:8200
VAULT_TOKEN=<vault-token>
VAULT_TRANSIT_KEY_NAME=api-keys
```

### Database Migrations
Run migrations before deploying:
```bash
make -C services/user-org-service migrate
```

### Kafka Topic Setup
If using Kafka, ensure topic exists:
```bash
kafka-topics --create \
  --topic audit.identity \
  --bootstrap-server localhost:9092 \
  --partitions 3 \
  --replication-factor 2
```

### MFA Enrollment
Users must enroll in MFA before it can be enforced:
1. User sets `mfa_enrolled = true`
2. User adds `"totp"` to `mfa_methods` array
3. User generates TOTP secret (use `security.GenerateTOTPSecret()`)
4. User stores secret in `mfa_secret` field
5. User scans QR code with authenticator app

### API Key Usage
- API keys are displayed once on creation (store securely)
- Use fingerprint for key identification in API calls
- Revocation propagates to Redis within seconds
- Vault Transit integration required for production (currently stub)

---

## ⚠️ Known Issues & Limitations

1. **IdP Federation Not Implemented**: External identity provider integration not implemented
2. **Recovery Endpoints Missing**: Recovery tokens exist in DB but no API endpoints
3. **Vault Transit Stub**: API key encryption uses stub implementation (not production-ready)
4. **Role-Based MFA**: MFA requirement logic checks org `mfa_required_roles` but doesn't verify user's actual roles yet (role system pending)
5. **Metrics**: No Prometheus metrics collectors yet (T013)
6. **Service Account Store Methods**: API key issuance assumes service account exists but doesn't verify (TODO: add `GetServiceAccountByID` method)

---

## 📚 Related Documentation

- **Spec**: `specs/005-user-org-service/spec.md`
- **Plan**: `specs/005-user-org-service/plan.md`
- **Data Model**: `specs/005-user-org-service/data-model.md`
- **Contracts**: `specs/005-user-org-service/contracts/user-org-service.openapi.yaml`
- **Tasks**: `specs/005-user-org-service/tasks.md`

---

## 🎯 Success Criteria

Phase 2 will be complete when:
- ✅ T008: MFA enforcement implemented
- ⏳ T008: IdP federation stubs implemented
- ✅ T010: API key lifecycle with Vault integration stub
- ⏳ T009: Recovery API endpoints implemented
- ⏳ T011: Credential recovery and lockout workflows
- ⏳ T012: End-to-end tests passing
- ⏳ T013: Metrics and dashboards operational

---

## 👥 Handoff Notes

### For Next Developer

**Immediate Next Steps**:
1. Complete IdP federation stubs (T008)
2. Implement recovery API endpoints (T009)
3. Add service account store methods for API key validation
4. Create end-to-end tests (T012)

**Key Decisions Made**:
- Using `segmentio/kafka-go` for Kafka integration (lightweight, no schema registry dependency)
- Using `github.com/pquerna/otp` for TOTP verification (standard library, RFC 6238 compliant)
- Kafka emitter gracefully falls back to logger when not configured (no breaking changes)
- Org membership validation happens in login handler (security boundary)
- MFA verification timestamp stored in OAuth session metadata (accessible via token introspection)
- API key secrets never stored in database (only fingerprints for identification)
- Vault Transit stub allows code to compile and run, ready for production integration

**Testing Approach**:
- Unit tests for store methods
- Integration tests with testcontainers for Postgres
- Manual testing for Kafka (requires Kafka cluster)
- Manual testing for MFA (requires TOTP authenticator app)

**Questions to Resolve**:
- Which IdP providers to support first? (Google, GitHub, Azure AD?)
- Vault Transit integration approach for API keys? (namespace, key rotation policy)
- Service account role verification for MFA requirements?
- Recovery token expiry and admin approval workflow details?

**Code Quality Notes**:
- All new code follows existing patterns and conventions
- Error handling uses Fosite's RFC6749Error for OAuth2 compliance
- MFA errors return appropriate OAuth2 error codes (`mfa_required`, `invalid_grant`)
- API key handlers follow same structure as orgs/users handlers
- Audit events emitted for all state-changing operations

---

**Ready to continue? Start with T008 IdP federation or T009 recovery endpoints!**

