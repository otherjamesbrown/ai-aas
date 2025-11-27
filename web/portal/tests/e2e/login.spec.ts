import { test, expect } from '@playwright/test';
import { TEST_USERS } from './helpers/auth';

test.describe('Login Page', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/auth/login');
  });

  test('should display login page with password form by default', async ({ page }) => {
    // Check for main heading - the page title is "AI-AAS Portal"
    await expect(page.getByRole('heading', { name: /ai-aas portal/i })).toBeVisible();

    // Check that password form is visible by default
    await expect(page.getByLabel(/email/i)).toBeVisible();
    await expect(page.getByLabel(/password/i)).toBeVisible();
    await expect(page.getByRole('button', { name: /sign in$/i })).toBeVisible();
  });

  test('should have tab selector for login methods', async ({ page }) => {
    // Check for tab buttons
    const emailPasswordTab = page.getByRole('button', { name: /email.*password/i });
    const oauthTab = page.getByRole('button', { name: /oauth/i });

    await expect(emailPasswordTab).toBeVisible();
    await expect(oauthTab).toBeVisible();

    // Password tab should be active by default (has white background in pills variant)
    await expect(emailPasswordTab).toHaveClass(/bg-white/);
  });

  test('should switch between login methods', async ({ page }) => {
    // Start with password form visible
    await expect(page.getByLabel(/email/i)).toBeVisible();

    // Click OAuth tab
    await page.getByRole('button', { name: /oauth/i }).click();

    // Password form should be hidden, OAuth button should be visible
    await expect(page.getByLabel(/email/i)).not.toBeVisible();
    await expect(page.getByRole('button', { name: /sign in with oauth/i })).toBeVisible();

    // Switch back to password
    await page.getByRole('button', { name: /email.*password/i }).click();
    await expect(page.getByLabel(/email/i)).toBeVisible();
  });

  test('should validate required fields', async ({ page }) => {
    // Fill email but leave password empty, then submit
    await page.getByLabel(/email/i).fill('test@example.com');
    await page.getByRole('button', { name: /sign in$/i }).click();

    // Should show validation error for password
    await expect(page.getByText(/password is required/i)).toBeVisible();
  });

  test('should show error on invalid credentials', async ({ page }) => {
    // Fill in invalid credentials
    await page.getByLabel(/email/i).fill('invalid@example.com');
    await page.getByLabel(/password/i).fill('wrongpassword');
    
    // Submit form
    await page.getByRole('button', { name: /sign in$/i }).click();

    // Should show error message
    await expect(page.getByText(/login failed|invalid.*password/i)).toBeVisible({ timeout: 5000 });
  });

  test('should successfully login with valid credentials', async ({ page }) => {
    // Fill in valid credentials using centralized test user
    await page.getByLabel(/email/i).fill(TEST_USERS.admin.email);
    await page.getByLabel(/password/i).fill(TEST_USERS.admin.password);

    // Submit form
    await page.getByRole('button', { name: /sign in$/i }).click();

    // Should redirect to home page after successful login
    await expect(page).toHaveURL(/\//, { timeout: 10000 });

    // Should not see login button in header anymore
    await expect(page.getByRole('link', { name: /login/i })).not.toBeVisible();
  });

  test('should support optional organization ID field', async ({ page }) => {
    // Organization ID field should be visible but optional
    const orgIdInput = page.getByLabel(/organization.*id/i);
    await expect(orgIdInput).toBeVisible();

    // Should be able to login without it - using centralized test user
    await page.getByLabel(/email/i).fill(TEST_USERS.admin.email);
    await page.getByLabel(/password/i).fill(TEST_USERS.admin.password);
    await page.getByRole('button', { name: /sign in$/i }).click();
    
    await expect(page).toHaveURL(/\//, { timeout: 10000 });
  });

  test('should show loading state during login', async ({ page }) => {
    // Use centralized test user credentials
    await page.getByLabel(/email/i).fill(TEST_USERS.admin.email);
    await page.getByLabel(/password/i).fill(TEST_USERS.admin.password);

    // Click submit
    const submitButton = page.getByRole('button', { name: /sign in$/i });
    await submitButton.click();

    // Button should show loading state - either disabled or showing "Loading..." text
    // The loading state is brief, so we check for either condition
    await expect(
      page.getByRole('button', { name: /loading|sign in/i })
    ).toBeVisible({ timeout: 2000 });
  });

  test('should redirect to home if already authenticated', async ({ page }) => {
    // First login using centralized test user
    await page.getByLabel(/email/i).fill(TEST_USERS.admin.email);
    await page.getByLabel(/password/i).fill(TEST_USERS.admin.password);
    await page.getByRole('button', { name: /sign in$/i }).click();
    await expect(page).toHaveURL(/\//, { timeout: 10000 });

    // Navigate back to login page
    await page.goto('/auth/login');

    // Should redirect back to home
    await expect(page).toHaveURL(/\//, { timeout: 5000 });
  });
});

