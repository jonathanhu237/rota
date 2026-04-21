package handler

import (
	"net/http"

	"github.com/jonathanhu237/rota/backend/internal/audit"
)

// AuditMiddleware wraps an http.Handler so every request receives the
// audit.Recorder and the caller's IP in its context. The authenticated
// user (actor) is attached later by RequireAuth once the session has been
// validated.
func AuditMiddleware(recorder audit.Recorder) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := audit.WithRecorder(r.Context(), recorder)
			ctx = audit.WithActorIP(ctx, clientIPRateLimitKey(r))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
