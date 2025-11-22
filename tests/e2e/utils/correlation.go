package utils

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// CorrelationIDs holds correlation identifiers
type CorrelationIDs struct {
	RequestID string
	TraceID   string
	SpanID    string
}

// GenerateCorrelationIDs generates new correlation IDs
func GenerateCorrelationIDs() *CorrelationIDs {
	return &CorrelationIDs{
		RequestID: uuid.New().String(),
		TraceID:   uuid.New().String(),
		SpanID:    uuid.New().String()[:16], // 16 chars for span ID
	}
}

// ExtractCorrelationIDs extracts correlation IDs from HTTP headers
func ExtractCorrelationIDs(headers http.Header) *CorrelationIDs {
	ids := &CorrelationIDs{}

	if requestID := headers.Get("X-Request-ID"); requestID != "" {
		ids.RequestID = requestID
	}
	if traceID := headers.Get("X-Trace-ID"); traceID != "" {
		ids.TraceID = traceID
	}
	if spanID := headers.Get("X-Span-ID"); spanID != "" {
		ids.SpanID = spanID
	}

	return ids
}

// SetCorrelationHeaders sets correlation IDs in HTTP headers
func SetCorrelationHeaders(headers http.Header, ids *CorrelationIDs) {
	if ids.RequestID != "" {
		headers.Set("X-Request-ID", ids.RequestID)
	}
	if ids.TraceID != "" {
		headers.Set("X-Trace-ID", ids.TraceID)
	}
	if ids.SpanID != "" {
		headers.Set("X-Span-ID", ids.SpanID)
	}
}

// PropagateCorrelationIDs propagates correlation IDs from request to response
func PropagateCorrelationIDs(reqHeaders, respHeaders http.Header) {
	// Extract from request
	ids := ExtractCorrelationIDs(reqHeaders)

	// Set in response if not already present
	if respHeaders.Get("X-Request-ID") == "" && ids.RequestID != "" {
		respHeaders.Set("X-Request-ID", ids.RequestID)
	}
	if respHeaders.Get("X-Trace-ID") == "" && ids.TraceID != "" {
		respHeaders.Set("X-Trace-ID", ids.TraceID)
	}
	if respHeaders.Get("X-Span-ID") == "" && ids.SpanID != "" {
		respHeaders.Set("X-Span-ID", ids.SpanID)
	}
}

// FormatCorrelationString formats correlation IDs as a string for logging
func FormatCorrelationString(ids *CorrelationIDs) string {
	parts := []string{}
	if ids.RequestID != "" {
		parts = append(parts, fmt.Sprintf("req=%s", ids.RequestID))
	}
	if ids.TraceID != "" {
		parts = append(parts, fmt.Sprintf("trace=%s", ids.TraceID))
	}
	if ids.SpanID != "" {
		parts = append(parts, fmt.Sprintf("span=%s", ids.SpanID))
	}
	return strings.Join(parts, " ")
}

