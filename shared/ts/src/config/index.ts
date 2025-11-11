import { config as loadEnv } from 'dotenv';

loadEnv();

export interface ServiceConfig {
  name: string;
  host: string;
  port: number;
}

export interface TelemetryConfig {
  endpoint: string;
  protocol: 'grpc' | 'http';
  headers: Record<string, string>;
  insecure: boolean;
}

export interface DatabaseConfig {
  dsn: string;
  maxIdleConns: number;
  maxOpenConns: number;
  connMaxLifetimeMs: number;
}

export interface SharedConfig {
  service: ServiceConfig;
  telemetry: TelemetryConfig;
  database: DatabaseConfig;
}

export function loadConfig(): SharedConfig {
  const protocolEnv = getEnv('OTEL_EXPORTER_OTLP_PROTOCOL', 'grpc').toLowerCase();
  if (protocolEnv !== 'grpc' && protocolEnv !== 'http') {
    throw new Error(`Unsupported OTLP protocol: ${protocolEnv}`);
  }

  const name = getEnv('SERVICE_NAME', 'shared-service');
  if (!name.trim()) {
    throw new Error('SERVICE_NAME must be provided');
  }

  return {
    service: {
      name,
      host: getEnv('SERVICE_HOST', '0.0.0.0'),
      port: parseNumber(getEnv('SERVICE_PORT', '8080'), 8080),
    },
    telemetry: {
      endpoint: getEnv('OTEL_EXPORTER_OTLP_ENDPOINT', 'localhost:4317'),
      protocol: protocolEnv as 'grpc' | 'http',
      headers: parseHeaders(getEnv('OTEL_EXPORTER_OTLP_HEADERS', '')),
      insecure: getEnv('OTEL_EXPORTER_OTLP_INSECURE', 'false') === 'true',
    },
    database: {
      dsn: getEnv('DATABASE_DSN', ''),
      maxIdleConns: parseNumber(getEnv('DATABASE_MAX_IDLE_CONNS', '2'), 2),
      maxOpenConns: parseNumber(getEnv('DATABASE_MAX_OPEN_CONNS', '10'), 10),
      connMaxLifetimeMs: parseDurationMs(getEnv('DATABASE_CONN_MAX_LIFETIME', '5m')),
    },
  };
}

export function mustLoadConfig(): SharedConfig {
  return loadConfig();
}

function getEnv(key: string, fallback: string): string {
  return process.env[key] ?? fallback;
}

function parseHeaders(raw: string): Record<string, string> {
  if (!raw) {
    return {};
  }
  return raw.split(',').reduce<Record<string, string>>((acc, pair) => {
    const [k, v] = pair.split('=');
    if (!k || v === undefined) {
      return acc;
    }
    acc[k.trim().toLowerCase()] = v.trim();
    return acc;
  }, {});
}

function parseDurationMs(value: string): number {
  const match = /^(\d+)(ms|s|m|h)$/.exec(value.trim());
  if (!match) {
    return 5 * 60 * 1000;
  }
  const [, amount, unit] = match;
  const quantity = Number(amount);
  switch (unit) {
    case 'ms':
      return quantity;
    case 's':
      return quantity * 1000;
    case 'm':
      return quantity * 60 * 1000;
    case 'h':
      return quantity * 60 * 60 * 1000;
    default:
      return 5 * 60 * 1000;
  }
}

function parseNumber(value: string, fallback: number): number {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : fallback;
}

