# Quickstart: UI Update & Admin Portal Rebuild

**Feature**: 018-ui-update  
**Date**: 2025-01-27

This guide helps developers get started with the UI rebuild work.

## Prerequisites

- **Node.js**: 20.x or later
- **pnpm**: 8.x or later
- **Git**: For version control
- **Docker**: Optional, for local backend services
- **Access**: Remote dev cluster access for testing

## Initial Setup

### 1. Clone and Navigate

```bash
git clone <repository-url>
cd ai-aas
git checkout feature/018-ui-update  # or create branch
cd web/portal
```

### 2. Install Dependencies

```bash
pnpm install
```

### 3. Configure Environment

Create `.env.local` (optional, for local development):

```bash
# API Configuration
VITE_API_BASE_URL=http://localhost:8080/api

# OAuth (if using)
VITE_OAUTH_CLIENT_ID=your-client-id
VITE_OAUTH_ISSUER_URL=http://localhost:8080
VITE_OAUTH_REDIRECT_URI=http://localhost:5173/auth/callback

# Feature Flags (optional)
VITE_FEATURE_FLAGS_API_URL=http://localhost:8080/api/v1/feature-flags

# OpenTelemetry (optional)
VITE_OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
VITE_OTEL_SERVICE_NAME=web-portal
VITE_OTEL_SERVICE_VERSION=0.1.0
```

**Note**: The new centralized config (`/src/config/api.ts`) automatically detects environment (localhost, nip.io, production) and computes API URLs accordingly.

## Development Workflow

### Start Development Server

```bash
pnpm dev
```

Server runs at `https://localhost:5173` (HTTPS with self-signed cert).

### Run Type Checking

```bash
pnpm build  # Runs tsc
```

### Run Linting

```bash
pnpm lint
```

### Run Unit Tests

```bash
pnpm test           # Run once
pnpm test:watch     # Watch mode
pnpm test:coverage  # With coverage
```

### Run E2E Tests

#### Local Development Server

```bash
# Start dev server in one terminal
pnpm dev

# Run tests in another terminal
pnpm test:e2e
```

#### Remote Dev Cluster

```bash
# Run smoke tests against remote cluster
PLAYWRIGHT_BASE_URL=https://portal.dev.otherjamesbrown.com \
SKIP_WEBSERVER=true \
pnpm playwright test tests/e2e/smoke.spec.ts --project=chromium

# Run full test suite
PLAYWRIGHT_BASE_URL=https://portal.dev.otherjamesbrown.com \
SKIP_WEBSERVER=true \
TEST_AUTH_TOKEN="<token>" \
pnpm playwright test
```

## Architecture Overview

### Key Files

- **`/src/config/api.ts`**: Centralized API configuration (NEW)
- **`/src/lib/http/client.ts`**: HTTP clients (`httpClient`, `publicClient`)
- **`/src/services/tokenManager.ts`**: Token refresh management (NEW)
- **`/src/services/healthMonitor.ts`**: Health check singleton (NEW)
- **`/src/providers/AppProviders.tsx`**: Combined provider wrapper (NEW)

### Component Structure

```
src/
├── components/
│   ├── layout/          # Layout components (Sidebar, Header, Footer)
│   └── ui/              # Reusable UI components (DataTable, StatusBadge, etc.)
├── app/
│   ├── pages/           # Page components
│   └── features/         # Feature-specific pages organized by CLI command groups
│       ├── admin/       # Existing admin pages
│       ├── model-management/  # Model Management UI (NEW)
│       ├── access-control/    # Access Control UI (NEW)
│       ├── platform-operations/ # Platform Operations UI (NEW)
│       └── utilities/         # Utilities UI (NEW)
```

## Common Tasks

### Add a New Page

1. Create page component in appropriate feature directory:
   ```typescript
   // src/app/features/model-management/pages/ModelRegistryPage.tsx
   import { AdminLayout } from '@/components/layout/AdminLayout';
   
   export function ModelRegistryPage() {
     return (
       <AdminLayout>
         <h1>Model Registry</h1>
         {/* Page content */}
       </AdminLayout>
     );
   }
   ```

2. Add route in `AppRouter.tsx`:
   ```typescript
   {
     path: '/model-management/registry',
     component: ModelRegistryPage,
   }
   ```

3. Add navigation link in `Sidebar.tsx`

### Use HTTP Clients

```typescript
import { httpClient } from '@/lib/http/client';
import { publicClient } from '@/lib/http/client';

// Authenticated request
const orgs = await httpClient.get('/api/v1/admin/organizations');

// Unauthenticated request (login, health check)
const health = await publicClient.get('/api/v1/status/healthz');
```

### Use Theme

```typescript
import { useTheme } from '@/hooks/useTheme';

function MyComponent() {
  const { mode, toggleTheme } = useTheme();
  
  return (
    <div className="bg-background text-foreground">
      <button onClick={toggleTheme}>
        Current theme: {mode}
      </button>
    </div>
  );
}
```

### Use Health Monitor

```typescript
import { useHealthStatus } from '@/hooks/useHealthStatus';

function HealthCheck() {
  const status = useHealthStatus();
  
  return (
    <div>
      {Object.entries(status).map(([name, health]) => (
        <div key={name}>
          {name}: {health.status}
        </div>
      ))}
    </div>
  );
}
```

## Testing

### Test Structure

```
tests/e2e/
├── smoke.spec.ts        # Fast smoke tests (< 2 min)
├── login.spec.ts         # Login flow tests
├── api-keys.spec.ts      # API Keys tests
├── helpers/
│   └── auth.ts           # Auth helpers
└── pages/
    └── LoginPage.ts      # Page objects
```

### Writing E2E Tests

```typescript
import { test, expect } from '@playwright/test';
import { LoginPage } from './pages/LoginPage';

test('can login and view dashboard', async ({ page }) => {
  const loginPage = new LoginPage(page);
  await loginPage.goto();
  await loginPage.login('admin@example.com', 'password');
  
  await expect(page).toHaveURL('/');
  await expect(page.getByRole('heading', { name: /dashboard/i })).toBeVisible();
});
```

### Running Specific Tests

```bash
# Run single test file
pnpm playwright test tests/e2e/login.spec.ts

# Run with UI mode
pnpm playwright test --ui

# Run with headed browser
PLAYWRIGHT_HEADLESS=false pnpm playwright test

# Run specific test
pnpm playwright test -g "can login"
```

## Building for Production

### Build

```bash
pnpm build
```

Output: `dist/` directory with static assets.

### Preview Production Build

```bash
pnpm preview
```

### Docker Build

```bash
# From repository root
docker build -f web/portal/Dockerfile -t web-portal:latest .
```

## Troubleshooting

### Port Already in Use

```bash
# Change port in vite.config.ts or use different port
pnpm dev -- --port 5174
```

### API Connection Issues

1. Check API URL in browser console (should show computed config)
2. Verify backend services are running
3. Check CORS configuration on backend
4. Verify API key/token is valid

### Health Check Flickering

- Ensure using `healthMonitor` singleton pattern
- Check React StrictMode (double-execution in dev)
- Verify `initialized.current` guard is in place

### Theme Not Applying

1. Check `tailwind.config.js` has `darkMode: 'class'` or `darkMode: 'media'`
2. Verify `useTheme` hook is used correctly
3. Check `localStorage` for theme preference
4. Verify CSS variables are defined in `global.css`

## Environment-Specific Configuration

### Localhost Development

- API URL: `http://localhost:8080/api` (from env or default)
- Portal URL: `https://localhost:5173`
- Auto-detected as `environment: 'local'`

### Nip.io Development

- Portal URL: `https://portal.dev.otherjamesbrown.com`
- API URL: Auto-computed as `https://api.dev.otherjamesbrown.com/api`
- Auto-detected as `environment: 'development'`

### Production

- Portal URL: `https://portal.dev.otherjamesbrown.com` (or production domain)
- API URL: From `VITE_API_BASE_URL` env var or computed
- Auto-detected as `environment: 'production'`

## Next Steps

1. Review `plan.md` for implementation details
2. Review `research.md` for architectural decisions
3. Review `data-model.md` for entity definitions
4. Start with Phase 1: Foundation (centralized config, HTTP clients)
5. Follow task breakdown in `tasks.md` (generated via `/speckit.tasks`)

## Getting Help

- **Architecture Questions**: See `architecture-recommendations.md`
- **Testing Questions**: See `testing-plan.md`
- **CLI Command Coverage**: See `spec.md` CLI Command Coverage Reference section
- **Constitution Compliance**: See `.specify/memory/constitution.md`

## CI/CD Integration

### GitHub Actions

Smoke tests run automatically on PR:
- Tests against remote dev cluster
- Blocks merge on failure
- Collects test artifacts on failure

### Manual CI Trigger

```bash
# From repository root
make ci-remote
```

This triggers remote CI workflow that runs all checks including Playwright tests.

---

**Ready to start?** Begin with Phase 1: Foundation tasks in `tasks.md`.

