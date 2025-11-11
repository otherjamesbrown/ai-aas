import { toErrorResponse } from '../errors';
import { buildAuditEvent, recordAuditEvent } from './audit';
import type { PolicyEngine } from './policy';

export interface AuthActor {
  subject: string;
  roles: string[];
}

export type ActorExtractor = (request: { headers: Record<string, unknown> }) => AuthActor;

export const headerExtractor: ActorExtractor = (request) => {
  const subject = String(request.headers['x-actor-subject'] ?? '');
  const rawRoles = String(request.headers['x-actor-roles'] ?? '');
  const roles = rawRoles
    .split(',')
    .map((role) => role.trim())
    .filter(Boolean);
  return { subject, roles };
};

export interface RequestLike {
  method: string;
  url: string;
  headers: Record<string, unknown>;
  log?: { warn?: (msg: unknown) => void };
}

export interface ReplyLike {
  status: (code: number) => ReplySender;
}

export interface ReplySender {
  send: (payload: unknown) => void;
}

export function createAuthMiddleware(
  engine: PolicyEngine,
  extractor: ActorExtractor = headerExtractor,
) {
  return async function authorize(
    request: RequestLike,
    reply: ReplyLike,
  ) {
    const actor = extractor(request);
    const action = `${request.method.toUpperCase()}:${request.url}`;
    const allowed = engine.isAllowed(action, actor.roles);
    recordAuditEvent(buildAuditEvent(action, actor, allowed));

    if (!allowed) {
      request.log?.warn?.({ actor, message: 'authorization denied' });
      const response = toErrorResponse(
        new Error('access denied'),
      );
      response.code = 'UNAUTHORIZED';
      response.actor = actor;
      response.request_id = String(request.headers['x-request-id'] ?? '');
      const sender = reply.status(403);
      sender.send(response);
      return reply;
    }
    return undefined;
  };
}

