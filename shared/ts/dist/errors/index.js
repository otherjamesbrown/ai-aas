export class SharedError extends Error {
    code;
    options;
    constructor(code, message, options = {}) {
        super(message);
        this.code = code;
        this.options = options;
    }
    toResponse() {
        return {
            error: this.message,
            code: this.code,
            detail: this.options.detail,
            request_id: this.options.requestId,
            trace_id: this.options.traceId,
            actor: this.options.actor,
            timestamp: (this.options.timestamp ?? new Date()).toISOString(),
        };
    }
}
export function toSharedError(err) {
    if (err instanceof SharedError) {
        return err;
    }
    const message = err instanceof Error ? err.message : String(err);
    return new SharedError('INTERNAL', 'unexpected error occurred', { detail: message });
}
export function toErrorResponse(err) {
    return toSharedError(err).toResponse();
}
//# sourceMappingURL=index.js.map