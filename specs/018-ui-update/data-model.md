# Data Model: UI Update & Admin Portal Rebuild

**Feature**: 018-ui-update  
**Date**: 2025-01-27  
**Phase**: 1 - Design & Contracts

This document defines the data entities and structures used in the UI rebuild. Note: The UI is a thin client; all persistent data is managed by backend services. This document describes client-side state and data structures.

## Client-Side Entities

### ApiConfig

**Purpose**: Centralized API configuration computed once at application startup.

**Fields**:
- `baseUrl: string` - API base URL without `/api` suffix (e.g., `https://api.example.com`)
- `apiUrl: string` - Full API URL with `/api` suffix (e.g., `https://api.example.com/api`)
- `isNipIo: boolean` - Whether running via nip.io domain
- `environment: 'local' | 'development' | 'production'` - Detected environment

**Validation Rules**:
- URLs must be valid HTTP/HTTPS URLs
- `apiUrl` must end with `/api` or be empty
- `baseUrl` must not end with `/api`
- `environment` must be one of the three valid values

**State Transitions**: None - immutable after initialization (frozen object)

**Storage**: Computed at module load, stored in memory only

---

### TokenManager

**Purpose**: Manages authentication token refresh scheduling and cancellation.

**Fields** (internal state):
- `refreshTimeout: NodeJS.Timeout | null` - Scheduled refresh timeout
- `onLogout: (() => void) | null` - Logout callback function

**Methods**:
- `setLogoutCallback(callback: () => void): void` - Set logout handler
- `scheduleRefresh(expiresIn: number): void` - Schedule token refresh
- `cancelRefresh(): void` - Cancel scheduled refresh
- `doRefresh(): Promise<void>` - Execute token refresh (private)

**Validation Rules**:
- `expiresIn` must be positive number (seconds)
- Refresh scheduled 60 seconds before expiry
- Logout callback called if refresh fails or token missing

**State Transitions**:
- `idle` → `scheduled` (when `scheduleRefresh()` called)
- `scheduled` → `refreshing` (when timeout fires)
- `refreshing` → `scheduled` (on success) or `logged-out` (on failure)
- `scheduled` → `idle` (when `cancelRefresh()` called)

**Storage**: In-memory singleton instance

---

### ServiceHealth

**Purpose**: Status information for backend services displayed in health check component.

**Fields**:
- `name: string` - Service name (e.g., "API Gateway", "CORS")
- `status: 'healthy' | 'unhealthy' | 'checking'` - Current health status
- `latency?: number` - Response latency in milliseconds (optional)

**Validation Rules**:
- `name` must be non-empty string
- `status` must be one of three valid values
- `latency` must be positive number if present

**State Transitions**:
- `checking` → `healthy` (on successful health check)
- `checking` → `unhealthy` (on failed health check)
- `healthy` → `checking` (when refresh starts)
- `unhealthy` → `checking` (when refresh starts)

**Storage**: Managed by `HealthMonitor` singleton, React components subscribe to updates

---

### Theme

**Purpose**: User theme preference for light/dark mode.

**Fields**:
- `mode: 'system' | 'light' | 'dark'` - Theme mode
- `systemPreference: 'light' | 'dark'` - Detected system preference (read-only)

**Validation Rules**:
- `mode` must be one of three valid values
- `systemPreference` automatically detected from `prefers-color-scheme`

**State Transitions**:
- `system` → `light` (user manually selects light)
- `system` → `dark` (user manually selects dark)
- `light` → `dark` (user toggles)
- `dark` → `light` (user toggles)
- `light` → `system` (user resets to system)
- `dark` → `system` (user resets to system)

**Storage**: `localStorage` key `theme-preference` (persists across sessions)

---

### HealthMonitor

**Purpose**: Singleton service managing health check execution outside React lifecycle.

**Fields** (internal state):
- `status: Map<string, ServiceHealth>` - Service status map
- `listeners: Set<() => void>` - Subscriber callbacks
- `intervalId: NodeJS.Timeout | null` - Health check interval

**Methods**:
- `start(): void` - Start health check monitoring
- `stop(): void` - Stop health check monitoring
- `subscribe(listener: () => void): () => void` - Subscribe to status updates (returns unsubscribe)
- `getStatus(): Record<string, ServiceHealth>` - Get current status

**Validation Rules**:
- Can only start if not already started
- Subscribers must be functions
- Interval must be positive number (30 seconds default)

**State Transitions**:
- `stopped` → `running` (when `start()` called)
- `running` → `stopped` (when `stop()` called)

**Storage**: In-memory singleton instance

---

## Backend Data Entities (Referenced by UI)

The UI displays and manipulates data from backend services. Key entities include:

### User
- `id: string`
- `email: string`
- `display_name?: string`
- `role: 'admin' | 'member'`
- `status: 'active' | 'suspended'`
- `organization_id: string`
- `created_at: string` (ISO-8601)
- `updated_at: string` (ISO-8601)

### Organization
- `id: string`
- `name: string`
- `slug: string`
- `status: 'active' | 'suspended'`
- `created_at: string` (ISO-8601)
- `updated_at: string` (ISO-8601)

### APIKey
- `id: string`
- `name: string`
- `key_prefix: string` (masked key)
- `scopes: string[]`
- `organization_id: string`
- `created_at: string` (ISO-8601)
- `expires_at?: string` (ISO-8601, optional)

### Model
- `id: string`
- `name: string`
- `huggingface_id: string`
- `status: 'registered' | 'cached' | 'deployed'`
- `environment: 'development' | 'staging' | 'production'`
- `created_at: string` (ISO-8601)

### UsageRecord
- `id: string`
- `organization_id: string`
- `user_id: string`
- `model_id: string`
- `tokens_used: number`
- `cost: number`
- `timestamp: string` (ISO-8601)

**Note**: Full backend entity schemas are defined in backend service specifications. UI uses these entities via REST API responses.

---

## Data Flow

### Authentication Flow
1. User submits login form → `publicClient.post('/v1/auth/login')`
2. Backend returns tokens → stored in `sessionStorage`
3. `TokenManager.scheduleRefresh()` called with `expires_in`
4. Token refresh scheduled 60 seconds before expiry
5. On refresh: `publicClient.post('/v1/auth/refresh')` → update `sessionStorage`
6. On 401: clear `sessionStorage` → redirect to login

### Health Check Flow
1. `HealthMonitor.start()` called on app initialization
2. Health checks run every 30 seconds
3. React components subscribe via `useHealthStatus()` hook
4. Status updates trigger React re-renders
5. UI displays current health status

### Theme Flow
1. App loads → detect system preference from `prefers-color-scheme`
2. Check `localStorage` for manual override
3. Apply theme (system preference or manual override)
4. User toggles theme → update `localStorage` → apply new theme
5. Theme persists across sessions via `localStorage`

---

## Validation Rules Summary

### Client-Side Validation
- API URLs: Must be valid HTTP/HTTPS URLs
- Tokens: Must be non-empty strings when present
- Theme: Must be valid mode value
- Health Status: Must be valid status value

### Backend Validation (Enforced by APIs)
- All entity IDs: UUID format
- Email addresses: Valid email format
- Timestamps: ISO-8601 format
- Scopes: Valid scope strings
- Status values: Enum values as defined in backend

---

## State Management

### React State
- Component-level state for UI interactions (forms, modals, etc.)
- TanStack Query cache for API responses
- React Context for theme and auth state

### Browser Storage
- `sessionStorage`: Authentication tokens, user data (ephemeral)
- `localStorage`: Theme preference (persistent)

### No Persistent Client Storage
- No IndexedDB usage
- No client-side database
- All persistent data managed by backend services

---

## Relationships

### User ↔ Organization
- Many-to-one: Users belong to one organization
- Organization has many users

### User ↔ APIKey
- One-to-many: User can have multiple API keys
- API keys belong to one organization

### Organization ↔ Model
- Many-to-many: Organizations can access multiple models
- Models can be accessed by multiple organizations

### User ↔ UsageRecord
- One-to-many: User has many usage records
- Usage records belong to one user and one organization

**Note**: Relationships are managed by backend services. UI displays relationships via API responses.

