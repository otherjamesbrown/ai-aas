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

export class SharedError extends Error {
  constructor(
    public readonly code: string,
    message: string,
    public readonly options: {
      detail?: string;
      requestId?: string;
      traceId?: string;
      actor?: Actor;
      timestamp?: Date;
    } = {},
  ) {
    super(message);
  }

  toResponse(): ErrorResponse {
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

export function toSharedError(err: unknown): SharedError {
  if (err instanceof SharedError) {
    return err;
  }
  const message = err instanceof Error ? err.message : String(err);
  return new SharedError('INTERNAL', 'unexpected error occurred', { detail: message });
}

export function toErrorResponse(err: unknown): ErrorResponse {
  return toSharedError(err).toResponse();
}

