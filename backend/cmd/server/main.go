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

	"github.com/jonathanhu237/rota/backend/internal/audit"
	"github.com/jonathanhu237/rota/backend/internal/config"
	"github.com/jonathanhu237/rota/backend/internal/email"
	"github.com/jonathanhu237/rota/backend/internal/handler"
	"github.com/jonathanhu237/rota/backend/internal/repository"
	"github.com/jonathanhu237/rota/backend/internal/service"
	_ "github.com/lib/pq"
	"golang.org/x/time/rate"
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
	leaveRepo := repository.NewLeaveRepository(db)
	outboxRepo := repository.NewOutboxRepository(db)
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

	var emailer email.Emailer
	switch cfg.EmailMode {
	case "", "log":
		emailer = email.NewLoggerEmailer(os.Stdout)
	case "smtp":
		emailer, err = email.NewSMTPEmailer(email.SMTPConfig{
			Host:     cfg.SMTPHost,
			Port:     cfg.SMTPPort,
			User:     cfg.SMTPUser,
			Password: cfg.SMTPPassword,
			From:     cfg.SMTPFrom,
			TLSMode:  cfg.SMTPTLSMode,
		})
		if err != nil {
			slog.Error("Failed to initialize SMTP emailer", "error", err)
			os.Exit(1)
		}
		logInsecureSMTPTLSWarning(slog.Default(), cfg.EmailMode, cfg.SMTPTLSMode)
	default:
		slog.Error("Invalid email mode", "email_mode", cfg.EmailMode)
		os.Exit(1)
	}

	sessionExpires := time.Duration(cfg.SessionExpiresHours) * time.Hour
	sessionStore := repository.NewSessionRepository(db, sessionExpires)
	cleanupCtx, stopSessionCleanup := context.WithCancel(context.Background())
	defer stopSessionCleanup()
	startSessionCleanup(cleanupCtx, db, 6*time.Hour, slog.Default())
	var auditRecorder audit.Recorder = repository.NewAuditRecorder(db, slog.Default())
	RunOutboxWorker(audit.WithRecorder(cleanupCtx, auditRecorder), outboxRepo, emailer, slog.Default())

	setupTokenRepo := repository.NewSetupTokenRepository(db)
	setupTxManager := repository.NewSetupTxManager(db)
	authService := service.NewAuthService(
		userRepo,
		sessionStore,
		service.WithAuthSetupFlows(service.SetupFlowConfig{
			TxManager:             setupTxManager,
			SetupTokenRepo:        setupTokenRepo,
			OutboxRepo:            outboxRepo,
			Logger:                slog.Default(),
			AppBaseURL:            cfg.AppBaseURL,
			InvitationTokenTTL:    cfg.InvitationTokenTTL,
			PasswordResetTokenTTL: cfg.PasswordResetTokenTTL,
		}),
	)
	userService := service.NewUserService(
		userRepo,
		sessionStore,
		service.WithSetupFlows(service.SetupFlowConfig{
			TxManager:          setupTxManager,
			OutboxRepo:         outboxRepo,
			Logger:             slog.Default(),
			AppBaseURL:         cfg.AppBaseURL,
			InvitationTokenTTL: cfg.InvitationTokenTTL,
		}),
	)
	positionService := service.NewPositionService(positionRepo)
	templateService := service.NewTemplateService(templateRepo, positionRepo)
	shiftChangeRepo := repository.NewShiftChangeRepository(db)
	publicationService := service.NewPublicationService(
		publicationRepo,
		nil,
		service.WithPublicationShiftChangeNotifications(
			shiftChangeRepo,
			outboxRepo,
			cfg.AppBaseURL,
			slog.Default(),
		),
	)
	userPositionService := service.NewUserPositionService(userPositionRepo)

	// Initialize handlers
	healthHandler := handler.NewHealthHandler()
	authHandler := handler.NewAuthHandler(authService)
	userHandler := handler.NewUserHandler(userService)
	positionHandler := handler.NewPositionHandler(positionService)
	templateHandler := handler.NewTemplateHandler(templateService)
	publicationHandler := handler.NewPublicationHandler(publicationService)
	userPositionHandler := handler.NewUserPositionHandler(userPositionService)
	shiftChangeService := service.NewShiftChangeService(
		shiftChangeRepo,
		publicationRepo,
		outboxRepo,
		cfg.AppBaseURL,
		nil,
		slog.Default(),
	)
	shiftChangeHandler := handler.NewShiftChangeHandler(shiftChangeService)
	leaveService := service.NewLeaveService(
		leaveRepo,
		shiftChangeRepo,
		shiftChangeService,
		publicationRepo,
		nil,
	)
	leaveHandler := handler.NewLeaveHandler(leaveService)
	loginRateLimitByIP := handler.NewRateLimitMiddleware(
		handler.ClientIPRateLimitKey,
		rate.Every(time.Minute/5),
		5,
	)
	loginRateLimitByEmail := handler.NewRateLimitMiddleware(
		handler.LoginEmailRateLimitKey,
		rate.Every(15*time.Minute/10),
		10,
	)
	passwordResetRateLimitByIP := handler.NewRateLimitMiddleware(
		handler.ClientIPRateLimitKey,
		rate.Every(time.Hour/3),
		3,
	)

	// Register routes
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler.HealthCheck)
	mux.HandleFunc(
		"POST /auth/login",
		handler.Chain(authHandler.Login, loginRateLimitByIP, loginRateLimitByEmail),
	)
	mux.HandleFunc(
		"POST /auth/password-reset-request",
		handler.Chain(authHandler.RequestPasswordReset, passwordResetRateLimitByIP),
	)
	mux.HandleFunc("GET /auth/setup-token", authHandler.PreviewSetupToken)
	mux.HandleFunc("POST /auth/setup-password", authHandler.SetupPassword)
	mux.HandleFunc("POST /auth/logout", authHandler.Logout)
	mux.HandleFunc("GET /auth/me", authHandler.RequireAuth(authHandler.Me))
	mux.HandleFunc("GET /users", authHandler.RequireAdmin(userHandler.List))
	mux.HandleFunc("POST /users", authHandler.RequireAdmin(userHandler.Create))
	mux.HandleFunc("GET /users/{id}", authHandler.RequireAdmin(userHandler.GetByID))
	mux.HandleFunc("PUT /users/{id}", authHandler.RequireAdmin(userHandler.Update))
	mux.HandleFunc("POST /users/{id}/resend-invitation", authHandler.RequireAdmin(userHandler.ResendInvitation))
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
	mux.HandleFunc("POST /templates/{id}/slots", authHandler.RequireAdmin(templateHandler.CreateSlot))
	mux.HandleFunc("PATCH /templates/{id}/slots/{slot_id}", authHandler.RequireAdmin(templateHandler.UpdateSlot))
	mux.HandleFunc("DELETE /templates/{id}/slots/{slot_id}", authHandler.RequireAdmin(templateHandler.DeleteSlot))
	mux.HandleFunc("POST /templates/{id}/slots/{slot_id}/positions", authHandler.RequireAdmin(templateHandler.CreateSlotPosition))
	mux.HandleFunc("PATCH /templates/{id}/slots/{slot_id}/positions/{position_entry_id}", authHandler.RequireAdmin(templateHandler.UpdateSlotPosition))
	mux.HandleFunc("DELETE /templates/{id}/slots/{slot_id}/positions/{position_entry_id}", authHandler.RequireAdmin(templateHandler.DeleteSlotPosition))
	mux.HandleFunc("GET /publications", authHandler.RequireAdmin(publicationHandler.List))
	mux.HandleFunc("POST /publications", authHandler.RequireAdmin(publicationHandler.Create))
	mux.HandleFunc("GET /publications/{id}", authHandler.RequireAdmin(publicationHandler.GetByID))
	mux.HandleFunc("PATCH /publications/{id}", authHandler.RequireAdmin(publicationHandler.Update))
	mux.HandleFunc("DELETE /publications/{id}", authHandler.RequireAdmin(publicationHandler.Delete))
	mux.HandleFunc("GET /publications/{id}/assignment-board", authHandler.RequireAdmin(publicationHandler.GetAssignmentBoard))
	mux.HandleFunc("POST /publications/{id}/auto-assign", authHandler.RequireAdmin(publicationHandler.AutoAssign))
	mux.HandleFunc("POST /publications/{id}/assignments", authHandler.RequireAdmin(publicationHandler.CreateAssignment))
	mux.HandleFunc("DELETE /publications/{id}/assignments/{assignment_id}", authHandler.RequireAdmin(publicationHandler.DeleteAssignment))
	mux.HandleFunc("POST /publications/{id}/publish", authHandler.RequireAdmin(publicationHandler.Publish))
	mux.HandleFunc("POST /publications/{id}/activate", authHandler.RequireAdmin(publicationHandler.Activate))
	mux.HandleFunc("POST /publications/{id}/end", authHandler.RequireAdmin(publicationHandler.End))
	mux.HandleFunc("GET /publications/{id}/roster", authHandler.RequireAuth(publicationHandler.GetRoster))
	mux.HandleFunc("GET /publications/current", authHandler.RequireAuth(publicationHandler.GetCurrent))
	mux.HandleFunc("GET /roster/current", authHandler.RequireAuth(publicationHandler.GetCurrentRoster))
	mux.HandleFunc("GET /publications/{id}/submissions/me", authHandler.RequireAuth(publicationHandler.ListMySubmissionSlots))
	mux.HandleFunc("POST /publications/{id}/submissions", authHandler.RequireAuth(publicationHandler.CreateSubmission))
	mux.HandleFunc("DELETE /publications/{id}/submissions/{slot_id}", authHandler.RequireAuth(publicationHandler.DeleteSubmission))
	mux.HandleFunc("GET /publications/{id}/shifts/me", authHandler.RequireAuth(publicationHandler.ListMyQualifiedShifts))
	mux.HandleFunc("POST /publications/{id}/shift-changes", authHandler.RequireAuth(shiftChangeHandler.Create))
	mux.HandleFunc("GET /publications/{id}/shift-changes", authHandler.RequireAuth(shiftChangeHandler.List))
	mux.HandleFunc("GET /publications/{id}/shift-changes/{request_id}", authHandler.RequireAuth(shiftChangeHandler.GetByID))
	mux.HandleFunc("POST /publications/{id}/shift-changes/{request_id}/approve", authHandler.RequireAuth(shiftChangeHandler.Approve))
	mux.HandleFunc("POST /publications/{id}/shift-changes/{request_id}/reject", authHandler.RequireAuth(shiftChangeHandler.Reject))
	mux.HandleFunc("POST /publications/{id}/shift-changes/{request_id}/cancel", authHandler.RequireAuth(shiftChangeHandler.Cancel))
	mux.HandleFunc("GET /publications/{id}/members", authHandler.RequireAuth(shiftChangeHandler.ListMembers))
	mux.HandleFunc("GET /publications/{id}/leaves", authHandler.RequireAdmin(leaveHandler.ListForPublication))
	mux.HandleFunc("POST /leaves", authHandler.RequireAuth(leaveHandler.Create))
	mux.HandleFunc("GET /leaves/{id}", authHandler.RequireAuth(leaveHandler.GetByID))
	mux.HandleFunc("POST /leaves/{id}/cancel", authHandler.RequireAuth(leaveHandler.Cancel))
	mux.HandleFunc("GET /users/me/leaves", authHandler.RequireAuth(leaveHandler.ListMine))
	mux.HandleFunc("GET /users/me/leaves/preview", authHandler.RequireAuth(leaveHandler.PreviewMine))
	mux.HandleFunc("GET /users/me/notifications/unread-count", authHandler.RequireAuth(shiftChangeHandler.UnreadCount))

	// Start server
	addr := fmt.Sprintf(":%d", cfg.ServerPort)
	slog.Info("Server is starting", "addr", addr)
	rootHandler := handler.AuditMiddleware(auditRecorder)(mux)
	if err := http.ListenAndServe(addr, rootHandler); err != nil {
		slog.Error("Failed to start server", "error", err)
		os.Exit(1)
	}
}

func logInsecureSMTPTLSWarning(logger *slog.Logger, emailMode, tlsMode string) {
	if logger == nil || emailMode != "smtp" || tlsMode != "none" {
		return
	}

	logger.Warn("SMTP is configured without TLS — credentials and emails may be visible on the network", "tls_mode", tlsMode)
}

type sessionCleanupDB interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func startSessionCleanup(ctx context.Context, db sessionCleanupDB, interval time.Duration, logger *slog.Logger) {
	if logger == nil {
		logger = slog.Default()
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < NOW() - INTERVAL '1 day';`); err != nil {
					logger.Error("Failed to clean up expired sessions", "error", err)
				}
			}
		}
	}()
}
