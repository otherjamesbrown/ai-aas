import { defineConfig } from 'vitest/config';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const rootDir = path.dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  test: {
    include: ['src/**/*.spec.ts'],
    environment: 'node',
  },
  resolve: {
    alias: [
      {
        find: '@ai-aas/shared/observability',
        replacement: path.resolve(rootDir, '../../shared/ts/src/observability/index.ts'),
      },
      {
        find: '@ai-aas/shared',
        replacement: path.resolve(rootDir, '../../shared/ts/src'),
      },
    ],
  },
});

