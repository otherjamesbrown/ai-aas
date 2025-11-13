# Testing Guide

This directory contains all tests for the web portal, organized by test type.

## Test Structure

```
tests/
├── unit/          # Unit tests (Vitest + React Testing Library)
├── integration/   # Integration tests (Vitest)
├── e2e/          # End-to-end tests (Playwright)
└── contract/     # Contract tests (Pact)
```

## Running Tests

### Unit Tests
```bash
# Run all unit tests
pnpm test

# Watch mode
pnpm test:watch

# With coverage
pnpm test:coverage

# UI mode
pnpm test:ui
```

### E2E Tests
```bash
# Run all E2E tests
pnpm test:e2e

# Interactive UI mode
pnpm test:e2e:ui

# Debug mode
pnpm test:e2e:debug

# Accessibility tests only
pnpm test:a11y
```

### All Tests
```bash
pnpm test:all
```

## Writing Tests

### Unit Tests

Use Vitest + React Testing Library for component and hook testing.

```typescript
import { describe, it, expect } from 'vitest';
import { render, screen } from '@/test/test-utils';
import { MyComponent } from '@/components/MyComponent';

describe('MyComponent', () => {
  it('renders correctly', () => {
    render(<MyComponent />);
    expect(screen.getByText('Hello')).toBeInTheDocument();
  });
});
```

### E2E Tests

Use Playwright for end-to-end testing.

```typescript
import { test, expect } from '@playwright/test';

test('user can login', async ({ page }) => {
  await page.goto('/auth/login');
  // ... test steps
});
```

### Accessibility Tests

Use axe-core with Playwright for accessibility testing.

```typescript
import { test, expect } from './axe-setup';

test('page has no accessibility violations', async ({ page, makeAxeBuilder }) => {
  await page.goto('/');
  const results = await makeAxeBuilder().analyze();
  expect(results.violations).toEqual([]);
});
```

## Test Utilities

- `@/test/test-utils` - Custom render function with all providers
- `@/test/setup.ts` - Global test setup and mocks

## Best Practices

1. **Test user behavior, not implementation details**
2. **Use data-testid sparingly** - prefer accessible queries
3. **Keep tests isolated** - each test should be independent
4. **Test accessibility** - use axe-core for automated checks
5. **Mock external dependencies** - APIs, timers, etc.
6. **Use descriptive test names** - what is being tested and expected outcome

## Coverage Goals

- Statements: 70%
- Branches: 70%
- Functions: 70%
- Lines: 70%

## CI Integration

Tests run automatically in CI:
- Unit tests on every commit
- E2E tests on PRs
- Accessibility tests on PRs
- Coverage reports generated

