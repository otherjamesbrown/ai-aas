import { describe, it, expect } from 'vitest';
import Fastify from 'fastify';
import { PolicyEngine, createAuthMiddleware, createRequestContextHook, Registry, httpHandler } from '../../../shared/ts/src/index';

const engine = PolicyEngine.fromString(
  JSON.stringify({
    rules: {
      'GET:/secure/resource': ['admin'],
    },
  }),
);

describe('shared authorization middleware (integration)', () => {
  it('enforces policy for secure routes', async () => {
    const app = Fastify();
    app.addHook('onRequest', createRequestContextHook());

    const registry = new Registry();
    registry.register('self', async () => {});
    app.get('/healthz', httpHandler(registry));

    await app.register(async (instance) => {
      instance.addHook('preHandler', createAuthMiddleware(engine));
      instance.get('/secure/resource', async () => ({ ok: true }));
    });

    const denied = await app.inject({
      method: 'GET',
      url: '/secure/resource',
      headers: {
        'x-actor-roles': 'viewer',
      },
    });
    expect(denied.statusCode).toBe(403);

    const allowed = await app.inject({
      method: 'GET',
      url: '/secure/resource',
      headers: {
        'x-actor-roles': 'admin',
      },
    });
    expect(allowed.statusCode).toBe(200);
    expect(JSON.parse(allowed.body)).toEqual({ ok: true });

    await app.close();
  });
});

