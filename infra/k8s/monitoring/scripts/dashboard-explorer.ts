#!/usr/bin/env npx ts-node
/**
 * Dashboard Explorer - Captures screenshots of Grafana dashboards
 *
 * This script navigates to a Grafana dashboard, captures screenshots,
 * and iterates through all dropdown variables to capture each state.
 *
 * Usage:
 *   npx ts-node dashboard-explorer.ts <dashboard-uid> <environment> [output-dir]
 *
 * Examples:
 *   npx ts-node dashboard-explorer.ts benchmark-system-health staging /tmp/screenshots
 *   npx ts-node dashboard-explorer.ts api-performance-v2 development
 *
 * Output:
 *   - default.png - Dashboard in default state
 *   - <variable>-<option>.png - Dashboard with each dropdown option selected
 *   - manifest.json - List of all captured screenshots with metadata
 *
 * The Claude agent can then use the Read tool on these PNG files to
 * visually analyze the dashboard and identify issues.
 */

import { chromium, Browser, Page } from 'playwright';
import * as fs from 'fs';
import * as path from 'path';

interface ScreenshotManifest {
  dashboardUid: string;
  environment: string;
  capturedAt: string;
  screenshots: {
    filename: string;
    variable?: string;
    option?: string;
    description: string;
  }[];
}

async function exploreDashboard(
  dashboardUid: string,
  environment: string,
  outputDir: string
): Promise<void> {
  const grafanaUrl = `https://grafana.${environment}.otherjamesbrown.com`;
  const grafanaUser = process.env.GRAFANA_USER || 'admin';
  const grafanaPassword = process.env.GRAFANA_PASSWORD || 'admin123';

  // Create output directory
  fs.mkdirSync(outputDir, { recursive: true });

  const manifest: ScreenshotManifest = {
    dashboardUid,
    environment,
    capturedAt: new Date().toISOString(),
    screenshots: [],
  };

  console.log(`Starting dashboard exploration...`);
  console.log(`  Dashboard: ${dashboardUid}`);
  console.log(`  Environment: ${environment}`);
  console.log(`  Output: ${outputDir}`);

  const browser: Browser = await chromium.launch({
    headless: true,
  });

  try {
    const context = await browser.newContext({
      viewport: { width: 1920, height: 1080 },
      ignoreHTTPSErrors: true,
    });
    const page: Page = await context.newPage();

    // Login to Grafana
    console.log(`\nLogging in to Grafana...`);
    await page.goto(`${grafanaUrl}/login`);
    await page.fill('[name="user"]', grafanaUser);
    await page.fill('[name="password"]', grafanaPassword);
    await page.click('button[type="submit"]');
    await page.waitForURL(`${grafanaUrl}/**`, { timeout: 10000 });
    console.log(`  Login successful`);

    // Navigate to dashboard
    console.log(`\nNavigating to dashboard...`);
    await page.goto(`${grafanaUrl}/d/${dashboardUid}?orgId=1`);
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(3000); // Wait for panels to load

    // Capture default state
    const defaultScreenshot = 'default.png';
    await page.screenshot({
      path: path.join(outputDir, defaultScreenshot),
      fullPage: true,
    });
    manifest.screenshots.push({
      filename: defaultScreenshot,
      description: 'Dashboard in default state',
    });
    console.log(`  Captured: ${defaultScreenshot}`);

    // Find all template variable dropdowns
    console.log(`\nSearching for template variables...`);

    // Grafana 11.x uses data-testid for variable pickers
    const variableSelectors = [
      '[data-testid="data-testid Variable value picker"]',
      '[data-testid="variable-value-picker"]',
      '.variable-value-picker',
      '[class*="variable-picker"]',
    ];

    let variableElements: any[] = [];
    for (const selector of variableSelectors) {
      const elements = await page.locator(selector).all();
      if (elements.length > 0) {
        variableElements = elements;
        console.log(`  Found ${elements.length} variables using selector: ${selector}`);
        break;
      }
    }

    if (variableElements.length === 0) {
      console.log(`  No template variables found (or different Grafana version)`);
    }

    // Iterate through each variable
    for (let i = 0; i < variableElements.length; i++) {
      try {
        // Re-find the element (DOM may have changed)
        const dropdown = (await page.locator(variableSelectors[0]).all())[i];
        if (!dropdown) continue;

        // Get variable name from aria-label or nearby label
        const ariaLabel = await dropdown.getAttribute('aria-label');
        const varName = ariaLabel?.replace('Variable value picker for ', '') || `var${i}`;

        console.log(`\n  Processing variable: ${varName}`);

        // Click to open dropdown
        await dropdown.click();
        await page.waitForTimeout(500);

        // Get all options
        const optionElements = await page.locator('[role="option"], [data-testid="select-option"]').all();
        const options: string[] = [];

        for (const opt of optionElements) {
          const text = await opt.textContent();
          if (text) options.push(text.trim());
        }

        console.log(`    Options: ${options.join(', ')}`);

        // Close dropdown first
        await page.keyboard.press('Escape');
        await page.waitForTimeout(300);

        // Iterate through each option
        for (const option of options) {
          try {
            // Click dropdown again
            const currentDropdown = (await page.locator(variableSelectors[0]).all())[i];
            if (!currentDropdown) continue;

            await currentDropdown.click();
            await page.waitForTimeout(500);

            // Select option
            const optionLocator = page.locator(`[role="option"]:has-text("${option}"), [data-testid="select-option"]:has-text("${option}")`).first();
            if (await optionLocator.isVisible()) {
              await optionLocator.click();
              await page.waitForTimeout(2000); // Wait for panels to refresh
              await page.waitForLoadState('networkidle');

              // Capture screenshot
              const safeOption = option.replace(/[^a-z0-9]/gi, '_').toLowerCase();
              const filename = `${varName}-${safeOption}.png`;
              await page.screenshot({
                path: path.join(outputDir, filename),
                fullPage: true,
              });
              manifest.screenshots.push({
                filename,
                variable: varName,
                option,
                description: `Dashboard with ${varName}="${option}"`,
              });
              console.log(`    Captured: ${filename}`);
            }
          } catch (optError) {
            console.log(`    Warning: Could not capture option "${option}": ${optError}`);
          }
        }
      } catch (varError) {
        console.log(`  Warning: Could not process variable ${i}: ${varError}`);
      }
    }

    // Write manifest
    const manifestPath = path.join(outputDir, 'manifest.json');
    fs.writeFileSync(manifestPath, JSON.stringify(manifest, null, 2));
    console.log(`\nManifest written to: ${manifestPath}`);

    console.log(`\n✓ Dashboard exploration complete!`);
    console.log(`  Total screenshots: ${manifest.screenshots.length}`);
    console.log(`  Output directory: ${outputDir}`);
    console.log(`\nTo analyze, use Read tool on the PNG files.`);

  } finally {
    await browser.close();
  }
}

// Main execution
const args = process.argv.slice(2);
if (args.length < 2) {
  console.error('Usage: npx ts-node dashboard-explorer.ts <dashboard-uid> <environment> [output-dir]');
  console.error('');
  console.error('Arguments:');
  console.error('  dashboard-uid  - Grafana dashboard UID (e.g., benchmark-system-health)');
  console.error('  environment    - Target environment (development, staging, production)');
  console.error('  output-dir     - Directory for screenshots (default: /tmp/dashboard-screenshots)');
  console.error('');
  console.error('Environment variables:');
  console.error('  GRAFANA_USER     - Grafana username (default: admin)');
  console.error('  GRAFANA_PASSWORD - Grafana password (default: admin123)');
  process.exit(1);
}

const [dashboardUid, environment, outputDir = '/tmp/dashboard-screenshots'] = args;

exploreDashboard(dashboardUid, environment, outputDir)
  .catch((error) => {
    console.error('Error:', error);
    process.exit(1);
  });
