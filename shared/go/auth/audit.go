package auth

import "sync/atomic"

// AuditEvent captures a single authorization decision.
type AuditEvent struct {
	Action  string   `json:"action"`
	Subject string   `json:"subject"`
	Roles   []string `json:"roles"`
	Allowed bool     `json:"allowed"`
}

// AuditRecorder records audit events. It is configurable via SetAuditRecorder.
type AuditRecorder func(AuditEvent)

var auditRecorder atomic.Value

func init() {
	auditRecorder.Store(AuditRecorder(func(AuditEvent) {}))
}

// SetAuditRecorder overrides the default audit recorder.
func SetAuditRecorder(recorder AuditRecorder) {
	if recorder == nil {
		recorder = func(AuditEvent) {}
	}
	auditRecorder.Store(recorder)
}

func recordAudit(event AuditEvent) {
	rec := auditRecorder.Load().(AuditRecorder)
	rec(event)
}

// NewAuditEvent constructs an audit event from the supplied action and actor.
func NewAuditEvent(action string, actor Actor, allowed bool) AuditEvent {
	return AuditEvent{
		Action:  action,
		Subject: actor.Subject,
		Roles:   append([]string(nil), actor.Roles...),
		Allowed: allowed,
	}
}
