package handler

import (
	"log/slog"
	"net/http"

	"github.com/jonathanhu237/rota/internal/domain/user"
)

type Handler struct {
	logger      *slog.Logger
	userService *user.Service
}

func New(logger *slog.Logger, userService *user.Service) *Handler {
	return &Handler{
		logger:      logger,
		userService: userService,
	}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/auth/login", h.login)
	mux.HandleFunc("POST /api/auth/logout", h.logout)

	mux.HandleFunc("GET /api/users", h.listUsers)

	return h.recoverer(mux)
}
