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
export declare function loadConfig(): SharedConfig;
export declare function mustLoadConfig(): SharedConfig;
