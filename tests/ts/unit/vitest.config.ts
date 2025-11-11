import { defineConfig } from 'vitest/config';

export default defineConfig({
  root: '.',
  test: {
    include: ['src/**/*.test.ts'],
    reporters: ['default'],
    coverage: {
      enabled: false
    }
  },
  resolve: {
    alias: {
      '@ai-aas/shared': '../../shared/ts/src'
    }
  }
});

