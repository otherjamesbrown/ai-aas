import { test, expect } from '@playwright/test';

/**
 * E2E test for API key creation and inference endpoint usage
 * 
 * This test:
 * 1. Logs into the UI
 * 2. Navigates to the API keys page
 * 3. Creates an API key
 * 4. Retrieves the API key secret from the modal
 * 5. Uses the API key to call the inference endpoint
 * 6. Verifies the mock response is received
 * 
 * Prerequisites:
 * - Web portal running on http://localhost:5173
 * - user-org-service running on http://localhost:8081
 * - api-router-service running on http://localhost:8080
 * - mock-inference service running on http://localhost:8000
 */
test.describe('API Key Creation and Inference', () => {
  // Use Acme Ltd admin user for tests (see seeded-users.md)
  const testEmail = 'admin@example-acme.com';
  const testPassword = 'AcmeAdmin2024!Secure';
  const apiRouterUrl = process.env.API_ROUTER_URL || 'http://localhost:8080';
  const inferenceEndpoint = `${apiRouterUrl}/v1/inference`;

  // Check that all required services are running before tests
  test.beforeAll(async ({ request }) => {
    const requiredServices = [
      { name: 'user-org-service', url: 'http://localhost:8081/healthz', required: true },
      { name: 'mock-inference', url: 'http://localhost:8000/health', required: true },
    ];
    
    const optionalServices = [
      { name: 'api-router-service', url: 'http://localhost:8080/v1/status/healthz', required: false },
    ];

    const missingRequired: string[] = [];
    const missingOptional: string[] = [];

    // Check required services
    for (const service of requiredServices) {
      try {
        const response = await request.get(service.url, { timeout: 5000 });
        if (!response.ok()) {
          missingRequired.push(`${service.name} (${service.url} returned ${response.status()})`);
        }
      } catch (error) {
        missingRequired.push(`${service.name} (${service.url} - ${error instanceof Error ? error.message : 'connection failed'})`);
      }
    }

    // Check optional services (for inference test)
    for (const service of optionalServices) {
      try {
        const response = await request.get(service.url, { timeout: 5000 });
        if (!response.ok()) {
          missingOptional.push(`${service.name} (${service.url} returned ${response.status()})`);
        }
      } catch (error) {
        missingOptional.push(`${service.name} (${service.url} - ${error instanceof Error ? error.message : 'connection failed'})`);
      }
    }

    if (missingRequired.length > 0) {
      throw new Error(
        `Required services are not running. Please start them before running tests:\n` +
        `Missing services:\n${missingRequired.map(s => `  - ${s}`).join('\n')}\n\n` +
        `To start services:\n` +
        `  1. Start dev stack: make up\n` +
        `  2. Start user-org-service: cd services/user-org-service && make run\n` +
        `  3. Mock inference should start with: make up\n`
      );
    }

    if (missingOptional.length > 0) {
      console.warn(`Optional services not available (inference test will be skipped):\n${missingOptional.map(s => `  - ${s}`).join('\n')}`);
    }
  });

  test.beforeEach(async ({ page, request }) => {
    // Listen for console messages
    page.on('console', msg => {
      console.log('PAGE CONSOLE:', msg.text());
    });

    // Login via Playwright Request API instead of browser to avoid Playwright/Fosite incompatibility
    // See: tmp_md/API_KEY_CREATION_ISSUE_SUMMARY.md for details
    console.log('Authenticating via Request API...');
    const loginResponse = await request.post('http://localhost:8081/v1/auth/login', {
      data: {
        email: testEmail,
        password: testPassword,
      },
    });

    if (!loginResponse.ok()) {
      throw new Error(`Login failed with status ${loginResponse.status()}: ${await loginResponse.text()}`);
    }

    const tokenData = await loginResponse.json() as { access_token: string; refresh_token?: string };
    console.log('Login successful, got token');

    // Navigate to the app first
    await page.goto('/');

    // Inject the auth token into sessionStorage
    await page.evaluate((data) => {
      sessionStorage.setItem('auth_token', data.access_token);
      if (data.refresh_token) {
        sessionStorage.setItem('refresh_token', data.refresh_token);
      }
    }, tokenData);

    console.log('Auth token injected into sessionStorage');
  });

  test('should create API key and use it for inference', async ({ page }) => {
    // Step 1: Navigate to API keys page
    await page.goto('/admin/api-keys');
    // Wait for the main heading (h1) - use first() to avoid matching the empty state h3
    await expect(page.getByRole('heading', { name: /^api keys$/i }).first()).toBeVisible({ timeout: 5000 });

    // Step 2: Click "Create API Key" button (use first() to handle both header and empty state buttons)
    const createButton = page.getByRole('button', { name: /create api key/i }).first();
    await expect(createButton).toBeVisible({ timeout: 5000 });
    console.log('Create API Key button found, clicking...');
    await createButton.click();
    console.log('Create API Key button clicked');

    // Step 3: Fill in the API key creation form
    console.log('Waiting for modal to appear...');
    await page.waitForSelector('h2:has-text("Create API Key")', { timeout: 5000 });
    console.log('Modal appeared');

    const displayName = `test-key-${Date.now()}`;
    console.log('Filling display name:', displayName);
    await page.getByLabel(/display name/i).fill(displayName);
    
    // Optional: Add scopes (the form has a scope input with placeholder)
    const scopeInput = page.getByPlaceholder(/enter scope/i);
    if (await scopeInput.isVisible().catch(() => false)) {
      await scopeInput.fill('inference:read');
      await page.getByRole('button', { name: /^add$/i }).click();
    }

    // Step 4: Submit the form
    console.log('Submitting form...');
    await page.getByRole('button', { name: /create key/i }).click();
    console.log('Form submitted');

    // Wait for either success (modal) or error
    await Promise.race([
      page.waitForSelector('text=API Key Created', { timeout: 15000 }).catch(() => null),
      page.waitForSelector('text=Important', { timeout: 15000 }).catch(() => null),
      page.waitForSelector('text=/error|failed|Error/i', { timeout: 15000 }).catch(() => null),
      page.waitForTimeout(15000),
    ]);

    // Check for error messages first
    const errorMessage = page.locator('text=/error|failed|Error/i').first();
    if (await errorMessage.isVisible().catch(() => false)) {
      const errorText = await errorMessage.textContent();
      // Take a screenshot for debugging
      await page.screenshot({ path: 'test-results/api-key-error.png', fullPage: true });
      throw new Error(`API key creation failed: ${errorText}`);
    }

    // Check if modal is visible
    const modalVisible = await page.getByRole('heading', { name: /api key created/i }).isVisible().catch(() => false) ||
                        await page.locator('text=Important').isVisible().catch(() => false);
    
    if (!modalVisible) {
      // Take a screenshot for debugging
      await page.screenshot({ path: 'test-results/api-key-no-modal.png', fullPage: true });
      // Check what's actually on the page
      const pageContent = await page.content();
      const hasLoading = pageContent.includes('Creating') || pageContent.includes('Loading');
      if (hasLoading) {
        throw new Error('API key creation is still in progress or timed out. Check network requests.');
      }
      throw new Error('API key creation modal did not appear. Check screenshot: test-results/api-key-no-modal.png');
    }

    // Step 5: Wait for the API key modal to appear and extract the secret
    // Wait for the modal heading
    await expect(
      page.getByRole('heading', { name: /api key created/i })
    ).toBeVisible({ timeout: 5000 });

    // Extract the API key secret from the modal
    // The secret is displayed in a div with font-mono class
    const secretElement = page.locator('.font-mono').first();
    await expect(secretElement).toBeVisible();
    
    const apiKeySecret = await secretElement.textContent();
    expect(apiKeySecret).toBeTruthy();
    expect(apiKeySecret!.trim().length).toBeGreaterThan(0);

    // Step 6: Close the modal (click "I've Saved It" button)
    await page.getByRole('button', { name: /i've saved it|close|done/i }).click();

    // Step 7: Use the API key to call the inference endpoint
    // First check if API router service is available
    let apiRouterAvailable = false;
    try {
      const healthCheck = await page.request.get('http://localhost:8080/v1/status/healthz', { timeout: 2000 });
      apiRouterAvailable = healthCheck.ok();
    } catch {
      apiRouterAvailable = false;
    }

    if (!apiRouterAvailable) {
      test.skip();
      return;
    }

    // Generate a UUID for request_id
    const requestId = crypto.randomUUID();
    const inferenceRequest = {
      request_id: requestId,
      model: 'gpt-4o',
      payload: 'Hello, this is a test prompt',
    };

    // Make the inference request using Playwright's request API
    const response = await page.request.post(inferenceEndpoint, {
      headers: {
        'Content-Type': 'application/json',
        'X-API-Key': apiKeySecret!.trim(),
      },
      data: inferenceRequest,
    });

    // Step 8: Verify the response
    expect(response.status()).toBe(200);
    
    const responseData = await response.json();
    expect(responseData).toHaveProperty('request_id', requestId);
    expect(responseData).toHaveProperty('output');
    expect(responseData.output).toHaveProperty('text');
    
    // Verify the mock response contains expected content
    // The mock inference service appends "[mock inference response]" or "[mock chat response]"
    const responseText = responseData.output.text as string;
    expect(responseText).toBeTruthy();
    
    // The mock should return the prompt with some mock suffix
    // Based on the mock service, it returns: prompt + ' [mock inference response]'
    expect(responseText).toContain('Hello, this is a test prompt');
    
    // Verify usage information is present
    expect(responseData).toHaveProperty('usage');
    expect(responseData.usage).toHaveProperty('tokens_input');
    expect(responseData.usage).toHaveProperty('tokens_output');
  });

  test('should handle API key creation with empty state', async ({ page }) => {
    // Navigate to API keys page
    await page.goto('/admin/api-keys');
    
    // If there are no keys, we should see the empty state
    const hasKeys = await page.locator('table tbody tr').count() > 0;
    
    if (!hasKeys) {
      // Check for empty state heading (h3)
      await expect(page.getByRole('heading', { name: /^no api keys$/i })).toBeVisible();
      
      // Click create button from empty state
      await page.getByRole('button', { name: /create api key/i }).first().click();
      
      // Fill and submit form
      const displayName = `test-key-empty-${Date.now()}`;
      await page.getByLabel(/display name/i).fill(displayName);
      await page.getByRole('button', { name: /create key/i }).click();
      
      // Verify modal appears
      await expect(
        page.getByRole('heading', { name: /api key created/i }).or(page.locator('text=Important'))
      ).toBeVisible({ timeout: 15000 });
    }
  });
});

