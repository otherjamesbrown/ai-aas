import { Pool } from 'pg';
import type { DatabaseConfig } from '../config';
import { type Probe } from './health';
export interface DataAccess {
    pool: Pool;
    probe: Probe;
}
export declare function initDataAccess(config: DatabaseConfig): DataAccess;
