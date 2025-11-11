import { Metadata } from '@grpc/grpc-js';
import { describe, it, expect, vi, beforeAll, beforeEach } from 'vitest';

const grpcExporterMock = vi.fn();
const httpExporterMock = vi.fn();
const nodeSdkStart = vi.fn();
const nodeSdkShutdown = vi.fn();

vi.mock('@opentelemetry/exporter-trace-otlp-grpc', () => ({
  __esModule: true,
  OTLPTraceExporter: class {
    constructor(options: unknown) {
      grpcExporterMock(options);
    }
  },
}));

vi.mock('@opentelemetry/exporter-trace-otlp-http', () => ({
  __esModule: true,
  OTLPTraceExporter: class {
    constructor(options: unknown) {
      httpExporterMock(options);
    }
  },
}));

vi.mock('@opentelemetry/sdk-node', () => ({
  __esModule: true,
  NodeSDK: class {
    public config: unknown;
    constructor(config: unknown) {
      this.config = config;
    }
    async start() {
      return nodeSdkStart(this.config);
    }
    async shutdown() {
      return nodeSdkShutdown();
    }
  },
}));

let startTelemetry: typeof import('./index').startTelemetry;
let resetTelemetryMetrics: typeof import('./index').resetTelemetryMetrics;
let telemetryExporterFailureCount: typeof import('./index').telemetryExporterFailureCount;

beforeAll(async () => {
  ({
    startTelemetry,
    resetTelemetryMetrics,
    telemetryExporterFailureCount,
  } = await import('./index'));
});

beforeEach(() => {
  vi.clearAllMocks();
  resetTelemetryMetrics();
});

describe('startTelemetry', () => {
  it('configures HTTP exporter and shuts down cleanly', async () => {
    const telemetry = await startTelemetry({
      serviceName: 'shared',
      environment: 'test',
      endpoint: 'collector:4318',
      protocol: 'http',
      headers: { authorization: 'Bearer token' },
      insecure: true,
    });

    expect(httpExporterMock).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'http://collector:4318/v1/traces',
        headers: { authorization: 'Bearer token' },
      }),
    );
    expect(nodeSdkStart).toHaveBeenCalled();
    await telemetry.shutdown();
    expect(nodeSdkShutdown).toHaveBeenCalled();
    expect(grpcExporterMock).not.toHaveBeenCalled();
  });

  it('defaults to grpc exporter with metadata', async () => {
    const telemetry = await startTelemetry({
      serviceName: 'shared',
      endpoint: 'collector:4317',
      protocol: 'grpc',
      headers: { 'x-team': 'observability' },
      insecure: false,
    });

    expect(grpcExporterMock).toHaveBeenCalled();
    const callArgs = grpcExporterMock.mock.calls.at(-1)?.[0] as { metadata?: Metadata } | undefined;
    expect(callArgs?.metadata?.get('x-team')).toEqual(['observability']);
    expect(httpExporterMock).not.toHaveBeenCalled();
    await telemetry.shutdown();
  });

  it('falls back to degraded telemetry when exporters fail', async () => {
    nodeSdkStart.mockRejectedValueOnce(new Error('boom grpc'));
    nodeSdkStart.mockRejectedValueOnce(new Error('boom http'));

    const telemetry = await startTelemetry({
      serviceName: 'shared',
      endpoint: 'collector:4317',
      protocol: 'grpc',
      headers: {},
      insecure: true,
    });

    expect(typeof telemetry.shutdown).toBe('function');
    await telemetry.shutdown();

    await expect(telemetryExporterFailureCount('shared', 'grpc')).resolves.toBeGreaterThan(0);
    await expect(telemetryExporterFailureCount('shared', 'http')).resolves.toBeGreaterThan(0);
    await expect(telemetryExporterFailureCount('shared', 'degraded')).resolves.toBeGreaterThan(0);
  });
});

