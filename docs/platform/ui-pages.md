# Web Portal UI Pages

This document describes all pages in the web portal, their functionality, and the corresponding admin-cli commands where applicable.

## Page Overview

| Route | Page | Description | Roles |
|-------|------|-------------|-------|
| `/` | Home | Dashboard landing page after login | All |
| `/auth/login` | Login | Authentication page (OAuth2/OIDC) | Public |
| `/admin/organization` | Organization Settings | Manage organization profile and settings | Owner, Admin |
| `/admin/members` | Member Management | Invite, manage, and remove org members | Owner, Admin, Manager |
| `/admin/budgets` | Budget Controls | Set spending limits and alerts | Owner, Admin |
| `/admin/api-keys` | API Keys | Create, rotate, and revoke API keys | Owner, Admin |
| `/usage` | Usage Dashboard | View usage metrics, trends, and costs | Owner, Admin, Manager, Analyst |
| `/support` | Support Console | Impersonation and support tooling | Support Engineers |
| `/access-denied` | Access Denied | Shown when user lacks permissions | All |

## Functionality Matrix: UI vs Admin CLI

This table shows which functionality is available in the web portal vs the admin-cli tool.

| Functionality | Portal Page | Admin CLI Command | Notes |
|--------------|-------------|-------------------|-------|
| **Authentication** |
| User login | `/auth/login` | - | Portal only (browser-based OAuth2/OIDC) |
| Bootstrap first admin | - | `admin-cli bootstrap` | CLI only (initial setup) |
| **Organization Management** |
| View organization | `/admin/organization` | `admin-cli org list` | Both |
| Create organization | - | `admin-cli org create` | CLI only |
| Update organization | `/admin/organization` | `admin-cli org update` | Both |
| Delete organization | - | `admin-cli org delete` | CLI only (destructive) |
| **Member Management** |
| List members | `/admin/members` | `admin-cli user list` | Both |
| Invite member | `/admin/members` | `admin-cli user create` | Both |
| Update member role | `/admin/members` | `admin-cli user update` | Both |
| Remove member | `/admin/members` | `admin-cli user delete` | Both |
| **Budget Controls** |
| View budgets | `/admin/budgets` | - | Portal only |
| Set spending limits | `/admin/budgets` | - | Portal only |
| Configure alerts | `/admin/budgets` | - | Portal only |
| **API Key Management** |
| List API keys | `/admin/api-keys` | `admin-cli apikey list` | Both |
| Create API key | `/admin/api-keys` | `admin-cli apikey create` | Both |
| Rotate API key | `/admin/api-keys` | - | Portal only |
| Revoke API key | `/admin/api-keys` | `admin-cli apikey delete` | Both |
| **Usage & Analytics** |
| View usage dashboard | `/usage` | - | Portal only (visual) |
| Export usage report | `/usage` | `admin-cli export usage` | Both |
| Export memberships | - | `admin-cli export memberships` | CLI only |
| **Routing & Models** |
| List routing policies | - | `admin-cli routing policy list` | CLI only |
| Create routing policy | - | `admin-cli routing policy create` | CLI only |
| Delete routing policy | - | `admin-cli routing policy delete` | CLI only |
| Register model | - | `admin-cli registry register` | CLI only |
| Deregister model | - | `admin-cli registry deregister` | CLI only |
| Enable/disable model | - | `admin-cli registry enable/disable` | CLI only |
| List model registry | - | `admin-cli registry list` | CLI only |
| **Infrastructure** |
| Deployment status | - | `admin-cli deployment status` | CLI only |
| Trigger sync | - | `admin-cli sync trigger` | CLI only |
| Check sync status | - | `admin-cli sync status` | CLI only |
| Manage credentials | - | `admin-cli credentials` | CLI only |
| **Support** |
| Impersonation | `/support` | - | Portal only |
| Read-only org view | `/support` | - | Portal only |

## Page Details

### Home (`/`)

The landing page after authentication. Displays:
- Quick action cards for common tasks
- Summary widgets for usage and organization health
- Navigation to primary features

**Component**: `src/app/pages/HomePage.tsx`

### Login (`/auth/login`)

Authentication page supporting:
- OAuth2/OIDC login flow
- MFA challenges when required by policy
- Session restoration after expiry
- Redirect handling for protected routes

**Component**: `src/app/pages/LoginPage.tsx`

### Organization Settings (`/admin/organization`)

Manage organization profile including:
- Organization name and details
- Contact information
- Billing profile references
- Policy flags (MFA requirements, etc.)

**Component**: `src/features/admin/org/OrganizationSettingsPage.tsx`
**CLI equivalent**: `admin-cli org update`

### Member Management (`/admin/members`)

Complete member lifecycle management:
- View all organization members with status
- Invite new members (triggers email)
- Assign/change roles (Owner, Admin, Manager, Analyst)
- Resend pending invitations
- Remove members with confirmation

**Component**: `src/features/admin/members/MemberManagementPage.tsx`
**CLI equivalent**: `admin-cli user list|create|update|delete`

### Budget Controls (`/admin/budgets`)

Financial controls for the organization:
- Set monthly/quarterly spending limits
- Configure budget threshold alerts
- View spending history timeline
- Currency display preferences

**Component**: `src/features/admin/budgets/BudgetControlsPage.tsx`
**CLI equivalent**: None (portal only)

### API Keys (`/admin/api-keys`)

API key lifecycle management:
- List all keys with masked fingerprints
- Create new keys with scope assignment
- Rotate existing keys
- Revoke keys with confirmation
- Download key value (shown once at creation)

**Component**: `src/features/admin/api-keys/ApiKeysPage.tsx`
**CLI equivalent**: `admin-cli apikey list|create|delete`

### Usage Dashboard (`/usage`)

Analytics and reporting:
- Near-real-time usage metrics
- Filterable by time range and model
- Cost estimation and trends
- Exportable CSV summaries
- Empty state guidance for new orgs

**Component**: `src/features/usage/components/UsageDashboard.tsx`
**CLI equivalent**: `admin-cli export usage` (for data export)

### Support Console (`/support`)

Support engineer tooling:
- Time-bound impersonation with consent
- Read-only organization view
- Visible impersonation banners
- Audit logging of all support actions

**Component**: `src/features/support/pages/SupportConsolePage.tsx`
**CLI equivalent**: None (portal only, requires consent flow)

### Access Denied (`/access-denied`)

Shown when a user attempts to access a page without required permissions:
- Clear explanation of access denial
- Contact guidance for requesting access
- Link to return to allowed pages

**Component**: `src/app/pages/AccessDeniedPage.tsx`

## Related Documentation

- [Web Portal Spec](../../specs/008-web-portal/spec.md) - Full feature specification
- [Admin CLI](../technical/services/admin-cli.md) - CLI installation and usage
- [Access Control](./access-control.md) - Role-based access policies
