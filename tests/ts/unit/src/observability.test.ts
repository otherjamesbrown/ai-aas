import { describe, it, expect, vi, afterEach, beforeAll, beforeEach } from 'vitest';
import type { TelemetryConfig } from '@ai-aas/shared/config';
import type { RequestWithHeaders, ReplyWithHeader } from '@ai-aas/shared/observability/middleware';

declare global {
  // eslint-disable-next-line no-var
  var __otelGrpcMock: ((options: unknown) => void) | undefined;
  // eslint-disable-next-line no-var
  var __otelHttpMock: ((options: unknown) => void) | undefined;
  // eslint-disable-next-line no-var
  var __otelNodeStartMock: ((config: unknown) => void) | undefined;
  // eslint-disable-next-line no-var
  var __otelNodeShutdownMock: (() => void) | undefined;
}

const grpcExporterMock = vi.fn();
const httpExporterMock = vi.fn();
const nodeSdkStart = vi.fn();
const nodeSdkShutdown = vi.fn();

let startTelemetry: typeof import('@ai-aas/shared/observability').startTelemetry;
let createRequestContextHook: typeof import('@ai-aas/shared/observability/middleware').createRequestContextHook;

beforeAll(async () => {
  ({ startTelemetry } = await import('@ai-aas/shared/observability'));
  ({ createRequestContextHook } = await import('@ai-aas/shared/observability/middleware'));
});

beforeEach(() => {
  globalThis.__otelGrpcMock = grpcExporterMock;
  globalThis.__otelHttpMock = httpExporterMock;
  globalThis.__otelNodeStartMock = nodeSdkStart;
  globalThis.__otelNodeShutdownMock = nodeSdkShutdown;
});

afterEach(() => {
  vi.clearAllMocks();
});

describe('observability helpers', () => {
  it('initializes telemetry pipeline and shuts down cleanly', async () => {
    const telemetryConfig: TelemetryConfig & { serviceName: string; environment?: string } = {
      serviceName: 'shared-service',
      environment: 'test',
      endpoint: 'otel-collector:4317',
      protocol: 'grpc',
      headers: { authorization: 'Bearer token' },
      insecure: true,
    };
    const telemetry = await startTelemetry(telemetryConfig);

    expect(grpcExporterMock).toHaveBeenCalledWith(expect.objectContaining({
      url: 'otel-collector:4317',
    }));
    expect(httpExporterMock).not.toHaveBeenCalled();
    expect(nodeSdkStart).toHaveBeenCalled();

    await telemetry.shutdown();
    expect(nodeSdkShutdown).toHaveBeenCalled();
  });

  it('injects request identifiers', async () => {
    const hook = createRequestContextHook();
    const request: RequestWithHeaders = { headers: {}, id: undefined };
    const reply: ReplyWithHeader = { header: vi.fn() };

    await hook(request, reply);
    expect(request.headers['x-request-id']).toBeTruthy();
    expect(request.id).toBe(request.headers['x-request-id']);
    expect(reply.header).toHaveBeenCalledWith('x-request-id', request.headers['x-request-id']);
  });
});

