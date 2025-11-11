import { bench, beforeAll, beforeEach, describe, vi } from 'vitest';

import { createRequestContextHook } from '@ai-aas/shared/observability/middleware';
import { startTelemetry } from '@ai-aas/shared/observability';
import { loadConfig } from '@ai-aas/shared/config';

const grpcExporterMock = vi.fn();
const httpExporterMock = vi.fn();
const nodeSdkStart = vi.fn();
const nodeSdkShutdown = vi.fn();

beforeAll(() => {
  (globalThis as any).__otelGrpcMock = grpcExporterMock;
  (globalThis as any).__otelHttpMock = httpExporterMock;
  (globalThis as any).__otelNodeStartMock = nodeSdkStart;
  (globalThis as any).__otelNodeShutdownMock = nodeSdkShutdown;

  process.env.SERVICE_NAME = 'shared-service';
  process.env.OTEL_EXPORTER_OTLP_ENDPOINT = 'localhost:4317';
  process.env.OTEL_EXPORTER_OTLP_PROTOCOL = 'grpc';
  process.env.OTEL_EXPORTER_OTLP_HEADERS = '';
  process.env.OTEL_EXPORTER_OTLP_INSECURE = 'true';
});

const hook = createRequestContextHook();

beforeEach(() => {
  vi.clearAllMocks();
});

describe('shared observability performance', () => {
  bench('request context hook', async () => {
    const reply = { header: () => {} };
    const request = { headers: {} as Record<string, string>, id: undefined as string | undefined };

    await hook(request, reply);
  });

  bench('telemetry init/graceful shutdown', async () => {
    nodeSdkStart.mockImplementationOnce(() => {});
    nodeSdkShutdown.mockImplementationOnce(() => {});

    const telemetry = await startTelemetry({
      serviceName: 'bench-service',
      environment: 'test',
      endpoint: 'collector:4317',
      protocol: 'grpc',
      headers: {},
      insecure: true,
    });

    await telemetry.shutdown();
  });

  bench('config load', () => {
    loadConfig();
  });
});

