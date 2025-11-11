import { describe, it, expect } from 'vitest';
import { loadConfig, toErrorResponse } from './index';

describe('shared ts exports', () => {
  it('exposes config loader', () => {
    expect(typeof loadConfig).toBe('function');
  });

  it('creates standardized error responses', () => {
    const response = toErrorResponse(new Error('boom'));
    expect(response.code).toBe('INTERNAL');
    expect(response.error).toBe('unexpected error occurred');
  });
});

