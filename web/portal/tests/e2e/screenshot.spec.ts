import { test } from '@playwright/test';
import { TEST_USERS } from './helpers/auth';

test('screenshot api keys page', async ({ page }) => {
  // Login first
  await page.goto('/auth/login');
  await page.getByLabel(/email/i).fill(TEST_USERS.admin.email);
  await page.getByLabel(/password/i).fill(TEST_USERS.admin.password);
  await page.getByRole('button', { name: /sign in$/i }).click();
  await page.waitForURL(/\/$/, { timeout: 15000 });
  
  // Take screenshot of home page after login
  await page.screenshot({ path: '/tmp/home-after-login.png', fullPage: true });
  
  // Navigate to API keys
  await page.goto('/admin/api-keys');
  await page.waitForLoadState('networkidle');
  
  // Take screenshot
  await page.screenshot({ path: '/tmp/api-keys-page.png', fullPage: true });
});
