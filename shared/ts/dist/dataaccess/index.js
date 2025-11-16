import { Pool } from 'pg';
import { databaseProbe } from './health';
export function initDataAccess(config) {
    const pool = new Pool({
        connectionString: config.dsn,
        max: config.maxOpenConns,
        idleTimeoutMillis: config.connMaxLifetimeMs,
    });
    return { pool, probe: databaseProbe(pool) };
}
//# sourceMappingURL=index.js.map