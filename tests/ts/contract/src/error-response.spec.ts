import { describe, it, expect } from 'vitest';
import { SharedError, toErrorResponse } from '@ai-aas/shared';

describe('error response contract', () => {
  it('matches schema expectations', () => {
    const err = new SharedError('EXAMPLE', 'example failure', {
      detail: 'missing widget',
      requestId: 'req-123',
      traceId: 'trace-456',
      actor: { subject: 'alice', roles: ['admin'] },
    });

    const payload = toErrorResponse(err);

    const requiredFields = ['error', 'code', 'request_id', 'trace_id', 'timestamp'];
    for (const field of requiredFields) {
      expect(typeof (payload as any)[field]).toBe('string');
    }

    expect(payload.code).toBe('EXAMPLE');
    expect(payload.actor?.subject).toBe('alice');
    expect(Array.isArray(payload.actor?.roles)).toBe(true);
    expect(payload.actor?.roles?.includes('admin')).toBe(true);

    // Timestamp should be RFC3339 compatible.
    expect(() => new Date(payload.timestamp).toISOString()).not.toThrow();
  });
});

