import { describe, it, expect, vi, beforeEach, beforeAll } from 'vitest';
import type { RequestWithHeaders, ReplyWithHeader } from './middleware';

vi.mock('node:crypto', () => ({
  randomUUID: vi.fn(() => 'generated-id'),
}));

let createRequestContextHook: typeof import('./middleware').createRequestContextHook;

beforeAll(async () => {
  ({ createRequestContextHook } = await import('./middleware'));
});

describe('createRequestContextHook', () => {
  let hook: ReturnType<typeof createRequestContextHook>;

  beforeEach(() => {
    vi.clearAllMocks();
    hook = createRequestContextHook();
  });

  it('injects a generated request id when missing', async () => {
    const reply: ReplyWithHeader = { header: vi.fn() };
    const request: RequestWithHeaders = { headers: {} };

    await hook(request, reply);

    expect(request.headers['x-request-id']).toBe('generated-id');
    expect(request.id).toBe('generated-id');
    expect(reply.header).toHaveBeenCalledWith('x-request-id', 'generated-id');
  });

  it('reuses provided request id', async () => {
    const reply: ReplyWithHeader = { header: vi.fn() };
    const request: RequestWithHeaders = { headers: { 'x-request-id': 'existing' } };

    await hook(request, reply);

    expect(request.headers['x-request-id']).toBe('existing');
    expect(request.id).toBe('existing');
    expect(reply.header).toHaveBeenCalledWith('x-request-id', 'existing');
  });
});

