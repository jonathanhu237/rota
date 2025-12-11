package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/jonathanhu237/rota/internal/config"
	"github.com/jonathanhu237/rota/internal/domain/user"
	"github.com/jonathanhu237/rota/internal/handler"
	"github.com/jonathanhu237/rota/internal/repository/postgres"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("failed to start the server", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.DB,
	)
	db, err := postgres.NewPool(ctx, dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	userRepo := postgres.NewUserRepository(db)
	if err := ensureAdmin(ctx, cfg, logger, userRepo); err != nil {
		return err
	}

	handler := handler.New()
	srv := http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: handler,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("starting server", "port", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		logger.Info("shutting down server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.ShutdownTimeout)*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("error shutting down server", "error", err)
		}
	}

	return nil
}

func ensureAdmin(ctx context.Context, cfg *config.Config, logger *slog.Logger, userRepo user.Repository) error {
	hasAdmin, err := userRepo.HasAdmin(ctx)
	if err != nil {
		return err
	}
	if hasAdmin {
		return nil
	}

	passwordHashBytes, err := bcrypt.GenerateFromPassword([]byte(cfg.InitAdmin.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	admin := &user.User{
		Username:     cfg.InitAdmin.Username,
		PasswordHash: string(passwordHashBytes),
		Name:         cfg.InitAdmin.Name,
		Email:        cfg.InitAdmin.Email,
		IsAdmin:      true,
		IsActive:     true,
	}

	if err := userRepo.Create(ctx, admin); err != nil {
		return err
	}

	logger.Info("initial admin user created", "username", admin.Username, "name", admin.Name, "email", admin.Email)
	return nil
}
