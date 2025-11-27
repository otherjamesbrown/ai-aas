import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright E2E Test Configuration
 *
 * Projects:
 * - smoke: Fast tests (< 2 min), Chromium only, critical paths
 * - chromium/firefox/webkit: Full test suite, all browsers
 * - accessibility: A11y tests with axe-core
 *
 * Environment Variables:
 * - PLAYWRIGHT_BASE_URL: Target URL (default: https://localhost:5173 or http if VITE_USE_HTTPS=false)
 * - SKIP_WEBSERVER: Set to 'true' for remote testing
 * - PLAYWRIGHT_HEADLESS: Set to 'false' to see browser
 * - VITE_USE_HTTPS: Set to 'false' to use HTTP (default: true/HTTPS)
 * - CI: Enables CI-specific settings (retries, workers)
 *
 * Usage:
 *   # Local smoke tests
 *   pnpm playwright test --project=smoke
 *
 *   # Remote smoke tests
 *   PLAYWRIGHT_BASE_URL=https://portal.172.232.58.222.nip.io SKIP_WEBSERVER=true pnpm playwright test --project=smoke
 *
 *   # Full test suite
 *   pnpm playwright test
 */

// Determine protocol based on VITE_USE_HTTPS env var
const useHttps = process.env.VITE_USE_HTTPS !== 'false';
const baseUrl = process.env.PLAYWRIGHT_BASE_URL || (useHttps ? 'https://localhost:5173' : 'http://localhost:5173');

export default defineConfig({
  testDir: './tests/e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  timeout: 30000,

  reporter: process.env.CI
    ? [
        ['html', { outputFolder: 'playwright-report' }],
        ['list'],
        ['json', { outputFile: 'test-results/results.json' }],
      ]
    : [
        ['list'],
        ['json', { outputFile: 'test-results/results.json' }],
        ['html', { open: 'never' }],
      ],

  use: {
    baseURL: baseUrl,
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
    ignoreHTTPSErrors: true,
    headless: process.env.PLAYWRIGHT_HEADLESS !== 'false',
  },

  projects: [
    // Smoke tests - fast, Chromium only, critical paths
    {
      name: 'smoke',
      testMatch: /smoke\.spec\.ts/,
      use: { ...devices['Desktop Chrome'] },
      timeout: 60000, // 1 minute per test max
    },

    // Full test suite - all browsers
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

    // Mobile viewports
    {
      name: 'mobile-chrome',
      testIgnore: /smoke\.spec\.ts/,
      use: { ...devices['Pixel 5'] },
    },
    {
      name: 'mobile-safari',
      testIgnore: /smoke\.spec\.ts/,
      use: { ...devices['iPhone 12'] },
    },

    // Accessibility testing
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
        url: baseUrl,
        reuseExistingServer: !process.env.CI,
        timeout: 120 * 1000,
      },
});
