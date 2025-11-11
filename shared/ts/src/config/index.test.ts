import { describe, it, expect, afterEach } from 'vitest';
import { loadConfig } from './index';

const ORIGINAL_ENV = { ...process.env };

afterEach(() => {
  process.env = { ...ORIGINAL_ENV };
});

describe('config loader', () => {
  it('loads defaults', () => {
    delete process.env.SERVICE_NAME;
    const cfg = loadConfig();
    expect(cfg.service.name).toBe('shared-service');
    expect(cfg.telemetry.protocol).toBe('grpc');
  });

  it('validates protocol', () => {
    process.env.OTEL_EXPORTER_OTLP_PROTOCOL = 'ws';
    expect(() => loadConfig()).toThrowError(/Unsupported OTLP protocol/);
  });
});

