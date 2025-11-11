import { describe, it, expect } from 'vitest';
import { PolicyEngine } from './policy';

describe('PolicyEngine', () => {
  it('loads JSON policy and checks roles', () => {
    const engine = PolicyEngine.fromString(
      JSON.stringify({
        rules: {
          'GET:/secure': ['admin'],
        },
      }),
    );

    expect(engine.isAllowed('GET:/secure', ['admin'])).toBe(true);
    expect(engine.isAllowed('GET:/secure', ['viewer'])).toBe(false);
  });
});

