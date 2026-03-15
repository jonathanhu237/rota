package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/config"
	"github.com/jonathanhu237/rota/backend/internal/handler"
	"github.com/jonathanhu237/rota/backend/internal/repository"
	"github.com/jonathanhu237/rota/backend/internal/service"
	"github.com/jonathanhu237/rota/backend/internal/token"
	_ "github.com/lib/pq"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	db, err := sql.Open("postgres", cfg.DatabaseDSN())
	if err != nil {
		slog.Error("Failed to initialize database connection", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		slog.Error("Failed to connect database", "error", err)
		os.Exit(1)
	}

	userRepo := repository.NewUserRepository(db)
	if err := service.EnsureBootstrapAdmin(ctx, service.BootstrapAdminInput{
		Email:    cfg.BootstrapAdminEmail,
		Password: cfg.BootstrapAdminPassword,
		Name:     cfg.BootstrapAdminName,
	}, userRepo); err != nil {
		switch {
		case errors.Is(err, service.ErrConfigInvalid):
			slog.Error("Invalid bootstrap admin configuration", "error", err)
		default:
			slog.Error("Failed to bootstrap admin user", "error", err)
		}
		os.Exit(1)
	}

	accessManager := token.NewAccessTokenManager(
		cfg.JWTSecret,
		time.Duration(cfg.JWTExpiresMinutes)*time.Minute,
	)
	authService := service.NewAuthService(userRepo, accessManager)

	// Initialize handlers
	healthHandler := handler.NewHealthHandler()
	authHandler := handler.NewAuthHandler(authService)
	userHandler := handler.NewUserHandler(userRepo)

	// Register routes
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler.HealthCheck)
	mux.HandleFunc("POST /auth/login", authHandler.Login)
	mux.HandleFunc("POST /auth/logout", authHandler.Logout)
	mux.HandleFunc("GET /auth/me", authHandler.RequireAuth(authHandler.Me))
	mux.HandleFunc("GET /users", authHandler.RequireAdmin(userHandler.List))

	// Start server
	addr := fmt.Sprintf(":%d", cfg.ServerPort)
	slog.Info("Server is starting", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("Failed to start server", "error", err)
		os.Exit(1)
	}
}
