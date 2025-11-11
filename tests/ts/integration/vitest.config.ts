import { defineConfig } from 'vitest/config';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const rootDir = path.dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  test: {
    include: ['src/**/*.spec.ts'],
    environment: 'node'
  },
  resolve: {
    alias: {
      '@ai-aas/shared': path.resolve(rootDir, '../../shared/ts/src')
    }
  }
});

