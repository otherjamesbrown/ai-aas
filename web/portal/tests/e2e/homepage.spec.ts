import { test, expect } from '@playwright/test';

test.describe('Homepage', () => {
  test('should load and display homepage', async ({ page }) => {
    await page.goto('/');

    // Check for main heading
    await expect(page.getByRole('heading', { name: /welcome to ai-aas portal/i })).toBeVisible();

    // Check for feature sections
    await expect(page.getByText(/organization management/i)).toBeVisible();
    await expect(page.getByText(/budget controls/i)).toBeVisible();
    await expect(page.getByText(/usage insights/i)).toBeVisible();
  });

  test('should show login button when not authenticated', async ({ page }) => {
    await page.goto('/');

    const loginLink = page.getByRole('link', { name: /get started/i });
    await expect(loginLink).toBeVisible();
    await expect(loginLink).toHaveAttribute('href', '/auth/login');
  });

  test('should navigate to login page', async ({ page }) => {
    await page.goto('/');
    await page.getByRole('link', { name: /get started/i }).click();

    await expect(page).toHaveURL(/\/auth\/login/);
  });

  test('should be responsive on mobile', async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 667 });
    await page.goto('/');

    // Check that content is still visible and readable
    await expect(page.getByRole('heading', { name: /welcome/i })).toBeVisible();
  });
});

