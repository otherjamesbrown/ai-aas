import { describe, it, expect, vi } from 'vitest';
import { PolicyEngine } from './policy';
import { createAuthMiddleware, ReplyLike } from './middleware';
import { setAuditRecorder, type AuditEvent } from './audit';

const engine = PolicyEngine.fromString(
  JSON.stringify({
    rules: {
      'GET:/secure': ['admin'],
    },
  }),
);

describe('auth middleware', () => {
  it('allows authorized request', async () => {
    const events: AuditEvent[] = [];
    setAuditRecorder((event) => events.push(event));
    const middleware = createAuthMiddleware(engine);
    const statusCalls: number[] = [];
    const reply: ReplyLike = {
      status: (code: number) => {
        statusCalls.push(code);
        return { send: () => {} };
      },
    };

    await middleware(
      {
        method: 'GET',
        url: '/secure',
        headers: { 'x-actor-roles': 'admin', 'x-request-id': 'req-1' },
      },
      reply,
    );
    expect(statusCalls.length).toBe(0);
    expect(events).toEqual([
      expect.objectContaining({ allowed: true }),
    ]);
  });

  it('denies unauthorized request', async () => {
    const events: AuditEvent[] = [];
    setAuditRecorder((event) => events.push(event));
    const middleware = createAuthMiddleware(engine);
    const statusCalls: number[] = [];
    const send = vi.fn();
    const reply: ReplyLike = {
      status: (code: number) => {
        statusCalls.push(code);
        return { send };
      },
    };

    await middleware(
      {
        method: 'GET',
        url: '/secure',
        headers: { 'x-actor-roles': 'viewer' },
      },
      reply,
    );
    expect(statusCalls).toEqual([403]);
    expect(send).toHaveBeenCalled();
    expect(events).toEqual([
      expect.objectContaining({ allowed: false }),
    ]);
  });
});

