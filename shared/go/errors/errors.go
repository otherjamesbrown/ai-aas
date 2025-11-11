package errors

import (
	"encoding/json"
	"fmt"
	"time"
)

// Actor contains authenticated subject metadata used in error payloads.
type Actor struct {
	Subject string   `json:"subject"`
	Roles   []string `json:"roles"`
}

// Error represents the standardized error schema shared across services.
type Error struct {
	Message   string    `json:"error"`
	Code      string    `json:"code"`
	Detail    string    `json:"detail,omitempty"`
	RequestID string    `json:"request_id,omitempty"`
	TraceID   string    `json:"trace_id,omitempty"`
	Actor     *Actor    `json:"actor,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// Option mutates an Error during construction.
type Option func(*Error)

// New constructs a new shared Error with the provided code and message.
func New(code, message string, opts ...Option) *Error {
	err := &Error{
		Message:   message,
		Code:      code,
		Timestamp: time.Now().UTC(),
	}
	for _, opt := range opts {
		opt(err)
	}
	return err
}

// Error satisfies the error interface.
func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// WithDetail attaches a detail string.
func WithDetail(detail string) Option {
	return func(e *Error) {
		e.Detail = detail
	}
}

// WithRequestID attaches a request ID.
func WithRequestID(id string) Option {
	return func(e *Error) {
		e.RequestID = id
	}
}

// WithTraceID attaches a trace ID.
func WithTraceID(id string) Option {
	return func(e *Error) {
		e.TraceID = id
	}
}

// WithActor attaches actor metadata.
func WithActor(actor *Actor) Option {
	return func(e *Error) {
		e.Actor = actor
	}
}

// WithTimestamp overrides the default timestamp.
func WithTimestamp(ts time.Time) Option {
	return func(e *Error) {
		e.Timestamp = ts.UTC()
	}
}

// From attempts to coerce any error into a shared Error.
// Non-shared errors are wrapped with a generic InternalError code.
func From(err error) *Error {
	if err == nil {
		return nil
	}
	if shared, ok := err.(*Error); ok {
		return shared
	}
	return New("INTERNAL", "unexpected error occurred", WithDetail(err.Error()))
}

// Marshal converts an error into a JSON byte slice following the shared schema.
func Marshal(err error) ([]byte, error) {
	return json.Marshal(From(err))
}
