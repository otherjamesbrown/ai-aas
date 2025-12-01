# AI-AAS Admin Portal

Web-based administration portal for the AI-AAS platform. Provides equivalent functionality to the CLI for managing models, organizations, users, and API keys.

## Architecture

### Folder Structure

```
src/
├── app/                    # Application routes and pages
│   ├── pages/              # Top-level pages (Home, Login)
│   ├── routes/             # Route definitions by domain
│   └── features/           # Feature-based pages
│       ├── model-management/
│       ├── access-control/
│       └── platform-operations/
├── components/
│   ├── ui/                 # Reusable UI components (Button, DataTable, etc.)
│   └── layout/             # Layout components (Sidebar, Header, Footer)
├── config/                 # Centralized configuration
├── features/               # Legacy feature modules
├── hooks/                  # Custom React hooks
├── lib/
│   └── http/               # HTTP client infrastructure
├── providers/              # React context providers
└── styles/                 # Global styles and themes
```

### Centralized Configuration

All API endpoints and configuration are managed through `src/config/api.ts`:

```typescript
import { apiConfig } from '@/config/api';

// API endpoint
const url = `${apiConfig.apiUrl}/users`;

// Auth endpoint
const authUrl = `${apiConfig.baseUrl}/v1/auth/login`;
```

### HTTP Client Patterns

The portal uses two HTTP clients from `src/lib/http/client.ts`:

1. **`httpClient`** - For authenticated requests (includes auth token automatically)
2. **`publicClient`** - For unauthenticated requests (login, health checks)

```typescript
import { httpClient, publicClient } from '@/lib/http/client';

// Authenticated request
const users = await httpClient.get('/users');

// Public request (no auth)
const health = await publicClient.get('/healthz');
```

### Provider Structure

Providers are organized in `src/providers/AppProviders.tsx` with a max depth of 4 levels:

1. StrictMode
2. ErrorBoundary
3. AppProviders (Auth, Query, Toast)
4. RouterProvider

## Development

### Prerequisites

- Node.js 20+
- pnpm 9+

### Setup

```bash
# Install dependencies
pnpm install

# Start development server
pnpm dev
```

### Environment Variables

Create a `.env.local` file:

```env
VITE_USER_ORG_SERVICE_URL=http://localhost:8081
VITE_API_ROUTER_SERVICE_URL=http://localhost:8080
```

## Testing

### Unit Tests

```bash
# Run unit tests
pnpm test

# Watch mode
pnpm test:watch
```

### E2E Tests

The portal uses Playwright for E2E testing.

```bash
# Install Playwright browsers
pnpm exec playwright install

# Run smoke tests (fast, Chromium only)
pnpm exec playwright test --project=smoke

# Run full test suite
pnpm exec playwright test

# Run against remote cluster
PLAYWRIGHT_BASE_URL=https://portal.example.com SKIP_WEBSERVER=true pnpm exec playwright test --project=smoke
```

### Test Projects

- **smoke** - Fast tests (< 2 min), Chromium only, critical paths
- **chromium/firefox/webkit** - Full test suite, all browsers
- **accessibility** - A11y tests with axe-core

## CLI Equivalents

Each UI page corresponds to CLI commands:

| UI Page | CLI Command |
|---------|-------------|
| Model Registry | `ai-aas-cli model registry list` |
| Model Cache | `ai-aas-cli model cache status` |
| Model Deploy | `ai-aas-cli model deploy list` |
| Organizations | `ai-aas-cli org list` |
| Users | `ai-aas-cli user list` |
| API Keys | `ai-aas-cli apikey list` |
| Platform Status | `ai-aas-cli status` |
| Usage Dashboard | `ai-aas-cli usage report` |

## Building for Production

```bash
# Build
pnpm build

# Preview production build
pnpm preview
```

## Linting and Type Checking

```bash
# Type check
pnpm exec tsc --noEmit

# Lint
pnpm lint

# Format
pnpm format
```
