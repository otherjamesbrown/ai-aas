import { describe, it, expect, vi, beforeAll, afterEach } from 'vitest';

type PoolConfig = {
  connectionString: string;
  max: number;
  idleTimeoutMillis: number;
};

class MockPool {
  public config: PoolConfig;

  constructor(config: PoolConfig) {
    this.config = config;
    mockPools.push(this);
  }

  async query(sql: string): Promise<string> {
    return sql;
  }

  async end(): Promise<void> {
    return;
  }
}

const mockPools: MockPool[] = [];

vi.mock('pg', () => {
  return {
    Pool: MockPool,
  };
});

let initDataAccess: typeof import('./index').initDataAccess;
let Registry: typeof import('./health').Registry;

beforeAll(async () => {
  ({ initDataAccess } = await import('./index'));
  ({ Registry } = await import('./health'));
});

afterEach(() => {
  mockPools.length = 0;
});

describe('initDataAccess', () => {
  it('creates a pool and probe wired to health registry', async () => {
    const dataAccess = initDataAccess({
      dsn: 'postgres://example',
      maxIdleConns: 2,
      maxOpenConns: 4,
      connMaxLifetimeMs: 1000,
    });

    expect(mockPools).toHaveLength(1);
    expect(mockPools[0].config).toMatchObject({
      connectionString: 'postgres://example',
      max: 4,
      idleTimeoutMillis: 1000,
    });
    expect(dataAccess.pool).toBe(mockPools[0]);

    const registry = new Registry();
    registry.register('database', dataAccess.probe);
    const result = await registry.evaluate();
    expect(result.checks.database.healthy).toBe(true);
  });
});

