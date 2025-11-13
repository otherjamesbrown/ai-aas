/**
 * Admin domain types for organization management
 */

export type OrgStatus = 'active' | 'suspended' | 'trial';
export type MemberRole = 'owner' | 'admin' | 'manager' | 'analyst' | 'custom';
export type InviteStatus = 'pending' | 'accepted' | 'revoked' | 'expired';
export type ApiKeyStatus = 'active' | 'revoked' | 'rotated';
export type EnforcementMode = 'monitor' | 'warn' | 'block';
export type AuditResult = 'success' | 'failure';

export interface BillingContact {
  name: string;
  email: string;
  phone?: string;
}

export interface Address {
  country: string; // ISO country code
  region?: string;
  postal_code: string;
  street_lines: string[];
}

export interface OrganizationProfile {
  org_id: string; // UUID
  name: string; // 1-120 chars
  status: OrgStatus;
  billing_contact: BillingContact;
  address: Address;
  policy_flags: string[]; // e.g., 'mfa_required', 'impose_budget_lock'
  updated_at: string; // ISO-8601 timestamp
}

export interface MemberAccount {
  member_id: string; // UUID
  identity_id: string; // UUID
  email: string; // RFC5322-compliant
  role: MemberRole;
  invite_status: InviteStatus;
  mfa_required: boolean;
  last_active_at: string | null; // ISO-8601 timestamp
  scopes: string[]; // API scopes for fine-grained permissions
}

export interface BudgetPolicy {
  policy_id: string; // UUID
  monthly_limit_cents: number; // >= 0
  alert_thresholds: number[]; // percentages, ascending
  currency: string; // ISO 4217 code
  enforcement_mode: EnforcementMode;
  alert_recipients: string[]; // email addresses
  effective_at: string; // ISO-8601 timestamp
}

export interface ApiKeyCredential {
  key_id: string; // UUID
  display_name: string;
  scopes: string[]; // permission identifiers
  status: ApiKeyStatus;
  created_at: string; // ISO-8601 timestamp
  rotated_at?: string; // ISO-8601 timestamp
  fingerprint: string; // last 8 characters (masked)
  expires_at?: string | null; // ISO-8601 timestamp
}

export interface AuditEvent {
  event_id: string; // UUID
  actor_id: string; // UUID
  organization_id: string; // UUID
  action: string; // e.g., 'member.invite', 'budget.update', 'apikey.revoke'
  target: string | Record<string, unknown>; // JSON summarizing affected entity
  result: AuditResult;
  timestamp: string; // ISO-8601
  metadata?: {
    ip?: string;
    user_agent?: string;
    impersonation?: boolean;
    [key: string]: unknown;
  };
}

// Request/Response types for API calls
export interface InviteMemberRequest {
  email: string;
  role: MemberRole;
  scopes?: string[];
}

export interface UpdateOrganizationRequest {
  name?: string;
  billing_contact?: Partial<BillingContact>;
  address?: Partial<Address>;
}

export interface UpdateBudgetRequest {
  monthly_limit_cents?: number;
  alert_thresholds?: number[];
  currency?: string;
  enforcement_mode?: EnforcementMode;
  alert_recipients?: string[];
}

export interface CreateApiKeyRequest {
  display_name: string;
  scopes: string[];
  expires_at?: string; // ISO-8601 timestamp
}

export interface ApiKeyResponse extends ApiKeyCredential {
  secret?: string; // Only present on creation
}

