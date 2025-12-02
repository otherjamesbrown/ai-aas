import { Page, Locator, expect } from '@playwright/test';

/**
 * LoginPage - Page Object Model for the login page
 *
 * Methods:
 * - goto(): Navigate to the login page
 * - login(): Fill credentials and submit
 * - expectLoaded(): Verify the page is loaded correctly
 * - expectError(): Verify an error message is displayed
 * - switchToOAuth(): Switch to OAuth login tab
 * - switchToPassword(): Switch to password login tab
 */
export class LoginPage {
  readonly page: Page;
  readonly heading: Locator;
  readonly emailInput: Locator;
  readonly passwordInput: Locator;
  readonly orgIdInput: Locator;
  readonly submitButton: Locator;
  readonly oauthTab: Locator;
  readonly passwordTab: Locator;
  readonly oauthButton: Locator;

  constructor(page: Page) {
    this.page = page;
    this.heading = page.getByRole('heading', { name: /ai-aas portal/i });
    this.emailInput = page.getByLabel(/email/i);
    this.passwordInput = page.getByLabel(/password/i);
    this.orgIdInput = page.getByLabel(/organization.*id/i);
    this.submitButton = page.getByRole('button', { name: /sign in$/i });
    this.oauthTab = page.getByRole('button', { name: /oauth/i });
    this.passwordTab = page.getByRole('button', { name: /email.*password/i });
    this.oauthButton = page.getByRole('button', { name: /sign in with oauth/i });
  }

  async goto() {
    await this.page.goto('/auth/login');
  }

  async expectLoaded() {
    await expect(this.heading).toBeVisible();
    await expect(this.emailInput).toBeVisible();
    await expect(this.passwordInput).toBeVisible();
    await expect(this.submitButton).toBeVisible();
  }

  async login(email: string, password: string, orgId?: string) {
    await this.emailInput.fill(email);
    await this.passwordInput.fill(password);
    if (orgId) {
      await this.orgIdInput.fill(orgId);
    }
    await this.submitButton.click();
  }

  async expectError(errorPattern: RegExp = /login failed|invalid|error/i) {
    await expect(this.page.getByText(errorPattern)).toBeVisible({ timeout: 5000 });
  }

  async expectLoginSuccess() {
    await expect(this.page).toHaveURL(/^\/$/, { timeout: 15000 });
  }

  async switchToOAuth() {
    await this.oauthTab.click();
    await expect(this.oauthButton).toBeVisible();
  }

  async switchToPassword() {
    await this.passwordTab.click();
    await expect(this.emailInput).toBeVisible();
  }
}
