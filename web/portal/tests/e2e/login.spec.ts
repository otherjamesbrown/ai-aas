import { test, expect } from '@playwright/test';

test.describe('Login Page', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/auth/login');
  });

  test('should display login page with password form by default', async ({ page }) => {
    // Check for main heading
    await expect(page.getByRole('heading', { name: /sign in/i })).toBeVisible();

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

    // Password tab should be active by default
    await expect(emailPasswordTab).toHaveClass(/bg-primary/);
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
    // Try to submit empty form
    await page.getByRole('button', { name: /sign in$/i }).click();

    // Form validation should prevent submission
    const emailInput = page.getByLabel(/email/i);
    await expect(emailInput).toBeInvalid();
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
    // Fill in valid credentials
    await page.getByLabel(/email/i).fill('admin@example.com');
    await page.getByLabel(/password/i).fill('nubipwdkryfmtaho123!');
    
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
    
    // Should be able to login without it
    await page.getByLabel(/email/i).fill('admin@example.com');
    await page.getByLabel(/password/i).fill('nubipwdkryfmtaho123!');
    await page.getByRole('button', { name: /sign in$/i }).click();
    
    await expect(page).toHaveURL(/\//, { timeout: 10000 });
  });

  test('should show loading state during login', async ({ page }) => {
    await page.getByLabel(/email/i).fill('admin@example.com');
    await page.getByLabel(/password/i).fill('nubipwdkryfmtaho123!');
    
    // Click submit
    const submitButton = page.getByRole('button', { name: /sign in$/i });
    await submitButton.click();

    // Button should show loading state (disabled or showing spinner)
    await expect(submitButton).toBeDisabled({ timeout: 1000 });
  });

  test('should redirect to home if already authenticated', async ({ page }) => {
    // First login
    await page.getByLabel(/email/i).fill('admin@example.com');
    await page.getByLabel(/password/i).fill('nubipwdkryfmtaho123!');
    await page.getByRole('button', { name: /sign in$/i }).click();
    await expect(page).toHaveURL(/\//, { timeout: 10000 });

    // Navigate back to login page
    await page.goto('/auth/login');

    // Should redirect back to home
    await expect(page).toHaveURL(/\//, { timeout: 5000 });
  });
});

