import { defineConfig, devices } from '@playwright/test';
import path from 'path';

/**
 * See https://playwright.dev/docs/test-configuration.
 */
export default defineConfig({
  testDir: './tests/e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: process.env.CI 
    ? [
        ['html'],
        ['list'],
        ['json', { outputFile: 'test-results/results.json' }],
      ]
    : [
        ['list'],
        ['json', { outputFile: 'test-results/results.json' }],
        ['html', { open: 'never' }], // Generate HTML report but don't open server
      ],
  use: {
    baseURL: process.env.PLAYWRIGHT_BASE_URL || (process.env.VITE_USE_HTTPS === 'false' ? 'http://localhost:5173' : 'https://localhost:5173'),
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
    ignoreHTTPSErrors: true,
    headless: process.env.PLAYWRIGHT_HEADLESS !== 'false', // Default to headless unless explicitly disabled
  },

  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
    {
      name: 'firefox',
      use: { ...devices['Desktop Firefox'] },
    },
    {
      name: 'webkit',
      use: { ...devices['Desktop Safari'] },
    },
    // Mobile viewports
    {
      name: 'Mobile Chrome',
      use: { ...devices['Pixel 5'] },
    },
    {
      name: 'Mobile Safari',
      use: { ...devices['iPhone 12'] },
    },
    // Accessibility testing
    {
      name: 'accessibility',
      use: { ...devices['Desktop Chrome'] },
      testMatch: /.*\.a11y\.spec\.ts/,
    },
  ],

  webServer: process.env.SKIP_WEBSERVER ? undefined : {
    command: 'pnpm dev',
    url: process.env.VITE_USE_HTTPS === 'false' ? 'http://localhost:5173' : 'https://localhost:5173',
    reuseExistingServer: !process.env.CI,
    timeout: 120 * 1000,
  },
});

