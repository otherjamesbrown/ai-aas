import { Counter } from 'prom-client';
export const telemetryExporterFailures = new Counter({
    name: 'shared_telemetry_export_failures_total',
    help: 'Number of telemetry exporter initialization failures by protocol.',
    labelNames: ['service_name', 'exporter'],
});
export function incrementTelemetryExporterFailure(serviceName, exporter) {
    telemetryExporterFailures.labels(serviceName || 'unknown', exporter).inc();
}
export function resetTelemetryMetrics() {
    telemetryExporterFailures.reset();
}
export async function telemetryExporterFailureCount(serviceName, exporter) {
    const metric = await telemetryExporterFailures.get();
    const target = metric?.values?.find((value) => value.labels.service_name === (serviceName || 'unknown') && value.labels.exporter === exporter);
    return target?.value ?? 0;
}
//# sourceMappingURL=metrics.js.map