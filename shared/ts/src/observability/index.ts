import { diag, DiagConsoleLogger, DiagLogLevel } from '@opentelemetry/api';
import { Metadata, credentials } from '@grpc/grpc-js';
import { OTLPTraceExporter as OTLPGrpcExporter } from '@opentelemetry/exporter-trace-otlp-grpc';
import { OTLPTraceExporter as OTLPHttpExporter } from '@opentelemetry/exporter-trace-otlp-http';
import { Resource } from '@opentelemetry/resources';
import { NodeSDK } from '@opentelemetry/sdk-node';
import { SemanticResourceAttributes } from '@opentelemetry/semantic-conventions';

import type { TelemetryConfig } from '../config';

diag.setLogger(new DiagConsoleLogger(), DiagLogLevel.INFO);

export interface Telemetry {
  shutdown(): Promise<void>;
}

export async function startTelemetry(config: TelemetryConfig & { serviceName: string; environment?: string }): Promise<Telemetry> {
  const exporter =
    config.protocol === 'http'
      ? new OTLPHttpExporter({
          url: buildHttpUrl(config),
          headers: config.headers,
        })
      : new OTLPGrpcExporter({
          url: config.endpoint,
          metadata: buildMetadata(config.headers),
          credentials: config.insecure ? credentials.createInsecure() : credentials.createSsl(),
        });

  const sdk = new NodeSDK({
    traceExporter: exporter,
    resource: new Resource({
      [SemanticResourceAttributes.SERVICE_NAME]: config.serviceName,
      [SemanticResourceAttributes.DEPLOYMENT_ENVIRONMENT]: config.environment ?? 'development',
    }),
  });

  await sdk.start();

  return {
    shutdown: async () => {
      await sdk.shutdown();
    },
  };
}

function buildMetadata(headers: Record<string, string>): Metadata | undefined {
  if (!headers || Object.keys(headers).length === 0) {
    return undefined;
  }
  const md = new Metadata();
  for (const [key, value] of Object.entries(headers)) {
    md.set(key, value);
  }
  return md;
}

function buildHttpUrl(config: TelemetryConfig): string {
  if (config.endpoint.startsWith('http')) {
    return config.endpoint;
  }
  const scheme = config.insecure ? 'http' : 'https';
  const base = `${scheme}://${config.endpoint}`.replace(/\/+$/, '');
  return `${base}/v1/traces`;
}

