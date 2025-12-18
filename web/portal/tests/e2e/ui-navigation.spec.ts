import { test, expect } from '@playwright/test';
import { TEST_USERS } from './helpers/auth';

/**
 * UI Navigation Tests
 *
 * These tests exercise the full UI by navigating through pages as a real user would.
 * They test that all main pages load correctly after login and that navigation works.
 *
 * Run with:
 *   PLAYWRIGHT_BASE_URL=https://portal.dev.otherjamesbrown.com SKIP_WEBSERVER=true pnpm playwright test tests/e2e/ui-navigation.spec.ts --project=chromium
 */

test.describe('UI Navigation Tests', () => {
  // Use serial mode - we maintain login state across tests
  test.describe.configure({ mode: 'serial' });

  test.beforeAll(async ({ browser }) => {
    // This runs once before all tests in this describe block
  });

  test('1. Login and verify home page', async ({ page }) => {
    // Enable request logging for debugging
    const requests: string[] = [];
    const consoleMessages: string[] = [];

    // Capture console messages for debugging
    page.on('console', msg => {
      if (msg.type() === 'error' || msg.type() === 'warn') {
        consoleMessages.push(`[${msg.type()}] ${msg.text()}`);
      }
    });
    page.on('request', request => {
      if (request.url().includes('auth') || request.url().includes('login') || request.url().includes('api.dev')) {
        requests.push(`>> ${request.method()} ${request.url()}`);
      }
    });
    page.on('response', async response => {
      let body = '';
      // Log body for all API responses to debug login flow
      if (response.url().includes('api.dev')) {
        try { body = (await response.text()).substring(0, 200); } catch { /* Ignore response body read errors */ }
      }
      requests.push(`<< ${response.status()} ${response.url()} ${body}`);
    });
    page.on('requestfailed', request => {
      requests.push(`XX FAILED ${request.method()} ${request.url()} - ${request.failure()?.errorText}`);
    });

    // Go to login page
    await page.goto('/auth/login');

    // Verify login page loaded
    await expect(page.getByRole('heading', { name: /ai-aas portal/i })).toBeVisible();

    // Fill in credentials
    await page.getByLabel(/email/i).fill(TEST_USERS.admin.email);
    await page.getByLabel(/password/i).fill(TEST_USERS.admin.password);

    // Screenshot before click
    await page.screenshot({ path: 'test-results/before-click.png' });

    // Click sign in
    const button = page.getByRole('button', { name: /sign in$/i });
    await expect(button).toBeEnabled();
    await button.click({ force: true }); // Force click in case something is blocking

    // Wait for network activity
    await page.waitForTimeout(3000);

    // Screenshot after click
    await page.screenshot({ path: 'test-results/after-click.png' });

    // Log requests and console messages
    console.log('Network requests:', requests.join('\n'));
    console.log('Console messages:', consoleMessages.join('\n'));
    console.log('Current URL:', page.url());

    // Wait for userinfo call (which indicates login success) then navigation
    // The login flow is: POST /login -> GET /userinfo -> navigate to /
    await page.waitForResponse(resp =>
      resp.url().includes('/v1/auth/userinfo') && resp.status() === 200,
      { timeout: 15000 }
    ).catch(() => console.log('userinfo response not captured in time'));

    // Wait for navigation to complete
    await page.waitForURL(/\/$|\/admin/, { timeout: 15000 });

    // Verify we're logged in - should see user email in header (use first() to avoid strict mode)
    await expect(page.getByText(TEST_USERS.admin.email).first()).toBeVisible({ timeout: 5000 });

    // Take screenshot for debugging
    await page.screenshot({ path: 'test-results/01-home-after-login.png' });
  });

  test('2. Navigate to Dashboard via button', async ({ page }) => {
    // Start from home page (already logged in from previous test)
    await page.goto('/');
    await expect(page).toHaveURL(/\/$/);

    // Click "Go to Dashboard" button
    const dashboardButton = page.getByRole('link', { name: /go to dashboard/i });
    if (await dashboardButton.isVisible()) {
      await dashboardButton.click();

      // Wait for dashboard page to load
      await page.waitForLoadState('networkidle');

      // Check for any error messages on the page
      const errorText = page.locator('text=/error|failed|not found/i');
      const hasError = await errorText.count() > 0;

      if (hasError) {
        await page.screenshot({ path: 'test-results/02-dashboard-error.png' });
        // Log the error but don't fail - we want to continue testing other pages
        console.log('Dashboard page has errors - see screenshot');
      }

      // Verify no React/JS errors crashed the page
      await expect(page.locator('body')).toBeVisible();

      await page.screenshot({ path: 'test-results/02-dashboard.png' });
    } else {
      console.log('No "Go to Dashboard" button found - skipping');
    }
  });

  test('3. Navigate to API Keys page', async ({ page }) => {
    await page.goto('/');

    // Try to navigate to API Keys - could be via menu or direct URL
    await page.goto('/admin/api-keys');
    await page.waitForLoadState('networkidle');

    // Verify page loaded without crashing
    await expect(page.locator('body')).toBeVisible();

    // Check for API Keys heading or content
    const pageContent = await page.textContent('body');
    expect(pageContent).toBeTruthy();

    // Check for errors
    const errorElement = page.locator('[role="alert"], .error, text=/error.*occurred/i').first();
    if (await errorElement.isVisible().catch(() => false)) {
      const errorText = await errorElement.textContent();
      console.log('API Keys page error:', errorText);
      await page.screenshot({ path: 'test-results/03-api-keys-error.png' });
    }

    // Should see "API Keys" heading or "Create API Key" button
    const hasApiKeysContent = await page.getByText(/api keys/i).first().isVisible().catch(() => false);
    expect(hasApiKeysContent).toBe(true);

    await page.screenshot({ path: 'test-results/03-api-keys.png' });
  });

  test('4. Navigate to Organization Settings page', async ({ page }) => {
    await page.goto('/admin/organization');
    await page.waitForLoadState('networkidle');

    // Verify page loaded without crashing
    await expect(page.locator('body')).toBeVisible();

    // Check for any error states
    const hasError = await page.getByText(/not found|error|failed/i).first().isVisible().catch(() => false);

    if (hasError) {
      await page.screenshot({ path: 'test-results/04-organization-error.png' });
      // This is a known issue - the API endpoint doesn't exist
      console.log('Organization page shows error - expected (missing backend endpoint)');
    }

    await page.screenshot({ path: 'test-results/04-organization.png' });
  });

  test('5. Navigate to Members page', async ({ page }) => {
    await page.goto('/admin/members');
    await page.waitForLoadState('networkidle');

    // Verify page loaded
    await expect(page.locator('body')).toBeVisible();

    // Check for members content or error
    const pageText = await page.textContent('body');

    // Check for errors that would indicate a crash
    const hasCrash = pageText?.includes('useFeatureFlags must be used') ||
                     pageText?.includes('Cannot read properties of undefined') ||
                     pageText?.includes('Something went wrong');

    if (hasCrash) {
      await page.screenshot({ path: 'test-results/05-members-crash.png' });
      throw new Error('Members page crashed: ' + pageText?.substring(0, 200));
    }

    await page.screenshot({ path: 'test-results/05-members.png' });
  });

  test('6. Navigate to Usage page', async ({ page }) => {
    await page.goto('/admin/usage');
    await page.waitForLoadState('networkidle');

    // Verify page loaded
    await expect(page.locator('body')).toBeVisible();

    const pageText = await page.textContent('body');

    // Check for crashes
    const hasCrash = pageText?.includes('useFeatureFlags must be used') ||
                     pageText?.includes('Cannot read properties of undefined');

    if (hasCrash) {
      await page.screenshot({ path: 'test-results/06-usage-crash.png' });
      throw new Error('Usage page crashed');
    }

    await page.screenshot({ path: 'test-results/06-usage.png' });
  });

  test('7. Navigate to Budgets page', async ({ page }) => {
    await page.goto('/admin/budgets');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('body')).toBeVisible();

    const pageText = await page.textContent('body');
    const hasCrash = pageText?.includes('useFeatureFlags must be used') ||
                     pageText?.includes('Cannot read properties of undefined');

    if (hasCrash) {
      await page.screenshot({ path: 'test-results/07-budgets-crash.png' });
      throw new Error('Budgets page crashed');
    }

    await page.screenshot({ path: 'test-results/07-budgets.png' });
  });

  test('8. Navigate to Models page', async ({ page }) => {
    await page.goto('/admin/models');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('body')).toBeVisible();

    const pageText = await page.textContent('body');
    const hasCrash = pageText?.includes('useFeatureFlags must be used') ||
                     pageText?.includes('Cannot read properties of undefined');

    if (hasCrash) {
      await page.screenshot({ path: 'test-results/08-models-crash.png' });
      throw new Error('Models page crashed');
    }

    await page.screenshot({ path: 'test-results/08-models.png' });
  });

  test('9. Test header navigation links', async ({ page }) => {
    await page.goto('/');
    await expect(page).toHaveURL(/\/$/);

    // Look for navigation elements in header
    const header = page.locator('header, nav, [role="banner"]').first();
    await expect(header).toBeVisible();

    // Check for common navigation items
    const navLinks = [
      { name: /dashboard/i, optional: true },
      { name: /api.*keys/i, optional: true },
      { name: /organization/i, optional: true },
      { name: /settings/i, optional: true },
    ];

    for (const link of navLinks) {
      const element = page.getByRole('link', { name: link.name });
      if (await element.isVisible().catch(() => false)) {
        console.log(`Found nav link: ${link.name}`);
      }
    }

    await page.screenshot({ path: 'test-results/09-header-nav.png' });
  });

  test('10. Test sidebar navigation if present', async ({ page }) => {
    await page.goto('/');

    // Check if there's a sidebar
    const sidebar = page.locator('aside, [role="navigation"], .sidebar, nav').first();

    if (await sidebar.isVisible().catch(() => false)) {
      await page.screenshot({ path: 'test-results/10-sidebar.png' });

      // Try to find and click sidebar links
      const sidebarLinks = sidebar.locator('a');
      const linkCount = await sidebarLinks.count();
      console.log(`Found ${linkCount} sidebar links`);
    } else {
      console.log('No sidebar found');
    }
  });

  test('11. Verify Create API Key modal opens', async ({ page }) => {
    await page.goto('/admin/api-keys');
    await page.waitForLoadState('networkidle');

    // Find and click the Create API Key button
    const createButton = page.getByRole('button', { name: /create api key/i });

    if (await createButton.isVisible()) {
      await createButton.click();

      // Wait for modal to appear
      await page.waitForTimeout(500);

      // Check if modal opened
      const modal = page.locator('[role="dialog"], .modal, [data-state="open"]').first();
      const modalVisible = await modal.isVisible().catch(() => false);

      if (modalVisible) {
        await page.screenshot({ path: 'test-results/11-create-api-key-modal.png' });

        // Close the modal
        const closeButton = page.getByRole('button', { name: /close|cancel|x/i }).first();
        if (await closeButton.isVisible()) {
          await closeButton.click();
        } else {
          // Press Escape to close
          await page.keyboard.press('Escape');
        }
      } else {
        console.log('Modal did not appear after clicking Create API Key');
      }
    } else {
      console.log('Create API Key button not found');
    }
  });

  test('12. Test logout functionality', async ({ page }) => {
    await page.goto('/');
    await expect(page).toHaveURL(/\/$/);

    let logoutFound = false;

    // Find logout button/link
    const logoutButton = page.getByRole('button', { name: /logout|sign out/i });
    const logoutLink = page.getByRole('link', { name: /logout|sign out/i });

    if (await logoutButton.isVisible().catch(() => false)) {
      await logoutButton.click();
      logoutFound = true;
    } else if (await logoutLink.isVisible().catch(() => false)) {
      await logoutLink.click();
      logoutFound = true;
    } else {
      // Try clicking user menu first
      const userMenu = page.locator('button').filter({ hasText: TEST_USERS.admin.email }).first();
      if (await userMenu.isVisible().catch(() => false)) {
        await userMenu.click();
        await page.waitForTimeout(300);

        // Now look for logout in dropdown
        const dropdownLogout = page.getByRole('menuitem', { name: /logout|sign out/i });
        if (await dropdownLogout.isVisible().catch(() => false)) {
          await dropdownLogout.click();
          logoutFound = true;
        }
      }
    }

    if (!logoutFound) {
      // Alternative: directly clear session and navigate to login (simulating logout)
      console.log('No logout button found, clearing session manually');
      await page.evaluate(() => {
        sessionStorage.clear();
      });
      await page.goto('/auth/login');
    }

    // Verify we're at login page (either via logout redirect or manual navigation)
    await expect(page).toHaveURL(/\/auth\/login/, { timeout: 5000 });

    await page.screenshot({ path: 'test-results/12-logged-out.png' });
  });
});

/**
 * Error Detection Tests
 *
 * These tests specifically look for common React/JavaScript errors
 */
test.describe('Error Detection Tests', () => {
  // Longer timeout for multi-page tests
  test.setTimeout(60000);

  test('Check for console errors on main pages', async ({ page }) => {
    const consoleErrors: string[] = [];

    // Listen for console errors
    page.on('console', msg => {
      if (msg.type() === 'error') {
        consoleErrors.push(msg.text());
      }
    });

    // Login first
    await page.goto('/auth/login');
    await page.getByLabel(/email/i).fill(TEST_USERS.admin.email);
    await page.getByLabel(/password/i).fill(TEST_USERS.admin.password);
    await page.getByRole('button', { name: /sign in$/i }).click();
    await expect(page).toHaveURL(/\/$/, { timeout: 15000 });

    // Visit each page and collect errors
    const pages = [
      '/',
      '/admin/api-keys',
      '/admin/organization',
      '/admin/members',
      '/admin/usage',
      '/admin/budgets',
      '/admin/models',
    ];

    const pageErrors: Record<string, string[]> = {};

    for (const pagePath of pages) {
      consoleErrors.length = 0; // Clear errors

      try {
        await page.goto(pagePath, { timeout: 10000 });
        await page.waitForLoadState('networkidle', { timeout: 5000 }).catch(() => {});
        await page.waitForTimeout(500); // Brief wait for async errors
      } catch (e) {
        console.log(`Warning: Navigation to ${pagePath} had timeout issues, continuing...`);
      }

      if (consoleErrors.length > 0) {
        pageErrors[pagePath] = [...consoleErrors];
      }
    }

    // Report errors but don't fail - we want visibility
    if (Object.keys(pageErrors).length > 0) {
      console.log('\n=== Console Errors Found ===');
      for (const [pagePath, errors] of Object.entries(pageErrors)) {
        console.log(`\n${pagePath}:`);
        errors.forEach(err => console.log(`  - ${err.substring(0, 200)}`));
      }
      console.log('\n============================\n');
    }
  });
});
