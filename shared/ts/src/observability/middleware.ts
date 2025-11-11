import { randomUUID } from 'node:crypto';

export interface RequestWithHeaders {
  headers: Record<string, unknown>;
  id?: string;
}

export interface ReplyWithHeader {
  header: (name: string, value: string) => void;
}

export function createRequestContextHook() {
  return async function requestContext(
    request: RequestWithHeaders,
    reply: ReplyWithHeader,
  ) {
    const reqId = String(request.headers['x-request-id'] ?? randomUUID());
    request.headers['x-request-id'] = reqId;
    reply.header('x-request-id', reqId);
    request.id = reqId;
  };
}

