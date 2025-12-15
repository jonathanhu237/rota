package handler

import (
	"context"
	"net/http"
	"runtime/debug"
	"time"
)

func (h *Handler) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				h.logger.Error("panic recovered",
					"error", err,
					"stack", string(debug.Stack()),
				)
				h.errorResponse(w, http.StatusInternalServerError, ErrCodeInternalServer, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (h *Handler) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rw, r)

		h.logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.status,
			"duration", time.Since(start),
		)
	})
}

func (h *Handler) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("__rota_token")
		if err != nil {
			h.unauthorized(w)
			return
		}

		claims, err := h.jwt.Parse(cookie.Value)
		if err != nil {
			h.unauthorized(w)
			return
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, contextKeyUserID, claims.UserID)
		ctx = context.WithValue(ctx, contextKeyIsAdmin, claims.IsAdmin)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
