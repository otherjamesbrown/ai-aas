import { Page, Locator, expect } from '@playwright/test';

/**
 * HomePage - Page Object Model for the home/landing page
 *
 * Methods:
 * - goto(): Navigate to the home page
 * - expectLoaded(): Verify the page is loaded correctly
 * - expectAuthenticated(): Verify user is authenticated
 * - navigateTo*(): Navigate to various sections
 */
export class HomePage {
  readonly page: Page;
  readonly heroHeading: Locator;
  readonly getStartedButton: Locator;
  readonly dashboardButton: Locator;
  readonly usageButton: Locator;
  readonly featuresSection: Locator;
  readonly quickLinksSection: Locator;

  constructor(page: Page) {
    this.page = page;
    this.heroHeading = page.getByRole('heading', { name: /ai-aas portal/i });
    this.getStartedButton = page.getByRole('button', { name: /get started/i });
    this.dashboardButton = page.getByRole('button', { name: /go to dashboard/i });
    this.usageButton = page.getByRole('button', { name: /view usage/i });
    this.featuresSection = page.getByRole('heading', { name: /features/i });
    this.quickLinksSection = page.getByRole('heading', { name: /quick links/i });
  }

  async goto() {
    await this.page.goto('/');
  }

  async expectLoaded() {
    await expect(this.heroHeading).toBeVisible();
  }

  async expectAuthenticated() {
    // When authenticated, we should see dashboard/usage buttons instead of get started
    await expect(this.dashboardButton).toBeVisible({ timeout: 5000 });
  }

  async expectUnauthenticated() {
    // When not authenticated, we should see get started button
    await expect(this.getStartedButton).toBeVisible({ timeout: 5000 });
  }

  async navigateToDashboard() {
    await this.dashboardButton.click();
    await expect(this.page).toHaveURL(/\/admin/, { timeout: 5000 });
  }

  async navigateToUsage() {
    await this.usageButton.click();
    await expect(this.page).toHaveURL(/\/usage/, { timeout: 5000 });
  }

  async navigateToOrganizationSettings() {
    await this.page.getByRole('link', { name: /organization settings/i }).click();
    await expect(this.page).toHaveURL(/\/admin\/organization/, { timeout: 5000 });
  }

  async navigateToMembers() {
    await this.page.getByRole('link', { name: /member management/i }).click();
    await expect(this.page).toHaveURL(/\/admin\/members/, { timeout: 5000 });
  }

  async navigateToBudgets() {
    await this.page.getByRole('link', { name: /budget controls/i }).click();
    await expect(this.page).toHaveURL(/\/admin\/budgets/, { timeout: 5000 });
  }

  async navigateToUsageDashboard() {
    await this.page.getByRole('link', { name: /usage dashboard/i }).click();
    await expect(this.page).toHaveURL(/\/usage/, { timeout: 5000 });
  }
}
