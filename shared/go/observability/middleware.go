package observability

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type ctxKey string

const (
	requestIDKey ctxKey = "shared.observability.request_id"
	startTimeKey ctxKey = "shared.observability.start_time"
)

// RequestIDFromContext returns the request ID if present.
func RequestIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(requestIDKey).(string)
	return id, ok
}

// RequestContextMiddleware injects request IDs and timing metadata.
func RequestContextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.NewString()
		}

		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		ctx = context.WithValue(ctx, startTimeKey, time.Now())

		r = r.WithContext(ctx)
		r.Header.Set("X-Request-ID", requestID)
		w.Header().Set("X-Request-ID", requestID)

		next.ServeHTTP(w, r)
	})
}
