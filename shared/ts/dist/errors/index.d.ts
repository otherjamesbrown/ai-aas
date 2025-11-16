export interface Actor {
    subject: string;
    roles: string[];
}
export interface ErrorResponse {
    error: string;
    code: string;
    detail?: string;
    request_id?: string;
    trace_id?: string;
    actor?: Actor;
    timestamp: string;
}
export declare class SharedError extends Error {
    readonly code: string;
    readonly options: {
        detail?: string;
        requestId?: string;
        traceId?: string;
        actor?: Actor;
        timestamp?: Date;
    };
    constructor(code: string, message: string, options?: {
        detail?: string;
        requestId?: string;
        traceId?: string;
        actor?: Actor;
        timestamp?: Date;
    });
    toResponse(): ErrorResponse;
}
export declare function toSharedError(err: unknown): SharedError;
export declare function toErrorResponse(err: unknown): ErrorResponse;
