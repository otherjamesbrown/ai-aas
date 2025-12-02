import { Page, Locator, expect } from '@playwright/test';

/**
 * ApiKeysPage - Page Object Model for the API keys management page
 *
 * Methods:
 * - goto(): Navigate to the API keys page
 * - expectLoaded(): Verify the page is loaded correctly
 * - createKey(): Create a new API key
 * - revokeKey(): Revoke an API key
 * - rotateKey(): Rotate an API key
 * - getKeySecret(): Get the secret from the success modal
 */
export class ApiKeysPage {
  readonly page: Page;
  readonly heading: Locator;
  readonly createButton: Locator;
  readonly dataTable: Locator;
  readonly emptyState: Locator;

  constructor(page: Page) {
    this.page = page;
    this.heading = page.getByRole('heading', { name: /^api keys$/i }).first();
    this.createButton = page.getByRole('button', { name: /create api key/i }).first();
    this.dataTable = page.locator('table');
    this.emptyState = page.getByText(/no api keys found/i);
  }

  async goto() {
    await this.page.goto('/admin/api-keys');
  }

  async expectLoaded() {
    await expect(this.heading).toBeVisible({ timeout: 10000 });
    // Either table or empty state should be visible
    const tableOrEmpty = this.dataTable.or(this.emptyState);
    await expect(tableOrEmpty).toBeVisible({ timeout: 5000 });
  }

  async createKey(displayName: string, scopes?: string[]): Promise<string> {
    // Click create button
    await this.createButton.click();

    // Wait for modal
    await expect(this.page.getByRole('heading', { name: /create api key/i })).toBeVisible({ timeout: 5000 });

    // Fill display name
    await this.page.getByLabel(/display name/i).fill(displayName);

    // Add scopes if provided
    if (scopes && scopes.length > 0) {
      const scopeInput = this.page.getByPlaceholder(/enter scope/i);
      for (const scope of scopes) {
        await scopeInput.fill(scope);
        await this.page.getByRole('button', { name: /^add$/i }).click();
      }
    }

    // Submit form
    await this.page.getByRole('button', { name: /create key/i }).click();

    // Wait for success modal
    await expect(
      this.page.getByRole('heading', { name: /api key created/i })
    ).toBeVisible({ timeout: 15000 });

    // Extract secret
    const secret = await this.getKeySecret();

    // Close modal
    await this.page.getByRole('button', { name: /i've saved it|close|done/i }).click();

    return secret;
  }

  async getKeySecret(): Promise<string> {
    const secretElement = this.page.locator('.font-mono').first();
    await expect(secretElement).toBeVisible();
    const secret = await secretElement.textContent();
    if (!secret) {
      throw new Error('Could not extract API key secret');
    }
    return secret.trim();
  }

  async revokeKey(displayName: string) {
    // Find the row with the key name
    const row = this.page.locator('tr').filter({ hasText: displayName });
    await expect(row).toBeVisible();

    // Click revoke button (trash icon)
    const revokeButton = row.getByRole('button', { name: /revoke/i });
    await revokeButton.click();

    // Confirm revocation in modal
    await expect(this.page.getByRole('heading', { name: /revoke api key/i })).toBeVisible();

    // Type confirmation text
    await this.page.getByPlaceholder(/type revoke/i).fill('REVOKE');

    // Click confirm
    await this.page.getByRole('button', { name: /revoke api key/i }).click();

    // Wait for modal to close
    await expect(this.page.getByRole('heading', { name: /revoke api key/i })).not.toBeVisible({ timeout: 5000 });
  }

  async rotateKey(displayName: string): Promise<string> {
    // Find the row with the key name
    const row = this.page.locator('tr').filter({ hasText: displayName });
    await expect(row).toBeVisible();

    // Click rotate button
    const rotateButton = row.getByRole('button', { name: /rotate/i });
    await rotateButton.click();

    // Confirm rotation in modal
    await expect(this.page.getByRole('heading', { name: /rotate api key/i })).toBeVisible();

    // Type confirmation text
    await this.page.getByPlaceholder(/type rotate/i).fill('ROTATE');

    // Click confirm
    await this.page.getByRole('button', { name: /rotate api key/i }).click();

    // Wait for new key modal
    await expect(
      this.page.getByRole('heading', { name: /api key created/i })
    ).toBeVisible({ timeout: 15000 });

    // Get new secret
    const secret = await this.getKeySecret();

    // Close modal
    await this.page.getByRole('button', { name: /i've saved it|close|done/i }).click();

    return secret;
  }

  async getKeyCount(): Promise<number> {
    const rows = await this.page.locator('table tbody tr').count();
    return rows;
  }

  async expectKeyExists(displayName: string) {
    const row = this.page.locator('tr').filter({ hasText: displayName });
    await expect(row).toBeVisible();
  }

  async expectKeyNotExists(displayName: string) {
    const row = this.page.locator('tr').filter({ hasText: displayName });
    await expect(row).not.toBeVisible();
  }
}
