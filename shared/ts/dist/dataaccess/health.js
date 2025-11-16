import { performance } from 'node:perf_hooks';
export class Registry {
    probes = new Map();
    register(name, probe) {
        this.probes.set(name, probe);
    }
    async evaluate() {
        const entries = await Promise.all(Array.from(this.probes.entries()).map(async ([name, probe]) => {
            const start = performance.now();
            try {
                await probe();
                return [
                    name,
                    {
                        healthy: true,
                        latencyMs: performance.now() - start,
                    },
                ];
            }
            catch (err) {
                return [
                    name,
                    {
                        healthy: false,
                        error: err instanceof Error ? err.message : String(err),
                        latencyMs: performance.now() - start,
                    },
                ];
            }
        }));
        return {
            checks: Object.fromEntries(entries),
        };
    }
}
export function httpHandler(registry) {
    return async (_request, reply) => {
        const result = await registry.evaluate();
        const status = Object.values(result.checks).some((check) => !check.healthy) ? 503 : 200;
        if (reply && typeof reply.status === 'function' && typeof reply.send === 'function') {
            const maybeReply = reply.status(status);
            if (maybeReply && typeof maybeReply.send === 'function') {
                maybeReply.send(result);
            }
            else {
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
export function databaseProbe(pool) {
    return async () => {
        await pool.query('SELECT 1');
    };
}
//# sourceMappingURL=health.js.map