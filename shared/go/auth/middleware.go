package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/ai-aas/shared-go/errors"
)

type contextKey string

const actorContextKey contextKey = "shared.auth.actor"

// Actor represents the authenticated subject attached to a request.
type Actor struct {
	Subject string
	Roles   []string
}

// ActorFromContext extracts the actor from request context.
func ActorFromContext(ctx context.Context) (Actor, bool) {
	actor, ok := ctx.Value(actorContextKey).(Actor)
	return actor, ok
}

// Extractor derives an actor from an HTTP request.
type Extractor func(*http.Request) Actor

// HeaderExtractor reads actor information from X-Actor-* headers.
func HeaderExtractor(r *http.Request) Actor {
	subject := r.Header.Get("X-Actor-Subject")
	rawRoles := r.Header.Get("X-Actor-Roles")
	var roles []string
	for _, role := range strings.Split(rawRoles, ",") {
		role = strings.TrimSpace(role)
		if role != "" {
			roles = append(roles, role)
		}
	}
	return Actor{
		Subject: subject,
		Roles:   roles,
	}
}

// Middleware enforces authorization using the supplied engine.
func Middleware(engine *Engine, extractor Extractor) func(http.Handler) http.Handler {
	if extractor == nil {
		extractor = HeaderExtractor
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor := extractor(r)
			action := r.Method + ":" + r.URL.Path
			allowed := engine.Allowed(action, actor.Roles)
			recordAudit(NewAuditEvent(action, actor, allowed))

			if !allowed {
				resp := errors.New("UNAUTHORIZED", "access denied",
					errors.WithActor(&errors.Actor{
						Subject: actor.Subject,
						Roles:   actor.Roles,
					}),
					errors.WithRequestID(r.Header.Get("X-Request-ID")),
				)
				data, _ := errors.Marshal(resp)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write(data)
				return
			}

			ctx := context.WithValue(r.Context(), actorContextKey, actor)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
