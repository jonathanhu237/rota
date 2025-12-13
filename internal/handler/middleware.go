package handler

import (
	"net/http"
	"runtime/debug"
)

func (h *Handler) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				h.logger.Error("panic recovered",
					"error", err,
					"stack", string(debug.Stack()),
				)
				h.errorResponse(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
