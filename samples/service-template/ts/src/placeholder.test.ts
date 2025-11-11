import { describe, it, expect } from 'vitest';
import Fastify from 'fastify';
import path from 'node:path';
import { PolicyEngine, createAuthMiddleware, Registry, httpHandler, createRequestContextHook } from '../../../shared/ts/src/index';

describe('service-template ts secure route', () => {
  it('enforces authorization', async () => {
    const app = Fastify({ logger: false });
    app.addHook('onRequest', createRequestContextHook());

    const registry = new Registry();
    registry.register('self', async () => {});
    app.get('/healthz', httpHandler(registry));

    const policyPath = path.resolve(__dirname, '../policies/service-template/policy.json');
    const policyEngine = await PolicyEngine.fromFile(policyPath);

    await app.register(async (instance) => {
      instance.addHook('preHandler', createAuthMiddleware(policyEngine));
      instance.get('/secure/data', async () => ({ ok: true }));
    });

    const denied = await app.inject({
      method: 'GET',
      url: '/secure/data',
      headers: { 'x-actor-roles': 'viewer' },
    });
    expect(denied.statusCode).toBe(403);

    const allowed = await app.inject({
      method: 'GET',
      url: '/secure/data',
      headers: { 'x-actor-roles': 'admin', 'x-actor-subject': 'alice' },
    });
    expect(allowed.statusCode).toBe(200);
    await app.close();
  });
});

