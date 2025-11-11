import { defineConfig } from 'vitest/config';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const rootDir = path.dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  root: '.',
  test: {
    include: ['src/**/*.spec.ts'],
    reporters: ['default'],
  },
  resolve: {
    alias: [
      {
        find: '@ai-aas/shared',
        replacement: path.resolve(rootDir, '../../../shared/ts/src'),
      },
    ],
  },
});

