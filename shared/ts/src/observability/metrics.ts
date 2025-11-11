import { Counter } from 'prom-client';

export const telemetryExporterFailures = new Counter({
  name: 'shared_telemetry_export_failures_total',
  help: 'Number of telemetry exporter initialization failures by protocol.',
  labelNames: ['service_name', 'exporter'] as const,
});

export function incrementTelemetryExporterFailure(serviceName: string, exporter: string) {
  telemetryExporterFailures.labels(serviceName || 'unknown', exporter).inc();
}

export function resetTelemetryMetrics(): void {
  telemetryExporterFailures.reset();
}

export async function telemetryExporterFailureCount(serviceName: string, exporter: string): Promise<number> {
  const metric = await telemetryExporterFailures.get();
  const target = metric?.values?.find(
    (value) =>
      value.labels.service_name === (serviceName || 'unknown') && value.labels.exporter === exporter,
  );
  return target?.value ?? 0;
}

