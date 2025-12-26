# 018-ui-update: Testing Plan

This document defines the testing strategy for the admin portal rebuild, focusing on Playwright E2E tests run against the remote development cluster.

## Testing Strategy Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                      Testing Pyramid                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│                    ┌───────────────┐                            │
│                    │  E2E Tests    │  ← Playwright (this doc)   │
│                    │  (Few, Slow)  │                            │
│                    └───────────────┘                            │
│               ┌─────────────────────────┐                       │
│               │   Integration Tests     │  ← API contract tests │
│               │   (Some, Medium)        │                       │
│               └─────────────────────────┘                       │
│          ┌───────────────────────────────────┐                  │
│          │         Unit Tests                │  ← Vitest        │
│          │         (Many, Fast)              │                  │
│          └───────────────────────────────────┘                  │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Test Environments

| Environment | URL | Purpose |
|-------------|-----|---------|
| Local | `https://localhost:5173` | Development iteration |
| Remote Dev | `https://portal.dev.otherjamesbrown.com` | CI/CD smoke tests |
| Remote Dev (internal) | `https://portal.dev.otherjamesbrown.com` | Full test suite |

---

## Smoke Tests

### Purpose
Fast, reliable tests that verify critical paths work. Run on every deployment to catch obvious regressions.

### Characteristics
- **Execution time**: < 2 minutes total
- **Test count**: 5-10 tests
- **Browsers**: Chromium only
- **Authentication**: UI login with seeded test user
- **Failure tolerance**: Zero - any failure blocks deployment

### Smoke Test Suite

```typescript
// web/portal/tests/e2e/smoke.spec.ts

import { test, expect } from '@playwright/test';

test.describe('Smoke Tests', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. Login page loads', async ({ page }) => {
    await page.goto('/auth/login');
    await expect(page.getByRole('heading', { name: /sign in/i })).toBeVisible();
    await expect(page.getByLabel(/email/i)).toBeVisible();
    await expect(page.getByLabel(/password/i)).toBeVisible();
  });

  test('2. Service health check displays', async ({ page }) => {
    await page.goto('/auth/login');
    // Health check should show status without rapid flickering
    const healthCheck = page.getByTestId('service-health-check');
    await expect(healthCheck).toBeVisible({ timeout: 10000 });
    // Wait and verify no rapid re-renders
    await page.waitForTimeout(2000);
    await expect(healthCheck).toBeVisible();
  });

  test('3. Can authenticate with test user', async ({ page }) => {
    await page.goto('/auth/login');
    await page.getByLabel(/email/i).fill('admin@example-acme.com');
    await page.getByLabel(/password/i).fill('AcmeAdmin2024!Secure');
    await page.getByRole('button', { name: /sign in$/i }).click();

    await expect(page).toHaveURL(/^\/$/, { timeout: 15000 });
  });

  test('4. Home page loads after login', async ({ page }) => {
    // Assume authenticated from previous test (serial mode)
    await page.goto('/');
    await expect(page.getByRole('heading', { name: /dashboard|home/i })).toBeVisible();
  });

  test('5. Can navigate to API Keys', async ({ page }) => {
    await page.goto('/admin/api-keys');
    await expect(page.getByRole('heading', { name: /api keys/i })).toBeVisible({ timeout: 10000 });
  });

  test('6. Can logout', async ({ page }) => {
    await page.goto('/');
    await page.getByRole('button', { name: /user|profile|account/i }).click();
    await page.getByRole('menuitem', { name: /logout|sign out/i }).click();

    await expect(page).toHaveURL(/\/auth\/login/);
  });
});
```

### Running Smoke Tests

```bash
# Against local dev server
cd web/portal
pnpm test:e2e:smoke

# Against remote dev cluster
PLAYWRIGHT_BASE_URL=https://portal.dev.otherjamesbrown.com \
SKIP_WEBSERVER=true \
pnpm playwright test tests/e2e/smoke.spec.ts --project=chromium
```

---

## Integration Tests

### Purpose
Comprehensive tests that verify feature functionality end-to-end. Run on PR merge and nightly.

### Characteristics
- **Execution time**: 5-15 minutes total
- **Test count**: 30-50 tests
- **Browsers**: Chromium, Firefox, WebKit
- **Authentication**: API key bypass for most tests, UI login for auth tests
- **Failure tolerance**: Non-blocking for deployment, but must pass before release

### Authentication Strategy

```typescript
// web/portal/tests/e2e/helpers/auth.ts

import { Page, BrowserContext } from '@playwright/test';

/**
 * Authenticate via UI (for testing login flow)
 */
export async function loginViaUI(page: Page, email: string, password: string) {
  await page.goto('/auth/login');
  await page.getByLabel(/email/i).fill(email);
  await page.getByLabel(/password/i).fill(password);
  await page.getByRole('button', { name: /sign in$/i }).click();
  await page.waitForURL(/^\/$/);
}

/**
 * Authenticate via API key (faster, for non-auth tests)
 */
export async function loginViaApiKey(context: BrowserContext) {
  // Set session storage with pre-generated token
  await context.addCookies([
    {
      name: 'auth_session',
      value: process.env.TEST_AUTH_TOKEN || '',
      domain: new URL(process.env.PLAYWRIGHT_BASE_URL || 'https://localhost:5173').hostname,
      path: '/',
    },
  ]);

  // Alternatively, inject into session storage
  await context.addInitScript((token) => {
    sessionStorage.setItem('auth_token', token);
    sessionStorage.setItem('user', JSON.stringify({
      id: 'test-user-id',
      email: 'admin@example-acme.com',
      role: 'admin',
      organization_id: 'acme-org-id',
    }));
  }, process.env.TEST_AUTH_TOKEN);
}

/**
 * Test user credentials
 */
export const TEST_USERS = {
  admin: {
    email: 'admin@example-acme.com',
    password: 'AcmeAdmin2024!Secure',
    role: 'admin',
  },
  member: {
    email: 'member@example-acme.com',
    password: 'AcmeMember2024!Secure',
    role: 'member',
  },
} as const;
```

### Integration Test Suites

#### Login Tests (`login.spec.ts`)

```typescript
test.describe('Login Page', () => {
  test('displays login form correctly', async ({ page }) => { /* ... */ });
  test('shows validation errors for empty fields', async ({ page }) => { /* ... */ });
  test('shows error for invalid credentials', async ({ page }) => { /* ... */ });
  test('successfully logs in with valid credentials', async ({ page }) => { /* ... */ });
  test('redirects authenticated users to home', async ({ page }) => { /* ... */ });
  test('supports OAuth login method', async ({ page }) => { /* ... */ });
  test('displays service health status', async ({ page }) => { /* ... */ });
});
```

#### API Keys Tests (`api-keys.spec.ts`)

```typescript
test.describe('API Keys Management', () => {
  test.beforeEach(async ({ context }) => {
    await loginViaApiKey(context);
  });

  test('displays list of API keys', async ({ page }) => { /* ... */ });
  test('can create new API key', async ({ page }) => { /* ... */ });
  test('can view API key scopes', async ({ page }) => { /* ... */ });
  test('can revoke API key', async ({ page }) => { /* ... */ });
  test('can rename API key', async ({ page }) => { /* ... */ });
  test('shows created/expiry dates correctly', async ({ page }) => { /* ... */ });
  test('table supports sorting', async ({ page }) => { /* ... */ });
});
```

#### Members Tests (`members.spec.ts`)

```typescript
test.describe('Members Management', () => {
  test.beforeEach(async ({ context }) => {
    await loginViaApiKey(context);
  });

  test('displays list of organization members', async ({ page }) => { /* ... */ });
  test('can invite new member', async ({ page }) => { /* ... */ });
  test('can change member role', async ({ page }) => { /* ... */ });
  test('can remove member', async ({ page }) => { /* ... */ });
  test('cannot remove self', async ({ page }) => { /* ... */ });
});
```

#### Usage Dashboard Tests (`usage.spec.ts`)

```typescript
test.describe('Usage Dashboard', () => {
  test.beforeEach(async ({ context }) => {
    await loginViaApiKey(context);
  });

  test('displays usage metrics', async ({ page }) => { /* ... */ });
  test('can filter by date range', async ({ page }) => { /* ... */ });
  test('can filter by model', async ({ page }) => { /* ... */ });
  test('can export usage data', async ({ page }) => { /* ... */ });
  test('shows usage breakdown table', async ({ page }) => { /* ... */ });
});
```

---

## Page Object Models

### Purpose
Encapsulate page interactions to reduce test duplication and improve maintainability.

### Example: Login Page Object

```typescript
// web/portal/tests/e2e/pages/LoginPage.ts

import { Page, Locator, expect } from '@playwright/test';

export class LoginPage {
  readonly page: Page;
  readonly emailInput: Locator;
  readonly passwordInput: Locator;
  readonly submitButton: Locator;
  readonly oauthButton: Locator;
  readonly healthCheck: Locator;
  readonly errorMessage: Locator;

  constructor(page: Page) {
    this.page = page;
    this.emailInput = page.getByLabel(/email/i);
    this.passwordInput = page.getByLabel(/password/i);
    this.submitButton = page.getByRole('button', { name: /sign in$/i });
    this.oauthButton = page.getByRole('button', { name: /sign in with oauth/i });
    this.healthCheck = page.getByTestId('service-health-check');
    this.errorMessage = page.getByRole('alert');
  }

  async goto() {
    await this.page.goto('/auth/login');
  }

  async login(email: string, password: string) {
    await this.emailInput.fill(email);
    await this.passwordInput.fill(password);
    await this.submitButton.click();
  }

  async expectLoaded() {
    await expect(this.emailInput).toBeVisible();
    await expect(this.passwordInput).toBeVisible();
    await expect(this.submitButton).toBeVisible();
  }

  async expectError(message: RegExp) {
    await expect(this.errorMessage).toContainText(message);
  }
}
```

### Example: API Keys Page Object

```typescript
// web/portal/tests/e2e/pages/ApiKeysPage.ts

import { Page, Locator, expect } from '@playwright/test';

export class ApiKeysPage {
  readonly page: Page;
  readonly heading: Locator;
  readonly createButton: Locator;
  readonly table: Locator;

  constructor(page: Page) {
    this.page = page;
    this.heading = page.getByRole('heading', { name: /api keys/i });
    this.createButton = page.getByRole('button', { name: /create.*token/i });
    this.table = page.getByRole('table');
  }

  async goto() {
    await this.page.goto('/admin/api-keys');
  }

  async expectLoaded() {
    await expect(this.heading).toBeVisible();
    await expect(this.table).toBeVisible();
  }

  async getKeyRow(label: string) {
    return this.table.getByRole('row', { name: new RegExp(label, 'i') });
  }

  async createKey(label: string) {
    await this.createButton.click();
    await this.page.getByLabel(/label/i).fill(label);
    await this.page.getByRole('button', { name: /create/i }).click();
  }

  async revokeKey(label: string) {
    const row = await this.getKeyRow(label);
    await row.getByRole('button', { name: /revoke/i }).click();
    await this.page.getByRole('button', { name: /confirm/i }).click();
  }
}
```

---

## Playwright Configuration

### Updated Configuration for Remote Testing

```typescript
// web/portal/playwright.config.ts

import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './tests/e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,

  reporter: process.env.CI
    ? [
        ['html', { outputFolder: 'playwright-report' }],
        ['json', { outputFile: 'test-results/results.json' }],
        ['github'], // GitHub Actions annotations
      ]
    : [['list'], ['html', { open: 'never' }]],

  use: {
    baseURL: process.env.PLAYWRIGHT_BASE_URL || 'https://localhost:5173',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
    ignoreHTTPSErrors: true,
  },

  projects: [
    // Smoke tests - fast, chromium only
    {
      name: 'smoke',
      testMatch: /smoke\.spec\.ts/,
      use: { ...devices['Desktop Chrome'] },
    },

    // Full tests - all browsers
    {
      name: 'chromium',
      testIgnore: /smoke\.spec\.ts/,
      use: { ...devices['Desktop Chrome'] },
    },
    {
      name: 'firefox',
      testIgnore: /smoke\.spec\.ts/,
      use: { ...devices['Desktop Firefox'] },
    },
    {
      name: 'webkit',
      testIgnore: /smoke\.spec\.ts/,
      use: { ...devices['Desktop Safari'] },
    },

    // Mobile
    {
      name: 'mobile-chrome',
      testIgnore: /smoke\.spec\.ts/,
      use: { ...devices['Pixel 5'] },
    },

    // Accessibility
    {
      name: 'accessibility',
      testMatch: /.*\.a11y\.spec\.ts/,
      use: { ...devices['Desktop Chrome'] },
    },
  ],

  webServer: process.env.SKIP_WEBSERVER
    ? undefined
    : {
        command: 'pnpm dev',
        url: 'https://localhost:5173',
        reuseExistingServer: !process.env.CI,
        timeout: 120 * 1000,
      },
});
```

---

## CI/CD Integration

### GitHub Actions Workflow

```yaml
# .github/workflows/e2e-tests.yml

name: E2E Tests

on:
  push:
    branches: [main, develop]
    paths:
      - 'web/portal/**'
  pull_request:
    branches: [main, develop]
    paths:
      - 'web/portal/**'

jobs:
  smoke-tests:
    name: Smoke Tests
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: pnpm/action-setup@v2
        with:
          version: 8

      - uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: 'pnpm'
          cache-dependency-path: web/portal/pnpm-lock.yaml

      - name: Install dependencies
        run: pnpm install
        working-directory: web/portal

      - name: Install Playwright
        run: pnpm playwright install chromium
        working-directory: web/portal

      - name: Run smoke tests
        run: |
          pnpm playwright test --project=smoke
        working-directory: web/portal
        env:
          PLAYWRIGHT_BASE_URL: https://portal.dev.otherjamesbrown.com
          SKIP_WEBSERVER: true

      - name: Upload test results
        uses: actions/upload-artifact@v4
        if: failure()
        with:
          name: smoke-test-results
          path: |
            web/portal/test-results/
            web/portal/playwright-report/

  integration-tests:
    name: Integration Tests
    runs-on: ubuntu-latest
    needs: smoke-tests
    strategy:
      matrix:
        project: [chromium, firefox, webkit]
    steps:
      - uses: actions/checkout@v4

      - uses: pnpm/action-setup@v2
        with:
          version: 8

      - uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: 'pnpm'
          cache-dependency-path: web/portal/pnpm-lock.yaml

      - name: Install dependencies
        run: pnpm install
        working-directory: web/portal

      - name: Install Playwright
        run: pnpm playwright install --with-deps ${{ matrix.project }}
        working-directory: web/portal

      - name: Run integration tests
        run: |
          pnpm playwright test --project=${{ matrix.project }}
        working-directory: web/portal
        env:
          PLAYWRIGHT_BASE_URL: https://portal.dev.otherjamesbrown.com
          SKIP_WEBSERVER: true
          TEST_AUTH_TOKEN: ${{ secrets.TEST_AUTH_TOKEN }}

      - name: Upload test results
        uses: actions/upload-artifact@v4
        if: always()
        with:
          name: test-results-${{ matrix.project }}
          path: |
            web/portal/test-results/
            web/portal/playwright-report/
```

---

## Running Tests

### Local Development

```bash
cd web/portal

# Run all tests with local dev server
pnpm test:e2e

# Run smoke tests only
pnpm test:e2e:smoke

# Run with UI mode for debugging
pnpm playwright test --ui

# Run specific test file
pnpm playwright test tests/e2e/login.spec.ts

# Run with headed browser
PLAYWRIGHT_HEADLESS=false pnpm playwright test
```

### Against Remote Cluster

```bash
cd web/portal

# Smoke tests against remote dev
PLAYWRIGHT_BASE_URL=https://portal.dev.otherjamesbrown.com \
SKIP_WEBSERVER=true \
pnpm playwright test --project=smoke

# Full suite against remote dev
PLAYWRIGHT_BASE_URL=https://portal.dev.otherjamesbrown.com \
SKIP_WEBSERVER=true \
TEST_AUTH_TOKEN="<token>" \
pnpm playwright test
```

### Viewing Reports

```bash
# Open HTML report
pnpm playwright show-report

# View trace for failed test
pnpm playwright show-trace test-results/<test-folder>/trace.zip
```

---

## Test Data Management

### Seeded Test Users

| Email | Password | Role | Organization |
|-------|----------|------|--------------|
| admin@example-acme.com | AcmeAdmin2024!Secure | admin | Acme Ltd |
| member@example-acme.com | AcmeMember2024!Secure | member | Acme Ltd |

### Test API Keys

For integration tests, pre-generate a test API key:

```bash
# Generate test token (run once, store in CI secrets)
admin-cli api-keys create \
  --name "playwright-tests" \
  --scopes "read:all,write:all" \
  --org "acme-org-id"
```

---

## Success Criteria

### Smoke Tests
- [ ] All 6 smoke tests pass consistently (> 95% success rate)
- [ ] Execution time < 2 minutes
- [ ] No flaky tests (rerun 10x without failure)
- [ ] Works against remote dev cluster

### Integration Tests
- [ ] > 90% code coverage of critical paths
- [ ] All browsers pass (Chromium, Firefox, WebKit)
- [ ] Mobile viewport tests pass
- [ ] No flaky tests after 3 retries

### Performance
- [ ] Login page loads in < 3 seconds
- [ ] No visible flickering on health check
- [ ] API responses complete in < 1 second
