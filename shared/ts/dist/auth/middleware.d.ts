import type { PolicyEngine } from './policy';
export interface AuthActor {
    subject: string;
    roles: string[];
}
export type ActorExtractor = (request: {
    headers: Record<string, unknown>;
}) => AuthActor;
export declare const headerExtractor: ActorExtractor;
export interface RequestLike {
    method: string;
    url: string;
    headers: Record<string, unknown>;
    log?: {
        warn?: (msg: unknown) => void;
    };
}
export interface ReplyLike {
    status: (code: number) => ReplySender;
}
export interface ReplySender {
    send: (payload: unknown) => void;
}
export declare function createAuthMiddleware(engine: PolicyEngine, extractor?: ActorExtractor): (request: RequestLike, reply: ReplyLike) => Promise<ReplyLike | undefined>;
