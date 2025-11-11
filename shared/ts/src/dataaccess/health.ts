import { performance } from 'node:perf_hooks';

export type Probe = () => Promise<void>;

export interface Status {
  healthy: boolean;
  error?: string;
  latencyMs: number;
}

export interface Result {
  checks: Record<string, Status>;
}

export class Registry {
  private probes: Map<string, Probe> = new Map();

  register(name: string, probe: Probe) {
    this.probes.set(name, probe);
  }

  async evaluate(): Promise<Result> {
    const entries = await Promise.all(
      Array.from(this.probes.entries()).map(async ([name, probe]) => {
        const start = performance.now();
        try {
          await probe();
          return [
            name,
            {
              healthy: true,
              latencyMs: performance.now() - start,
            } satisfies Status,
          ] as const;
        } catch (err) {
          return [
            name,
            {
              healthy: false,
              error: err instanceof Error ? err.message : String(err),
              latencyMs: performance.now() - start,
            } satisfies Status,
          ] as const;
        }
      }),
    );

    return {
      checks: Object.fromEntries(entries),
    };
  }
}

type ReplyLike = {
  status?: (code: number) => ReplyLike | void;
  send?: (body: unknown) => void;
};

export function httpHandler(registry: Registry) {
  return async (_request: unknown, reply: ReplyLike) => {
    const result = await registry.evaluate();
    const status = Object.values(result.checks).some((check) => !check.healthy) ? 503 : 200;
    if (reply && typeof reply.status === 'function' && typeof reply.send === 'function') {
      const maybeReply = reply.status(status);
      if (maybeReply && typeof maybeReply.send === 'function') {
        maybeReply.send(result);
      } else {
        reply.send(result);
      }
      return;
    }
    if (reply && typeof reply.send === 'function') {
      reply.send(result);
      return;
    }
    return result;
  };
}

export function databaseProbe(pool: { query: (query: string) => Promise<unknown> }): Probe {
  return async () => {
    await pool.query('SELECT 1');
  };
}

