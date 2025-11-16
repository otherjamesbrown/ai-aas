let recorder = () => { };
export function setAuditRecorder(next) {
    recorder = next ?? (() => { });
}
export function recordAuditEvent(event) {
    recorder(event);
}
export function buildAuditEvent(action, actor, allowed) {
    return {
        action,
        subject: actor.subject,
        roles: [...actor.roles],
        allowed,
    };
}
//# sourceMappingURL=audit.js.map