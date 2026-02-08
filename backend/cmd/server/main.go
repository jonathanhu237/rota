package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/jonathanhu237/rota/backend/internal/config"
	"github.com/jonathanhu237/rota/backend/internal/handler"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Initialize handlers
	healthHandler := handler.NewHealthHandler()

	// Register routes
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler.HealthCheck)

	// Start server
	addr := fmt.Sprintf(":%d", cfg.ServerPort)
	slog.Info("Server is starting", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("Failed to start server", "error", err)
		os.Exit(1)
	}
}
