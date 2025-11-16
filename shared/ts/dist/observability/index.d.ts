import type { TelemetryConfig } from '../config';
export interface Telemetry {
    shutdown(): Promise<void>;
}
export declare function startTelemetry(config: TelemetryConfig & {
    serviceName: string;
    environment?: string;
}): Promise<Telemetry>;
export { telemetryExporterFailures, resetTelemetryMetrics, telemetryExporterFailureCount, } from './metrics';
