import { Pool } from 'pg';
import type { DatabaseConfig } from '../config';
import { databaseProbe, type Probe } from './health';

export interface DataAccess {
  pool: Pool;
  probe: Probe;
}

export function initDataAccess(config: DatabaseConfig): DataAccess {
  const pool = new Pool({
    connectionString: config.dsn,
    max: config.maxOpenConns,
    idleTimeoutMillis: config.connMaxLifetimeMs,
  });

  return { pool, probe: databaseProbe(pool) };
}

