import { describe, it, expect, beforeEach } from 'vitest';

import {
  startTelemetry,
  resetTelemetryMetrics,
  telemetryExporterFailureCount,
} from '../../../../shared/ts/src/observability/index';

describe('telemetry failover', () => {
  beforeEach(() => {
    resetTelemetryMetrics();
  });

  it('falls back when exporter protocol is unsupported', async () => {
    const telemetry = await startTelemetry({
      serviceName: 'integration-ts',
      environment: 'test',
      endpoint: '127.0.0.1:43179',
      protocol: 'ws' as 'grpc',
      headers: {},
      insecure: true,
    });

    await expect(telemetry.shutdown()).resolves.toBeUndefined();

    await expect(telemetryExporterFailureCount('integration-ts', 'ws')).resolves.toBeGreaterThan(0);
    await expect(telemetryExporterFailureCount('integration-ts', 'degraded')).resolves.toBeGreaterThan(0);
  });
});

