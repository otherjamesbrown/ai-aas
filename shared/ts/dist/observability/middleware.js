import { randomUUID } from 'node:crypto';
export function createRequestContextHook() {
    return async function requestContext(request, reply) {
        const reqId = String(request.headers['x-request-id'] ?? randomUUID());
        request.headers['x-request-id'] = reqId;
        reply.header('x-request-id', reqId);
        request.id = reqId;
    };
}
//# sourceMappingURL=middleware.js.map