# API Contracts: UI Update & Admin Portal Rebuild

**Feature**: 018-ui-update  
**Date**: 2025-01-27  
**Phase**: 1 - Design & Contracts

## Overview

The UI rebuild is a **thin client** that uses existing backend REST APIs. No new API endpoints are required. This document references the existing API contracts used by the UI.

## API-First Principle

Per Constitution Gate I (API-First Interfaces), the UI:
- ✅ Uses existing documented REST APIs
- ✅ Contains no business logic
- ✅ Is a thin client layer
- ✅ Uses same APIs as ai-aas-cli tool

## Backend API References

The UI uses the **same API services and endpoints as the CLI**. Base URLs are computed from the portal domain (e.g., `portal.dev.ai-aas.local` → `api.dev.ai-aas.local`, `user-org.dev.ai-aas.local`).

### User-Org Service
- **Base URL**: `https://user-org.{domain}` (e.g., `https://user-org.dev.ai-aas.local`)
- **Authentication**: Bearer token (JWT)
- **Endpoints Used** (same as CLI):
  - `GET /v1/orgs` - List organizations
  - `POST /v1/orgs` - Create organization
  - `PUT /v1/orgs/{orgID}` - Update organization
  - `DELETE /v1/orgs/{orgID}` - Delete organization
  - `GET /v1/orgs/{orgID}/users` - List users in organization
  - `POST /v1/orgs/{orgID}/invites` - Invite user to organization
  - `PUT /v1/orgs/{orgID}/users/{userID}` - Update user
  - `DELETE /v1/orgs/{orgID}/users/{userID}` - Delete user
  - `GET /v1/orgs/{orgID}/api-keys` - List API keys
  - `POST /v1/orgs/{orgID}/users/{userID}/api-keys` - Create API key
  - `DELETE /v1/orgs/{orgID}/api-keys/{apiKeyID}` - Delete API key
  - `GET /v1/orgs/{orgID}/users/{userID}/model-access` - Get user model access
  - `PUT /v1/orgs/{orgID}/users/{userID}/model-access/mode` - Set access mode
  - `POST /v1/orgs/{orgID}/users/{userID}/model-access/grants` - Grant model access
  - `DELETE /v1/orgs/{orgID}/users/{userID}/model-access/grants/{modelName}` - Revoke model access

### API Router Service (Gateway)
- **Base URL**: `https://api.{domain}` (e.g., `https://api.dev.ai-aas.local`)
- **Authentication**: Bearer token (JWT) or API key
- **Endpoints Used** (same as CLI):
  - `GET /v1/models` - List available models
  - `POST /v1/chat/completions` - Chat completions (OpenAI-compatible)
  - `POST /v1/completions` - Text completions (OpenAI-compatible)
  - `GET /v1/status/healthz` - Health check (public)
  - `GET /v1/status/readyz` - Readiness check (public)
  - `POST /v1/auth/login` - User login (proxied to user-org-service)
  - `POST /v1/auth/token` - Token refresh (proxied to user-org-service)
  - `POST /v1/auth/logout` - User logout (proxied to user-org-service)
  - `GET /v1/auth/userinfo` - Get user info (proxied to user-org-service)

### Admin API Service
- **Base URL**: `https://admin-api.{domain}` (e.g., `https://admin-api.dev.ai-aas.local`)
- **Authentication**: Bearer token (JWT, admin only)
- **Endpoints Used** (same as CLI):
  - `POST /v1/routing/policies` - Create routing policy
  - `GET /v1/routing/policies` - List routing policies
  - `DELETE /v1/routing/policies/{policyID}` - Delete routing policy
  - `POST /v1/bootstrap` - Bootstrap platform (first admin account)

### Analytics Service
- **Base URL**: `https://analytics.{domain}` (e.g., `https://analytics.dev.ai-aas.local`)
- **Authentication**: Bearer token (JWT)
- **Endpoints Used** (same as CLI):
  - `GET /analytics/v1/usage` - Query usage data
  - `GET /analytics/v1/usage/summary` - Get usage summary
  - `GET /analytics/v1/usage/export` - Export usage data

**Note**: Model management operations (registry, cache, deploy) may use direct database access via CLI or future model-service API endpoints. UI will use the same approach as CLI.

## API Contract Standards

All APIs follow Constitution requirements:

### Request Format
- **Method**: REST (GET, POST, PUT, DELETE)
- **Content-Type**: `application/json`
- **Authentication**: Bearer token in `Authorization` header (except public endpoints)
- **Headers**: 
  - `X-Correlation-ID`: UUID for request tracing
  - `Content-Type`: `application/json`

### Response Format
- **Status Codes**: Standard HTTP status codes (200, 201, 400, 401, 403, 404, 500)
- **Error Format**: RFC7807 Problem Details
- **Success Format**: JSON object or array
- **Timestamps**: ISO-8601 format
- **Pagination**: Cursor-based (when applicable)

### Error Handling
- **401 Unauthorized**: Redirect to login (handled by `httpClient` interceptor)
- **403 Forbidden**: Show error message
- **404 Not Found**: Show error message
- **500 Server Error**: Show error message with retry option

## OpenAPI Specifications

Full API specifications are available in:
- Backend service repositories
- API documentation portal (if available)
- OpenAPI schema files in backend services

## UI API Client Implementation

The UI uses standardized HTTP clients:

### Authenticated Requests
```typescript
import { httpClient } from '@/lib/http/client';
import { apiConfig } from '@/config/api';

// Compute service base URL from portal domain
const userOrgBaseUrl = apiConfig.baseUrl.replace('api.', 'user-org.');

// All authenticated requests use httpClient with service-specific base URLs
const response = await httpClient.get(`${userOrgBaseUrl}/v1/orgs`);
```

### Unauthenticated Requests
```typescript
import { publicClient } from '@/lib/http/client';
import { apiConfig } from '@/config/api';

// Public requests (login, health check) use publicClient
// Auth endpoints are proxied through API Router
const response = await publicClient.post(`${apiConfig.baseUrl}/v1/auth/login`, credentials);
```

### Error Handling
```typescript
try {
  const response = await httpClient.get('/api/v1/admin/users');
  return response.data;
} catch (error) {
  if (error.response?.status === 401) {
    // Automatically handled by interceptor (redirects to login)
    throw error;
  }
  // Show error toast
  showError(error.response?.data?.detail || 'An error occurred');
  throw error;
}
```

## CLI Command to API Mapping

All CLI commands map to the same APIs used by the UI:

| CLI Command | API Endpoint | UI Page |
|-------------|--------------|---------|
| `ai-aas-cli org list` | `GET https://user-org.{domain}/v1/orgs` | OrganizationsPage |
| `ai-aas-cli user list --org-id {id}` | `GET https://user-org.{domain}/v1/orgs/{id}/users` | UsersPage |
| `ai-aas-cli apikey list --org-id {id}` | `GET https://user-org.{domain}/v1/orgs/{id}/api-keys` | ApiKeysPage |
| `ai-aas-cli inference get-models` | `GET https://api.{domain}/v1/models` | ModelRegistryPage |
| `ai-aas-cli status` | `GET https://api.{domain}/v1/status/healthz` | StatusPage |

**Note**: Complete mapping ensures UI operations produce identical results to CLI commands.

## Testing API Contracts

API contracts are tested via:
1. **Backend Integration Tests**: Verify API behavior
2. **E2E Tests**: Verify UI → API → Backend flow
3. **Contract Tests**: Verify request/response formats

UI tests use the same test data and scenarios as CLI tests to ensure consistency.

---

## Summary

- ✅ UI uses existing backend REST APIs
- ✅ No new API endpoints required
- ✅ Same APIs as ai-aas-cli tool
- ✅ API-First principle maintained
- ✅ OpenAPI specifications available in backend services

