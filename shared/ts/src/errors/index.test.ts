import { describe, it, expect } from 'vitest';
import { SharedError, toErrorResponse } from './index';

describe('SharedError', () => {
  it('converts to response', () => {
    const err = new SharedError('BAD_REQUEST', 'bad input', {
      detail: 'missing field',
      requestId: 'req-1',
      actor: { subject: 'user', roles: ['admin'] },
    });
    const response = err.toResponse();
    expect(response.code).toBe('BAD_REQUEST');
    expect(response.detail).toBe('missing field');
    expect(response.actor?.subject).toBe('user');
  });

  it('wraps unknown errors', () => {
    const response = toErrorResponse(new Error('boom'));
    expect(response.code).toBe('INTERNAL');
    expect(response.detail).toBe('boom');
  });
});

