import { defineConfig } from 'vitest/config';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const rootDir = path.dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  root: '.',
  test: {
    include: ['src/**/*.test.ts'],
    reporters: ['default'],
    coverage: {
      enabled: false,
    },
  },
  resolve: {
    alias: [
      {
        find: '@ai-aas/shared',
        replacement: path.resolve(rootDir, '../../../shared/ts/src'),
      },
      {
        find: 'pg',
        replacement: path.resolve(rootDir, '__mocks__/pg.ts'),
      },
      {
        find: '@opentelemetry/exporter-trace-otlp-grpc',
        replacement: path.resolve(rootDir, '__mocks__/otel-exporter-grpc.ts'),
      },
      {
        find: '@opentelemetry/exporter-trace-otlp-http',
        replacement: path.resolve(rootDir, '__mocks__/otel-exporter-http.ts'),
      },
      {
        find: '@opentelemetry/sdk-node',
        replacement: path.resolve(rootDir, '__mocks__/otel-sdk-node.ts'),
      },
    ],
  },
  server: {
    deps: {
      inline: ['pg'],
    },
  },
});

