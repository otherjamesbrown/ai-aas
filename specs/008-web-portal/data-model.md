# Data Model: Web Portal

**Branch**: `008-web-portal`  
**Date**: 2025-11-11  
**Spec**: `/specs/008-web-portal/spec.md`

## Entities

### OrganizationProfile
- **description**: Tenant-level metadata displayed and editable within the portal.
- **fields**:
  - `org_id`: UUID (immutable, required)
  - `name`: string (1–120 chars, trimmed)
  - `status`: enum (`active`, `suspended`, `trial`)
  - `billing_contact`: object (`name`, `email`, `phone`)
  - `address`: object (ISO country, region, postal code, street lines)
  - `policy_flags`: array of strings (e.g., `mfa_required`, `impose_budget_lock`)
  - `updated_at`: ISO-8601 timestamp
- **relationships**:
  - 1..* with **MemberAccount**
  - 1..1 with **BudgetPolicy**
  - 1..* with **AuditEvent**
- **rules**:
  - Updates require confirmation dialog; changes persisted via Organization service.
  - Edits can be rolled back within 24h by contacting support (tracked via audit).

### MemberAccount
- **description**: Represents a user belonging to an organization with assigned role and invite state.
- **fields**:
  - `member_id`: UUID
  - `identity_id`: UUID (maps to identity service)
  - `email`: RFC5322-compliant string
  - `role`: enum (`owner`, `admin`, `manager`, `analyst`, `custom`)
  - `invite_status`: enum (`pending`, `accepted`, `revoked`, `expired`)
  - `mfa_required`: boolean
  - `last_active_at`: ISO-8601 timestamp (nullable)
  - `scopes`: array of strings (API scopes for fine-grained permissions)
- **relationships**:
  - Belongs to **OrganizationProfile**
  - Generates **AuditEvent** entries on lifecycle changes
- **state transitions**:
  - `pending → accepted` upon invite acceptance; triggers welcome email.
  - `pending → revoked` when cancelled; no reactivation without new invite.
  - `accepted → revoked` when removed; requires confirmation modal and audit log.
- **rules**:
  - Email invites expire after configurable duration (default 7 days).
  - Role downgrade cannot remove last `owner`.

### RoleDefinition
- **description**: RBAC bundle that dictates navigation visibility and action permissions.
- **fields**:
  - `role_key`: string (kebab-case)
  - `display_name`: string
  - `description`: markdown/text
  - `permissions`: array of permission identifiers (e.g., `org.members.read`)
  - `feature_flags`: array linking to LaunchDarkly flag keys
- **relationships**:
  - Assigned to **MemberAccount**
  - Referenced by UI to determine page routing
- **rules**:
  - System roles (`owner`, `admin`, `manager`, `analyst`) immutable through UI.
  - Custom roles editable via API only; portal reflects read-only state.

### BudgetPolicy
- **description**: Configuration controlling spend limits, alerts, and enforcement behavior.
- **fields**:
  - `policy_id`: UUID
  - `monthly_limit_cents`: integer (>=0)
  - `alert_thresholds`: array of percentages (ascending)
  - `currency`: ISO 4217 code
  - `enforcement_mode`: enum (`monitor`, `warn`, `block`)
  - `alert_recipients`: array of emails
  - `effective_at`: ISO-8601 timestamp
- **relationships**:
  - Linked 1..1 with **OrganizationProfile**
  - Changes generate **AuditEvent** records
- **rules**:
  - UI enforces min/max thresholds from policy flags.
  - Enforcement changes require typed confirmation and MFA if enabled.

### ApiKeyCredential
- **description**: Represents an API key managed through the portal.
- **fields**:
  - `key_id`: UUID
  - `display_name`: string
  - `scopes`: array of permission identifiers
  - `status`: enum (`active`, `revoked`, `rotated`)
  - `created_at` / `rotated_at`: ISO-8601 timestamps
  - `fingerprint`: last 8 characters (stored, masked)
  - `expires_at`: ISO-8601 timestamp (nullable)
- **relationships**:
  - Belongs to **OrganizationProfile**
  - Lifecycle events captured as **AuditEvent**
- **rules**:
  - Full secret shown once at creation; subsequent downloads require rotation.
  - Revocation is idempotent; repeated revokes warn without duplicating audit entries.

### UsageSnapshot
- **description**: Aggregated usage metrics rendered in dashboards and exports.
- **fields**:
  - `window_start` / `window_end`: ISO-8601 timestamps
  - `model`: string (e.g., `gpt-4o`)
  - `operation`: enum (`chat`, `embeddings`, `fine-tune`)
  - `requests`: integer
  - `tokens`: integer
  - `cost_cents`: integer
  - `confidence`: enum (`estimated`, `finalized`)
- **relationships**:
  - Derived from Billing service datasets scoped to **OrganizationProfile**
  - Aggregated into **UsageReport**
- **rules**:
  - Filters by time/model must map to server-supported query params.
  - Empty datasets surface actionable guidance in UI.

### UsageReport
- **description**: UI-friendly structure combining charts, KPIs, and table data.
- **fields**:
  - `time_range`: enum (`last_24h`, `last_7d`, `last_30d`, `custom`)
  - `totals`: object (`requests`, `tokens`, `cost_cents`)
  - `breakdowns`: array of **UsageSnapshot**
  - `generated_at`: ISO-8601 timestamp
  - `source`: enum (`billing-api`, `cache`, `degraded`)
- **relationships**:
  - Backed by **UsageSnapshot** records
  - Exportable to CSV for support workflows
- **rules**:
  - `source=degraded` triggers degraded-state banner and retry/backoff.
  - Exports queue asynchronous job; UI shows status updates via notifications.

### AuditEvent
- **description**: Immutable log of privileged or destructive actions initiated from the portal.
- **fields**:
  - `event_id`: UUID
  - `actor_id`: UUID (member performing action)
  - `organization_id`: UUID
  - `action`: string (`member.invite`, `budget.update`, `apikey.revoke`, etc.)
  - `target`: string/JSON summarizing affected entity
  - `result`: enum (`success`, `failure`)
  - `timestamp`: ISO-8601
  - `metadata`: JSON object (IP, user agent, impersonation flag)
- **relationships**:
  - Produced via Audit service integrations
  - Referenced by support investigators and compliance exports
- **rules**:
  - Must be emitted within 2 seconds of action completion.
  - Portal displays toast linking to latest audit entry for transparency.

### SupportImpersonationSession
- **description**: Temporary read-only session enabling support engineers to view customer context.
- **fields**:
  - `session_id`: UUID
  - `support_user_id`: UUID
  - `organization_id`: UUID
  - `requested_at`: ISO-8601 timestamp
  - `expires_at`: ISO-8601 timestamp
  - `consent_token`: string (signed proof of customer approval)
  - `scope`: enum (`read-only`)
- **relationships**:
  - Generates **AuditEvent** (`support.impersonation.start/stop`)
  - Binds to **MemberAccount** target when consent captured
- **rules**:
  - All interactions read-only; write attempts blocked with explicit messaging.
  - Session auto-terminates at `expires_at` or upon manual revoke.

