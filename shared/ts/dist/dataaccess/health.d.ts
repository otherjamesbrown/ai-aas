export type Probe = () => Promise<void>;
export interface Status {
    healthy: boolean;
    error?: string;
    latencyMs: number;
}
export interface Result {
    checks: Record<string, Status>;
}
export declare class Registry {
    private probes;
    register(name: string, probe: Probe): void;
    evaluate(): Promise<Result>;
}
type ReplyLike = {
    status?: (code: number) => ReplyLike | void;
    send?: (body: unknown) => void;
};
export declare function httpHandler(registry: Registry): (_request: unknown, reply: ReplyLike) => Promise<Result | undefined>;
export declare function databaseProbe(pool: {
    query: (query: string) => Promise<unknown>;
}): Probe;
export {};
