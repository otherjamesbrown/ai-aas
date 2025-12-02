import { test, expect } from '@playwright/test';
import { TEST_USERS } from './helpers/auth';

/**
 * Data Integrity Tests
 *
 * These tests validate that API responses contain the expected data structure
 * and that the UI properly displays data fields. This catches issues like:
 * - Missing required fields from API responses
 * - Null/undefined nested objects that crash the UI
 * - Type mismatches between API and UI expectations
 *
 * Run with:
 *   SKIP_WEBSERVER=true pnpm playwright test tests/e2e/data-integrity.spec.ts --project=chromium
 */

test.describe('Data Integrity Tests', () => {
  test.beforeEach(async ({ page }) => {
    // Login before each test
    await page.goto('/auth/login');
    await page.getByLabel(/email/i).fill(TEST_USERS.admin.email);
    await page.getByLabel(/password/i).fill(TEST_USERS.admin.password);
    await page.getByRole('button', { name: /sign in$/i }).click();
    await expect(page).toHaveURL(/\/$|\/admin/, { timeout: 15000 });
  });

  test('Organization Settings page - validate form fields are populated', async ({ page }) => {
    // Intercept the organization API call to capture the response
    let orgApiResponse: any = null;

    page.on('response', async (response) => {
      if (response.url().includes('/organizations/me') && response.status() === 200) {
        try {
          orgApiResponse = await response.json();
        } catch {
          // Response might not be JSON
        }
      }
    });

    await page.goto('/admin/organization');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(1000); // Wait for data to load

    // Take screenshot for debugging
    await page.screenshot({ path: 'test-results/org-settings-data.png' });

    // Check that the page didn't crash
    const pageContent = await page.textContent('body');
    expect(pageContent).not.toContain('Cannot read properties of undefined');
    expect(pageContent).not.toContain('Something went wrong');

    // Verify key form fields exist and are not showing loading state
    const loadingSpinner = page.locator('[role="status"], .loading, .spinner').first();
    const isStillLoading = await loadingSpinner.isVisible().catch(() => false);

    if (isStillLoading) {
      // Wait a bit more for data to load
      await page.waitForTimeout(2000);
    }

    // Check Organization Name field
    const orgNameInput = page.locator('input[name="name"]');
    if (await orgNameInput.isVisible()) {
      const orgNameValue = await orgNameInput.inputValue();
      console.log('Organization Name value:', orgNameValue);
      // Should have some value (not empty string from failed API)
      expect(orgNameValue.length).toBeGreaterThan(0);
    }

    // Check Billing Contact fields
    const billingNameInput = page.locator('input[name="billing_name"]');
    if (await billingNameInput.isVisible()) {
      // The field should exist and be interactable (even if empty)
      await expect(billingNameInput).toBeEnabled();
    }

    const billingEmailInput = page.locator('input[name="billing_email"]');
    if (await billingEmailInput.isVisible()) {
      await expect(billingEmailInput).toBeEnabled();
    }

    // Check Address fields
    const countryInput = page.locator('input[name="country"]');
    if (await countryInput.isVisible()) {
      await expect(countryInput).toBeEnabled();
    }

    // Validate the API response structure if captured
    if (orgApiResponse) {
      console.log('Organization API Response:', JSON.stringify(orgApiResponse, null, 2));

      // Validate required fields exist
      expect(orgApiResponse).toHaveProperty('orgId');
      expect(orgApiResponse).toHaveProperty('name');

      // Note: These fields might be optional, but we log their presence
      console.log('Has billing_contact:', !!orgApiResponse.billing_contact);
      console.log('Has address:', !!orgApiResponse.address);
    }
  });

  test('API Keys page - validate API response and table data', async ({ page }) => {
    let apiKeysResponse: any = null;

    page.on('response', async (response) => {
      if (response.url().includes('/api-keys') && response.status() === 200) {
        try {
          apiKeysResponse = await response.json();
        } catch {
          // Response might not be JSON
        }
      }
    });

    await page.goto('/admin/api-keys');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(1000);

    // Check page didn't crash
    const pageContent = await page.textContent('body');
    expect(pageContent).not.toContain('Cannot read properties of undefined');
    expect(pageContent).not.toContain('Something went wrong');

    // Validate API response if captured
    if (apiKeysResponse) {
      console.log('API Keys response type:', typeof apiKeysResponse);

      if (Array.isArray(apiKeysResponse)) {
        console.log('API Keys count:', apiKeysResponse.length);

        // Validate structure of first item if present
        if (apiKeysResponse.length > 0) {
          const firstKey = apiKeysResponse[0];
          console.log('First API key structure:', Object.keys(firstKey));

          // Common expected fields
          expect(firstKey).toHaveProperty('id');
          expect(firstKey).toHaveProperty('name');
        }
      } else if (apiKeysResponse.keys) {
        // Might be wrapped in an object
        console.log('API Keys count:', apiKeysResponse.keys.length);
      }
    }

    await page.screenshot({ path: 'test-results/api-keys-data.png' });
  });

  test('Members page - validate user list data', async ({ page }) => {
    let membersResponse: any = null;

    page.on('response', async (response) => {
      if ((response.url().includes('/members') || response.url().includes('/users'))
          && response.status() === 200) {
        try {
          membersResponse = await response.json();
        } catch {
          // Response might not be JSON
        }
      }
    });

    await page.goto('/admin/members');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(1000);

    // Check page didn't crash
    const pageContent = await page.textContent('body');
    expect(pageContent).not.toContain('Cannot read properties of undefined');
    expect(pageContent).not.toContain('Something went wrong');

    // Validate API response if captured
    if (membersResponse) {
      console.log('Members response:', JSON.stringify(membersResponse, null, 2).substring(0, 500));

      // Check for expected structure
      if (Array.isArray(membersResponse)) {
        console.log('Members count:', membersResponse.length);

        if (membersResponse.length > 0) {
          const firstMember = membersResponse[0];
          console.log('First member keys:', Object.keys(firstMember));

          // Validate expected fields
          expect(firstMember).toHaveProperty('email');
        }
      } else if (membersResponse.users || membersResponse.members) {
        const list = membersResponse.users || membersResponse.members;
        console.log('Members count:', list.length);
      }
    }

    await page.screenshot({ path: 'test-results/members-data.png' });
  });

  test('User info API - validate auth response structure', async ({ page }) => {
    let userInfoResponse: any = null;

    page.on('response', async (response) => {
      if (response.url().includes('/auth/userinfo') && response.status() === 200) {
        try {
          userInfoResponse = await response.json();
        } catch {
          // Response might not be JSON
        }
      }
    });

    // Navigate to trigger userinfo call
    await page.goto('/admin/api-keys');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(1000);

    // Validate userinfo response
    if (userInfoResponse) {
      console.log('UserInfo response:', JSON.stringify(userInfoResponse, null, 2));

      // Required fields for auth context
      expect(userInfoResponse).toHaveProperty('sub');
      expect(userInfoResponse).toHaveProperty('email');
      expect(userInfoResponse).toHaveProperty('organization_id');

      // Check for roles (might be optional but UI depends on it)
      console.log('Has roles:', !!userInfoResponse.roles);
      console.log('Roles value:', userInfoResponse.roles);

      // Validate roles is an array if present
      if (userInfoResponse.roles !== undefined) {
        expect(Array.isArray(userInfoResponse.roles)).toBe(true);
      }
    } else {
      console.log('Warning: UserInfo response was not captured');
    }
  });

  test('Feature flags API - validate response structure', async ({ page }) => {
    let featureFlagsResponse: any = null;

    page.on('response', async (response) => {
      if (response.url().includes('/feature-flags') && response.status() === 200) {
        try {
          featureFlagsResponse = await response.json();
        } catch {
          // Response might not be JSON
        }
      }
    });

    await page.goto('/admin/api-keys');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(1000);

    // Validate feature flags response
    if (featureFlagsResponse) {
      console.log('Feature flags response:', JSON.stringify(featureFlagsResponse, null, 2));

      // Should be an object with boolean values
      expect(typeof featureFlagsResponse).toBe('object');

      // Common expected flags
      const expectedFlags = ['api-key-management', 'budget-controls', 'dark-mode'];
      for (const flag of expectedFlags) {
        if (flag in featureFlagsResponse) {
          expect(typeof featureFlagsResponse[flag]).toBe('boolean');
        }
      }
    }
  });
});

/**
 * Critical Data Validation Tests
 *
 * These tests FAIL if critical data is missing - they catch production bugs
 */
test.describe('Critical Data Validation', () => {
  // Longer timeout for these tests since they visit multiple pages
  test.setTimeout(60000);

  test.beforeEach(async ({ page }) => {
    await page.goto('/auth/login');
    await page.getByLabel(/email/i).fill(TEST_USERS.admin.email);
    await page.getByLabel(/password/i).fill(TEST_USERS.admin.password);
    await page.getByRole('button', { name: /sign in$/i }).click();
    await expect(page).toHaveURL(/\/$|\/admin/, { timeout: 15000 });
  });

  test('Organization page should not crash on missing nested fields', async ({ page }) => {
    // This test specifically catches the bug where profile.billing_contact.name crashes
    const consoleErrors: string[] = [];

    page.on('console', msg => {
      if (msg.type() === 'error') {
        consoleErrors.push(msg.text());
      }
    });

    page.on('pageerror', error => {
      consoleErrors.push(`PageError: ${error.message}`);
    });

    await page.goto('/admin/organization');
    await page.waitForLoadState('networkidle').catch(() => {});
    await page.waitForTimeout(2000);

    // Filter for critical JS errors
    const criticalErrors = consoleErrors.filter(err =>
      err.includes('Cannot read properties of undefined') ||
      err.includes('TypeError') ||
      err.includes('is not defined')
    );

    if (criticalErrors.length > 0) {
      console.log('Critical errors found:', criticalErrors);
      await page.screenshot({ path: 'test-results/org-page-critical-errors.png' });
    }

    // FAIL if we have JavaScript runtime errors
    expect(criticalErrors, 'Page should not have JavaScript runtime errors').toHaveLength(0);
  });

  test('All admin pages should render without JavaScript errors', async ({ page }) => {
    const pageErrors: Record<string, string[]> = {};

    const pages = [
      '/admin/api-keys',
      '/admin/organization',
      '/admin/members',
      '/admin/usage',
      '/admin/budgets',
      '/admin/models',
    ];

    // Set up error listeners once before visiting pages
    const allErrors: { page: string; error: string }[] = [];
    let currentPage = '';

    page.on('console', msg => {
      if (msg.type() === 'error' &&
          (msg.text().includes('TypeError') ||
           msg.text().includes('Cannot read properties'))) {
        allErrors.push({ page: currentPage, error: msg.text() });
      }
    });

    page.on('pageerror', error => {
      allErrors.push({ page: currentPage, error: `PageError: ${error.message}` });
    });

    for (const pagePath of pages) {
      currentPage = pagePath;

      try {
        await page.goto(pagePath, { timeout: 10000 });
        await page.waitForLoadState('networkidle', { timeout: 5000 }).catch(() => {});
        await page.waitForTimeout(500);
      } catch (e) {
        console.log(`Warning: Navigation to ${pagePath} had issues, continuing...`);
      }
    }

    // Group errors by page
    for (const err of allErrors) {
      if (!pageErrors[err.page]) {
        pageErrors[err.page] = [];
      }
      pageErrors[err.page].push(err.error);
    }

    // Report and fail if any page has errors
    if (Object.keys(pageErrors).length > 0) {
      console.log('Pages with JavaScript errors:', JSON.stringify(pageErrors, null, 2));
    }

    expect(Object.keys(pageErrors), 'No pages should have JavaScript runtime errors').toHaveLength(0);
  });
});
