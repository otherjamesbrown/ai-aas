export interface AuditEvent {
  action: string;
  subject: string;
  roles: string[];
  allowed: boolean;
}

export type AuditRecorder = (event: AuditEvent) => void;

let recorder: AuditRecorder = () => {};

export function setAuditRecorder(next: AuditRecorder) {
  recorder = next ?? (() => {});
}

export function recordAuditEvent(event: AuditEvent) {
  recorder(event);
}

export function buildAuditEvent(action: string, actor: { subject: string; roles: string[] }, allowed: boolean): AuditEvent {
  return {
    action,
    subject: actor.subject,
    roles: [...actor.roles],
    allowed,
  };
}

