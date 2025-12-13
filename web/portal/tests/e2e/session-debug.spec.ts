import { test, expect } from '@playwright/test';
import { TEST_USERS } from './helpers/auth';

/**
 * Debug test to understand session persistence issues
 */
test.describe('Session Debug Tests', () => {
  test('Debug session state across navigation', async ({ page }) => {
    // Login
    await page.goto('/auth/login');
    await page.getByLabel(/email/i).fill(TEST_USERS.admin.email);
    await page.getByLabel(/password/i).fill(TEST_USERS.admin.password);
    await page.getByRole('button', { name: /sign in$/i }).click();
    await expect(page).toHaveURL(/\/$/, { timeout: 15000 });

    // Check session storage after login
    const sessionAfterLogin = await page.evaluate(() => {
      return {
        auth_token: sessionStorage.getItem('auth_token'),
        auth_user: sessionStorage.getItem('auth_user'),
        refresh_token: sessionStorage.getItem('refresh_token'),
      };
    });
    console.log('Session after login:', {
      hasToken: !!sessionAfterLogin.auth_token,
      hasUser: !!sessionAfterLogin.auth_user,
      hasRefresh: !!sessionAfterLogin.refresh_token,
      tokenPreview: sessionAfterLogin.auth_token?.substring(0, 30) + '...',
    });

    // Navigate to API keys page
    await page.goto('/admin/api-keys');
    await page.waitForLoadState('networkidle');

    // Check session storage after navigation
    const sessionAfterNav = await page.evaluate(() => {
      return {
        auth_token: sessionStorage.getItem('auth_token'),
        auth_user: sessionStorage.getItem('auth_user'),
        refresh_token: sessionStorage.getItem('refresh_token'),
      };
    });
    console.log('Session after navigation:', {
      hasToken: !!sessionAfterNav.auth_token,
      hasUser: !!sessionAfterNav.auth_user,
      hasRefresh: !!sessionAfterNav.refresh_token,
      tokenPreview: sessionAfterNav.auth_token?.substring(0, 30) + '...',
    });

    // Check if we're still logged in visually
    const headerText = await page.locator('header').textContent();
    console.log('Header content:', headerText);

    // Check for any errors in the console
    const isLoggedIn = headerText?.includes(TEST_USERS.admin.email) || !headerText?.includes('Login');
    console.log('Is logged in (visual check):', isLoggedIn);

    // Verify tokens are preserved
    expect(sessionAfterNav.auth_token).toBe(sessionAfterLogin.auth_token);
    expect(sessionAfterNav.auth_user).toBe(sessionAfterLogin.auth_user);

    // Take screenshot
    await page.screenshot({ path: 'test-results/debug-session.png' });
  });

  test('Check network requests during navigation', async ({ page }) => {
    const requests: { url: string; status: number }[] = [];

    // Listen for responses
    page.on('response', async (response) => {
      const url = response.url();
      if (url.includes('api.dev.otherjamesbrown.com') || url.includes('/api/') || url.includes('/v1/')) {
        requests.push({
          url: url.replace(/https?:\/\/[^/]+/, ''),
          status: response.status(),
        });
      }
    });

    // Login
    await page.goto('/auth/login');
    await page.getByLabel(/email/i).fill(TEST_USERS.admin.email);
    await page.getByLabel(/password/i).fill(TEST_USERS.admin.password);
    await page.getByRole('button', { name: /sign in$/i }).click();
    await expect(page).toHaveURL(/\/$/, { timeout: 15000 });

    requests.length = 0; // Clear login requests

    // Navigate to API keys
    await page.goto('/admin/api-keys');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(2000); // Wait for any delayed requests

    console.log('\n=== Network requests after navigation to /admin/api-keys ===');
    for (const req of requests) {
      console.log(`${req.status} ${req.url}`);
    }
    console.log('=== End of requests ===\n');

    // Check for 401s
    const unauthorized = requests.filter(r => r.status === 401);
    if (unauthorized.length > 0) {
      console.log('Found 401 unauthorized requests:');
      unauthorized.forEach(r => console.log(`  - ${r.url}`));
    }
  });
});
