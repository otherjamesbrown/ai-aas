export interface AuditEvent {
    action: string;
    subject: string;
    roles: string[];
    allowed: boolean;
}
export type AuditRecorder = (event: AuditEvent) => void;
export declare function setAuditRecorder(next: AuditRecorder): void;
export declare function recordAuditEvent(event: AuditEvent): void;
export declare function buildAuditEvent(action: string, actor: {
    subject: string;
    roles: string[];
}, allowed: boolean): AuditEvent;
