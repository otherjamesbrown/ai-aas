# Spec: Token Rate-Limit Policies

**Spec Number:** 035
**Epic Bead:** aas-2zku
**Status:** Draft
**Created:** 2026-01-01

## Overview

Token Rate-Limit Policies allow organization administrators to control API usage by setting token consumption limits across rolling time windows (1 hour, 24 hours, 7 days). Policies can be applied as an organization default or overridden per-user.

### Goals

1. **Flexible Rate Limiting** - Support any combination of 1h/24h/7d token limits
2. **Hierarchical Application** - Org default with per-user overrides
3. **Usage Transparency** - Users can see their consumption as % and absolute values
4. **OpenAI Compatibility** - Error responses and headers match OpenAI API spec
5. **Fast Enforcement** - Check limits at API Router for minimal latency

### Non-Goals

- Monetary/cost-based budgets (different feature)
- Per-model token limits (future enhancement)
- Service account rate limits (service accounts are unlimited)
- Custom time windows beyond 1h/24h/7d (keep it simple)
- Override approval workflows (direct assignment only)

---

## User Scenarios

### US-1: Org Admin Creates Token Rate-Limit Policy

**Actor:** Organization Administrator
**Precondition:** Logged in with org admin permissions

1. Org admin wants to limit their team's API usage
2. Runs `ai-aas-org token-policy create --name "standard" --1h 10000 --24h 100000 --7d 500000`
3. System creates policy with all three limits
4. CLI displays policy ID and confirmation

**Acceptance Criteria:**
- Policy is created in org admin's organization
- All three limits are optional (any combination valid)
- Policy name must be unique within org
- Cannot create policy named "No Token Rate-Limit" (reserved)

### US-2: Org Admin Creates Policy with Partial Limits

**Actor:** Organization Administrator
**Precondition:** Logged in with org admin permissions

1. Org admin wants hourly burst control but no long-term limits
2. Runs `ai-aas-org token-policy create --name "burst-control" --1h 5000`
3. System creates policy with only 1h limit (24h and 7d unlimited)
4. CLI displays policy details

**Acceptance Criteria:**
- Omitted time windows have no limit (null)
- At least one limit must be specified
- Policy is valid with any single limit

### US-3: Org Admin Sets Organization Default Policy

**Actor:** Organization Administrator
**Precondition:** Policy "standard" exists in org

1. Org admin wants all users to have the "standard" policy by default
2. Runs `ai-aas-org token-policy set-default --policy standard`
3. System sets org's default policy
4. All users without overrides now inherit "standard"

**Acceptance Criteria:**
- New orgs default to "No Token Rate-Limit" (system-wide singleton)
- Setting default affects all users without explicit overrides
- Can set back to "No Token Rate-Limit" to remove limits

### US-4: Org Admin Overrides Policy for Specific User

**Actor:** Organization Administrator
**Precondition:** User "bob" exists, policy "low-usage" exists

1. Org admin needs to restrict Bob's usage below org default
2. Runs `ai-aas-org user set-token-policy --user bob --policy low-usage`
3. System sets Bob's policy override
4. Bob now uses "low-usage" regardless of org default

**Acceptance Criteria:**
- User override takes precedence over org default
- Can assign "No Token Rate-Limit" to give user unlimited access
- Override is removed with `--policy inherit` (falls back to org default)

### US-5: User Queries Their Token Usage

**Actor:** API User
**Precondition:** User has active API key

1. User wants to check their remaining tokens
2. Calls `GET /v1/usage/tokens` with their API key
3. System returns usage for each applicable time window

**Response:**
```json
{
  "policy_name": "standard",
  "windows": [
    {
      "window": "1h",
      "limit": 10000,
      "used": 3500,
      "remaining": 6500,
      "percentage": 35.0,
      "resets_at": "2026-01-01T15:00:00Z"
    },
    {
      "window": "24h",
      "limit": 100000,
      "used": 45000,
      "remaining": 55000,
      "percentage": 45.0,
      "resets_at": "2026-01-02T10:00:00Z"
    },
    {
      "window": "7d",
      "limit": 500000,
      "used": 120000,
      "remaining": 380000,
      "percentage": 24.0,
      "resets_at": "2026-01-07T10:00:00Z"
    }
  ]
}
```

**Acceptance Criteria:**
- Only shows windows with limits (not unlimited windows)
- Shows percentage (0-100) for each window
- Shows absolute used/remaining/limit
- Shows reset time for each window

### US-6: User Hits Rate Limit

**Actor:** API User
**Precondition:** User has consumed 24h token limit

1. User makes API request
2. API Router checks cached limits
3. Request is rejected with HTTP 429

**Response:**
```json
{
  "error": {
    "message": "Rate limit exceeded: 24h token limit (100000) reached. Resets at 2026-01-02T10:00:00Z",
    "type": "tokens",
    "code": "rate_limit_exceeded",
    "window": "24h",
    "limit": 100000,
    "used": 100000,
    "resets_at": "2026-01-02T10:00:00Z"
  }
}
```

**Headers (OpenAI compatible):**
```
x-ratelimit-limit-tokens: 100000
x-ratelimit-remaining-tokens: 0
x-ratelimit-reset-tokens: 2026-01-02T10:00:00Z
Retry-After: 51780
```

**Acceptance Criteria:**
- Error message specifies which window triggered the limit
- OpenAI-compatible headers are included
- Retry-After is seconds until reset
- Multiple windows can be exceeded; report the one resetting soonest

### US-7: Request Exceeds Limit Mid-Request

**Actor:** API User
**Precondition:** User has 500 tokens remaining in 1h window

1. User sends request estimating 200 tokens
2. API Router allows request (under limit)
3. Actual response uses 800 tokens (over limit)
4. System records 800 tokens used (now over by 300)
5. User's NEXT request is blocked until window resets

**Acceptance Criteria:**
- Pre-request check uses estimated tokens
- Actual usage is recorded post-request
- Overage is allowed for current request
- Subsequent requests are blocked until window resets

### US-8: Org Admin Views User Usage

**Actor:** Organization Administrator
**Precondition:** Logged in with org admin permissions

1. Org admin wants to check Bob's usage
2. Runs `ai-aas-org user usage --user bob`
3. CLI displays Bob's current usage across all windows

**Acceptance Criteria:**
- Admins can view any user's usage in their org
- Shows effective policy (inherited or overridden)
- Shows all active windows with usage stats

### US-9: User with No Token Rate-Limit Policy

**Actor:** API User
**Precondition:** Org default is "No Token Rate-Limit"

1. User makes API request
2. API Router sees no limits configured
3. Request proceeds without rate limit check

**Response to usage query:**
```json
{
  "policy_name": "No Token Rate-Limit",
  "windows": []
}
```

**Acceptance Criteria:**
- No rate limiting applied when policy is "No Token Rate-Limit"
- Usage query returns empty windows array
- Still track usage for analytics (separate from enforcement)

---

## Functional Requirements

### Policy Management

- **FR-001**: Org admins can create named token rate-limit policies with optional limits for 1h, 24h, and 7d windows
- **FR-002**: At least one time window limit must be specified when creating a policy
- **FR-003**: Policy names must be unique within an organization
- **FR-004**: A system-wide "No Token Rate-Limit" policy exists that all orgs can reference
- **FR-005**: Policies cannot be deleted if assigned to users or set as org default (409 Conflict)

### Policy Assignment

- **FR-006**: New organizations default to "No Token Rate-Limit"
- **FR-007**: Org admins can set any policy (including "No Token Rate-Limit") as org default
- **FR-008**: Org admins can override policy for individual users
- **FR-009**: User policy override takes precedence over org default
- **FR-010**: Users can be set to "inherit" to remove override and use org default

### Token Counting

- **FR-011**: Token usage = input tokens + output tokens (total_tokens)
- **FR-012**: Usage is tracked per-user across all models (not per-model)
- **FR-013**: Service accounts are not subject to token rate limits

### Enforcement

- **FR-014**: Rate limit enforcement happens at API Router (fast path)
- **FR-015**: API Router caches user's effective policy and current usage
- **FR-016**: Pre-request: Reject if any window's limit is already exceeded
- **FR-017**: Post-request: Record actual tokens used, even if it exceeds limit
- **FR-018**: Allow current request to complete; block subsequent requests if over limit

### Error Responses

- **FR-019**: Rate limit errors return HTTP 429 with OpenAI-compatible format
- **FR-020**: Error includes which window triggered and when it resets
- **FR-021**: Include standard headers: x-ratelimit-limit-tokens, x-ratelimit-remaining-tokens, x-ratelimit-reset-tokens, Retry-After

### Usage Visibility

- **FR-022**: Users can query their own token usage via API
- **FR-023**: Org admins can query any user's usage in their org
- **FR-024**: Usage shows percentage consumed and absolute values
- **FR-025**: Usage shows reset time for each window

---

## API Endpoints

### Policy Management (Org Admin)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/v1/orgs/{orgId}/token-policies` | Create policy |
| GET | `/v1/orgs/{orgId}/token-policies` | List policies |
| GET | `/v1/orgs/{orgId}/token-policies/{policyId}` | Get policy |
| PUT | `/v1/orgs/{orgId}/token-policies/{policyId}` | Update policy |
| DELETE | `/v1/orgs/{orgId}/token-policies/{policyId}` | Delete policy |

### Org Default (Org Admin)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/orgs/{orgId}/token-policy` | Get org default policy |
| PUT | `/v1/orgs/{orgId}/token-policy` | Set org default policy |

### User Policy Override (Org Admin)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/orgs/{orgId}/users/{userId}/token-policy` | Get user's effective policy |
| PUT | `/v1/orgs/{orgId}/users/{userId}/token-policy` | Set user's policy override |
| DELETE | `/v1/orgs/{orgId}/users/{userId}/token-policy` | Remove override (inherit) |

### Usage Query

| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/usage/tokens` | Get own usage (user) |
| GET | `/v1/orgs/{orgId}/users/{userId}/usage/tokens` | Get user's usage (admin) |

---

## Data Model

### TokenRateLimitPolicy

| Field | Type | Description |
|-------|------|-------------|
| id | UUID | Primary key |
| org_id | UUID | Owning org (null for system "No Token Rate-Limit") |
| name | string | Human-readable name |
| description | string | Optional description |
| limit_1h | int | 1-hour limit (null = unlimited) |
| limit_24h | int | 24-hour limit (null = unlimited) |
| limit_7d | int | 7-day limit (null = unlimited) |
| is_builtin | bool | True for "No Token Rate-Limit" |
| created_at | timestamp | Creation time |
| updated_at | timestamp | Last update time |

### Organization (additions)

| Field | Type | Description |
|-------|------|-------------|
| default_token_policy_id | UUID | FK to TokenRateLimitPolicy |

### User (additions)

| Field | Type | Description |
|-------|------|-------------|
| token_policy_override_id | UUID | FK to TokenRateLimitPolicy (null = inherit) |

### TokenUsageWindow

| Field | Type | Description |
|-------|------|-------------|
| user_id | UUID | User being tracked |
| window_type | enum | '1h', '24h', '7d' |
| window_start | timestamp | Start of rolling window |
| tokens_used | bigint | Tokens consumed in window |
| updated_at | timestamp | Last update time |

---

## Architecture

```
┌─────────────┐      ┌──────────────────┐      ┌─────────────────┐
│   Client    │─────▶│   API Router     │─────▶│  Inference      │
│             │      │                  │      │  Backend        │
└─────────────┘      └────────┬─────────┘      └────────┬────────┘
                              │                         │
                     1. Check limits            3. Report usage
                        (cached)                        │
                              │                         │
                     ┌────────▼─────────────────────────▼───┐
                     │         User-Org Service             │
                     │   (source of truth for policies      │
                     │    and usage aggregation)            │
                     └──────────────────────────────────────┘
```

**Flow:**
1. API Router caches user's effective policy on first request
2. Pre-request: Check if any window limit exceeded
3. Forward to inference backend if allowed
4. Post-request: Report actual tokens used to user-org-service
5. User-org-service aggregates usage into rolling windows

---

## Success Criteria

- **SC-001**: Org admin can create, list, update, delete token policies
- **SC-002**: Org admin can set org default and per-user overrides
- **SC-003**: Users see accurate usage % and absolute values
- **SC-004**: Rate-limited requests return OpenAI-compatible 429
- **SC-005**: Enforcement adds < 10ms latency to requests (cached)
- **SC-006**: Usage is accurately tracked within 1 second of request completion

---

## Open Questions (Resolved)

1. ~~Custom time windows?~~ **Decision: No, keep 1h/24h/7d fixed**
2. ~~Per-model limits?~~ **Decision: Defer to v2**
3. ~~Usage retention?~~ **Decision: 30 days**
4. ~~Cache TTL?~~ **Decision: 60 seconds**
