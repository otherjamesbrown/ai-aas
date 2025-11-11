import { describe, it, expect, vi, afterEach } from 'vitest';
import { buildAuditEvent, setAuditRecorder } from '@ai-aas/shared/auth/audit';
import {
  createAuthMiddleware,
  headerExtractor,
  type PolicyEngine,
  type ReplyLike,
  type ReplySender,
  type RequestLike,
} from '@ai-aas/shared/auth/middleware';

afterEach(() => {
  setAuditRecorder(() => {});
  vi.restoreAllMocks();
});

describe('auth utilities contract', () => {
  it('builds immutable audit events', () => {
    const actor = { subject: 'alice', roles: ['admin'] };
    const event = buildAuditEvent('GET:/secure', actor, true);
    expect(event.action).toBe('GET:/secure');
    expect(event.subject).toBe('alice');
    expect(event.roles).toEqual(['admin']);
    actor.roles.push('viewer');
    expect(event.roles).toEqual(['admin']);
  });

  it('records audit events and enforces policy', async () => {
    const recorder = vi.fn();
    setAuditRecorder(recorder);
    const isAllowed = vi.fn<[string, string[]], boolean>()
      .mockReturnValueOnce(false)
      .mockReturnValueOnce(true);
    const engine: PolicyEngine = { isAllowed };
    const middleware = createAuthMiddleware(engine, headerExtractor);

    const deniedSend = vi.fn();
    const deniedStatus = vi.fn<[number], ReplySender>().mockReturnValue({ send: deniedSend });
    const deniedReply: ReplyLike = {
      status: deniedStatus,
    };
    const deniedRequest: RequestLike = {
      method: 'GET',
      url: '/secure',
      headers: { 'x-actor-roles': 'viewer', 'x-request-id': 'req-1' },
    };
    await middleware(
      deniedRequest,
      deniedReply,
    );
    expect(recorder).toHaveBeenCalledWith(expect.objectContaining({ allowed: false }));

    const okSend = vi.fn();
    const okStatus = vi.fn<[number], ReplySender>().mockReturnValue({ send: okSend });
    const okReply: ReplyLike = {
      status: okStatus,
    };
    const okRequest: RequestLike = {
      method: 'GET',
      url: '/secure',
      headers: { 'x-actor-roles': 'admin', 'x-request-id': 'req-2' },
    };
    await middleware(
      okRequest,
      okReply,
    );
    expect(recorder).toHaveBeenLastCalledWith(expect.objectContaining({ allowed: true }));
    expect(isAllowed).toHaveBeenCalledTimes(2);
  });
});

