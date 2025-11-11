import { describe, it, expect } from 'vitest';
import { Registry } from './health';

describe('Registry', () => {
  it('evaluates probes', async () => {
    const registry = new Registry();
    registry.register('ok', async () => {});
    registry.register('fail', async () => {
      throw new Error('boom');
    });

    const result = await registry.evaluate();
    expect(result.checks.ok.healthy).toBe(true);
    expect(result.checks.fail.healthy).toBe(false);
  });
});

