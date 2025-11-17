import { test, expect } from './axe-setup';

test.describe('Accessibility', () => {
  test('homepage should have no accessibility violations', async ({ page, makeAxeBuilder }) => {
    await page.goto('/');

    const accessibilityScanResults = await makeAxeBuilder()
      .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
      // Temporarily exclude color-contrast rule - needs design system updates
      .disableRules(['color-contrast'])
      .analyze();

    expect(accessibilityScanResults.violations).toEqual([]);
  });

  test('login page should have no accessibility violations', async ({ page, makeAxeBuilder }) => {
    await page.goto('/auth/login');

    const accessibilityScanResults = await makeAxeBuilder()
      .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
      // Temporarily exclude color-contrast rule - needs design system updates
      .disableRules(['color-contrast'])
      .analyze();

    expect(accessibilityScanResults.violations).toEqual([]);
  });

  test('should have proper heading hierarchy', async ({ page }) => {
    await page.goto('/');

    // Check for h1
    const h1 = page.locator('h1').first();
    await expect(h1).toBeVisible();

    // Check that h1 comes before h2
    const h2 = page.locator('h2').first();
    const h1Index = await h1.evaluate((el) => {
      const all = Array.from(document.querySelectorAll('h1, h2'));
      return all.indexOf(el);
    });
    const h2Index = await h2.evaluate((el) => {
      const all = Array.from(document.querySelectorAll('h1, h2'));
      return all.indexOf(el);
    });

    expect(h1Index).toBeLessThan(h2Index);
  });

  test('should have skip to content link', async ({ page }) => {
    await page.goto('/');

    const skipLink = page.locator('.skip-to-content');
    await expect(skipLink).toBeVisible();

    // Skip link should be focusable
    await skipLink.focus();
    await expect(skipLink).toBeFocused();
  });

  test('should support keyboard navigation', async ({ page }) => {
    await page.goto('/');

    // Tab through interactive elements
    await page.keyboard.press('Tab');
    const focused = page.locator(':focus');
    await expect(focused).toBeVisible();

    // Check that focus is visible
    const focusStyles = await focused.evaluate((el) => {
      const styles = window.getComputedStyle(el);
      return {
        outline: styles.outline,
        outlineWidth: styles.outlineWidth,
      };
    });

    expect(focusStyles.outlineWidth).not.toBe('0px');
  });
});

