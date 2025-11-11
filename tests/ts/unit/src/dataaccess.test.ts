import { describe, it, expect, vi, afterEach, beforeAll } from 'vitest';

vi.mock('pg', () => import('../__mocks__/pg'));

type InitDataAccess = typeof import('@ai-aas/shared/dataaccess').initDataAccess;
type HealthModule = typeof import('@ai-aas/shared/dataaccess/health');

let initDataAccess: InitDataAccess;
let Registry: HealthModule['Registry'];
let httpHandler: HealthModule['httpHandler'];
let mockPoolInstances: typeof import('pg')['mockPoolInstances'];

beforeAll(async () => {
  ({ mockPoolInstances } = await import('pg'));
  ({ initDataAccess } = await import('@ai-aas/shared/dataaccess'));
  ({ Registry, httpHandler } = await import('@ai-aas/shared/dataaccess/health'));
});

afterEach(() => {
  vi.clearAllMocks();
  mockPoolInstances.length = 0;
});

describe('data access helpers', () => {
  it('creates pooled connections and probes', async () => {
    const dataAccess = initDataAccess({
      dsn: 'postgres://example',
      maxIdleConns: 1,
      maxOpenConns: 4,
      connMaxLifetimeMs: 1000,
    });

    expect(mockPoolInstances).toHaveLength(1);
    expect(mockPoolInstances[0].config).toMatchObject({
      connectionString: 'postgres://example',
      max: 4,
      idleTimeoutMillis: 1000,
    });
    expect(dataAccess.pool).toBe(mockPoolInstances[0]);
    const registry = new Registry();
    registry.register('database', dataAccess.probe);
    const result = await registry.evaluate();
    expect(result.checks.database.healthy).toBe(true);

    const handler = httpHandler(registry);
    const request = { method: 'GET' } as const;
    const send = vi.fn();
    const status = vi.fn<[number], { send: (body: unknown) => void }>().mockReturnValue({ send });
    const reply = {
      status,
      send,
    };
    await handler(request, reply);
    expect(status).toHaveBeenCalledWith(200);
  });
});

