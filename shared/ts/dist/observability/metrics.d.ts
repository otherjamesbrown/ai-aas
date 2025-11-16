import { Counter } from 'prom-client';
export declare const telemetryExporterFailures: Counter<"service_name" | "exporter">;
export declare function incrementTelemetryExporterFailure(serviceName: string, exporter: string): void;
export declare function resetTelemetryMetrics(): void;
export declare function telemetryExporterFailureCount(serviceName: string, exporter: string): Promise<number>;
