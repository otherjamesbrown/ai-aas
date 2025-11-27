import { Page, BrowserContext } from '@playwright/test';

/**
 * Test user credentials
 * These are seeded users in the development environment.
 * Password can be overridden via TEST_USER_PASSWORD env var for deployed environments
 * where the seeded password may differ.
 */
const testPassword = process.env.TEST_USER_PASSWORD || 'TestPass123';

export const TEST_USERS = {
  admin: {
    email: 'admin@example-acme.com',
    password: testPassword,
    role: 'admin',
  },
  member: {
    email: 'member@example-acme.com',
    password: testPassword,
    role: 'member',
  },
} as const;

/**
 * Login via UI (for testing the login flow itself)
 */
export async function loginViaUI(
  page: Page,
  email: string = TEST_USERS.admin.email,
  password: string = TEST_USERS.admin.password
): Promise<void> {
  await page.goto('/auth/login');

  // Wait for login form to be ready
  await page.waitForSelector('input[type="email"], input[name="email"]', { state: 'visible' });

  // Fill credentials
  await page.getByLabel(/email/i).fill(email);
  await page.getByLabel(/password/i).fill(password);

  // Submit
  await page.getByRole('button', { name: /sign in$/i }).click();

  // Wait for navigation to complete
  await page.waitForURL(/^\/$/, { timeout: 15000 });
}

/**
 * Login via API and inject token into browser context
 * Faster than UI login, use for tests that don't test login itself
 */
export async function loginViaAPI(
  context: BrowserContext,
  baseUrl: string,
  email: string = TEST_USERS.admin.email,
  password: string = TEST_USERS.admin.password
): Promise<void> {
  // Call login API directly
  const response = await context.request.post(`${baseUrl}/v1/auth/login`, {
    data: {
      email,
      password,
    },
  });

  if (!response.ok()) {
    throw new Error(`Login API failed: ${response.status()} ${response.statusText()}`);
  }

  const data = await response.json();

  if (!data.access_token) {
    throw new Error('No access token in login response');
  }

  // Inject token into session storage via script
  await context.addInitScript(
    ({ token, user }) => {
      sessionStorage.setItem('auth_token', token);
      sessionStorage.setItem('auth_user', JSON.stringify(user));
    },
    {
      token: data.access_token,
      user: {
        id: 'test-user',
        email,
        name: email.split('@')[0],
        roles: ['admin'],
        scopes: [],
      },
    }
  );
}

/**
 * Check if user is logged in
 */
export async function isLoggedIn(page: Page): Promise<boolean> {
  const token = await page.evaluate(() => sessionStorage.getItem('auth_token'));
  return !!token;
}

/**
 * Logout user
 */
export async function logout(page: Page): Promise<void> {
  await page.evaluate(() => {
    sessionStorage.removeItem('auth_token');
    sessionStorage.removeItem('refresh_token');
    sessionStorage.removeItem('auth_user');
  });
  await page.goto('/auth/login');
}
