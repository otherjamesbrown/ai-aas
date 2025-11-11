import { defineConfig } from 'vitest/config';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const rootDir = path.dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  root: '.',
  test: {
    include: [],
  },
  benchmark: {
    include: ['src/**/*.bench.ts'],
  },
  resolve: {
    alias: {
      '@ai-aas/shared': path.resolve(rootDir, '../../../shared/ts/src'),
      '@opentelemetry/exporter-trace-otlp-grpc': path.resolve(rootDir, '__mocks__/otel-exporter-grpc.ts'),
      '@opentelemetry/exporter-trace-otlp-http': path.resolve(rootDir, '__mocks__/otel-exporter-http.ts'),
      '@opentelemetry/sdk-node': path.resolve(rootDir, '__mocks__/otel-sdk-node.ts'),
    },
  },
});

