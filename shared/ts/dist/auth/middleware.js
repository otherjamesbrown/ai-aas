import { toErrorResponse } from '../errors';
import { buildAuditEvent, recordAuditEvent } from './audit';
export const headerExtractor = (request) => {
    const subject = String(request.headers['x-actor-subject'] ?? '');
    const rawRoles = String(request.headers['x-actor-roles'] ?? '');
    const roles = rawRoles
        .split(',')
        .map((role) => role.trim())
        .filter(Boolean);
    return { subject, roles };
};
export function createAuthMiddleware(engine, extractor = headerExtractor) {
    return async function authorize(request, reply) {
        const actor = extractor(request);
        const action = `${request.method.toUpperCase()}:${request.url}`;
        const allowed = engine.isAllowed(action, actor.roles);
        recordAuditEvent(buildAuditEvent(action, actor, allowed));
        if (!allowed) {
            request.log?.warn?.({ actor, message: 'authorization denied' });
            const response = toErrorResponse(new Error('access denied'));
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
//# sourceMappingURL=middleware.js.map