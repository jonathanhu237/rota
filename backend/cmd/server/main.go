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
	"github.com/jonathanhu237/rota/backend/internal/session"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
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
	positionRepo := repository.NewPositionRepository(db)
	templateRepo := repository.NewTemplateRepository(db)
	publicationRepo := repository.NewPublicationRepository(db)
	userPositionRepo := repository.NewUserPositionRepository(db)
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

	// Initialize Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.RedisHost, cfg.RedisPort),
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	defer redisClient.Close()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		slog.Error("Failed to connect Redis", "error", err)
		os.Exit(1)
	}

	sessionStore := session.NewStore(
		redisClient,
		time.Duration(cfg.SessionExpiresHours)*time.Hour,
	)
	authService := service.NewAuthService(userRepo, sessionStore)
	userService := service.NewUserService(userRepo, sessionStore)
	positionService := service.NewPositionService(positionRepo)
	templateService := service.NewTemplateService(templateRepo, positionRepo)
	publicationService := service.NewPublicationService(publicationRepo, nil)
	userPositionService := service.NewUserPositionService(userPositionRepo)

	// Initialize handlers
	healthHandler := handler.NewHealthHandler()
	authHandler := handler.NewAuthHandler(authService)
	userHandler := handler.NewUserHandler(userService)
	positionHandler := handler.NewPositionHandler(positionService)
	templateHandler := handler.NewTemplateHandler(templateService)
	publicationHandler := handler.NewPublicationHandler(publicationService)
	userPositionHandler := handler.NewUserPositionHandler(userPositionService)

	// Register routes
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler.HealthCheck)
	mux.HandleFunc("POST /auth/login", authHandler.Login)
	mux.HandleFunc("POST /auth/logout", authHandler.Logout)
	mux.HandleFunc("GET /auth/me", authHandler.RequireAuth(authHandler.Me))
	mux.HandleFunc("GET /users", authHandler.RequireAdmin(userHandler.List))
	mux.HandleFunc("POST /users", authHandler.RequireAdmin(userHandler.Create))
	mux.HandleFunc("GET /users/{id}", authHandler.RequireAdmin(userHandler.GetByID))
	mux.HandleFunc("PUT /users/{id}", authHandler.RequireAdmin(userHandler.Update))
	mux.HandleFunc("PATCH /users/{id}/password", authHandler.RequireAdmin(userHandler.UpdatePassword))
	mux.HandleFunc("PATCH /users/{id}/status", authHandler.RequireAdmin(userHandler.UpdateStatus))
	mux.HandleFunc("GET /users/{id}/positions", authHandler.RequireAdmin(userPositionHandler.List))
	mux.HandleFunc("PUT /users/{id}/positions", authHandler.RequireAdmin(userPositionHandler.Replace))
	mux.HandleFunc("GET /positions", authHandler.RequireAdmin(positionHandler.List))
	mux.HandleFunc("POST /positions", authHandler.RequireAdmin(positionHandler.Create))
	mux.HandleFunc("GET /positions/{id}", authHandler.RequireAdmin(positionHandler.GetByID))
	mux.HandleFunc("PUT /positions/{id}", authHandler.RequireAdmin(positionHandler.Update))
	mux.HandleFunc("DELETE /positions/{id}", authHandler.RequireAdmin(positionHandler.Delete))
	mux.HandleFunc("GET /templates", authHandler.RequireAdmin(templateHandler.List))
	mux.HandleFunc("POST /templates", authHandler.RequireAdmin(templateHandler.Create))
	mux.HandleFunc("GET /templates/{id}", authHandler.RequireAdmin(templateHandler.GetByID))
	mux.HandleFunc("PUT /templates/{id}", authHandler.RequireAdmin(templateHandler.Update))
	mux.HandleFunc("DELETE /templates/{id}", authHandler.RequireAdmin(templateHandler.Delete))
	mux.HandleFunc("POST /templates/{id}/clone", authHandler.RequireAdmin(templateHandler.Clone))
	mux.HandleFunc("POST /templates/{id}/shifts", authHandler.RequireAdmin(templateHandler.CreateShift))
	mux.HandleFunc("PATCH /templates/{id}/shifts/{shift_id}", authHandler.RequireAdmin(templateHandler.UpdateShift))
	mux.HandleFunc("DELETE /templates/{id}/shifts/{shift_id}", authHandler.RequireAdmin(templateHandler.DeleteShift))
	mux.HandleFunc("GET /publications", authHandler.RequireAdmin(publicationHandler.List))
	mux.HandleFunc("POST /publications", authHandler.RequireAdmin(publicationHandler.Create))
	mux.HandleFunc("GET /publications/{id}", authHandler.RequireAdmin(publicationHandler.GetByID))
	mux.HandleFunc("DELETE /publications/{id}", authHandler.RequireAdmin(publicationHandler.Delete))
	mux.HandleFunc("GET /publications/{id}/assignment-board", authHandler.RequireAdmin(publicationHandler.GetAssignmentBoard))
	mux.HandleFunc("POST /publications/{id}/assignments", authHandler.RequireAdmin(publicationHandler.CreateAssignment))
	mux.HandleFunc("DELETE /publications/{id}/assignments/{assignment_id}", authHandler.RequireAdmin(publicationHandler.DeleteAssignment))
	mux.HandleFunc("POST /publications/{id}/activate", authHandler.RequireAdmin(publicationHandler.Activate))
	mux.HandleFunc("POST /publications/{id}/end", authHandler.RequireAdmin(publicationHandler.End))
	mux.HandleFunc("GET /publications/{id}/roster", authHandler.RequireAuth(publicationHandler.GetRoster))
	mux.HandleFunc("GET /publications/current", authHandler.RequireAuth(publicationHandler.GetCurrent))
	mux.HandleFunc("GET /roster/current", authHandler.RequireAuth(publicationHandler.GetCurrentRoster))
	mux.HandleFunc("GET /publications/{id}/submissions/me", authHandler.RequireAuth(publicationHandler.ListMySubmissionShiftIDs))
	mux.HandleFunc("POST /publications/{id}/submissions", authHandler.RequireAuth(publicationHandler.CreateSubmission))
	mux.HandleFunc("DELETE /publications/{id}/submissions/{shift_id}", authHandler.RequireAuth(publicationHandler.DeleteSubmission))
	mux.HandleFunc("GET /publications/{id}/shifts/me", authHandler.RequireAuth(publicationHandler.ListMyQualifiedShifts))

	// Start server
	addr := fmt.Sprintf(":%d", cfg.ServerPort)
	slog.Info("Server is starting", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("Failed to start server", "error", err)
		os.Exit(1)
	}
}
