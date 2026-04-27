package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/jonathanhu237/rota/backend/internal/audit"
	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials     = errors.New("invalid credentials")
	ErrInvalidCurrentPassword = errors.New("invalid current password")
	ErrUnauthorized           = errors.New("unauthorized")
	ErrUserPending            = errors.New("user pending")
	ErrUserDisabled           = errors.New("user disabled")
)

type authUserRepository interface {
	GetByID(ctx context.Context, id int64) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
}

type sessionStore interface {
	Create(ctx context.Context, userID int64) (string, int64, error)
	Get(ctx context.Context, sessionID string) (int64, error)
	Refresh(ctx context.Context, sessionID string) (int64, error)
	Delete(ctx context.Context, sessionID string) error
	DeleteUserSessions(ctx context.Context, userID int64) error
}

type authPasswordTxRunner interface {
	WithinTx(
		ctx context.Context,
		fn func(
			ctx context.Context,
			userRepo repository.AuthPasswordUserRepository,
			sessionRepo repository.AuthPasswordSessionRepository,
		) error,
	) error
}

type AuthService struct {
	userRepo         authUserRepository
	sessionStore     sessionStore
	setupFlows       *setupFlowHelper
	passwordTxRunner authPasswordTxRunner
}

type AuthServiceOption func(*AuthService)

type LoginResult struct {
	SessionID string
	ExpiresIn int64
	User      *model.User
}

func WithAuthSetupFlows(config SetupFlowConfig) AuthServiceOption {
	return func(service *AuthService) {
		service.setupFlows = newSetupFlowHelper(config)
	}
}

func WithAuthPasswordTxRunner(txRunner authPasswordTxRunner) AuthServiceOption {
	return func(service *AuthService) {
		service.passwordTxRunner = txRunner
	}
}

func NewAuthService(userRepo authUserRepository, sessionStore sessionStore, opts ...AuthServiceOption) *AuthService {
	service := &AuthService{
		userRepo:     userRepo,
		sessionStore: sessionStore,
	}

	for _, opt := range opts {
		opt(service)
	}

	return service
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*LoginResult, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if errors.Is(err, repository.ErrUserNotFound) {
		recordLoginFailure(ctx, email, "invalid_credentials")
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}

	if user.Status == model.UserStatusDisabled {
		recordLoginFailure(ctx, email, "user_disabled")
		return nil, ErrUserDisabled
	}
	if user.Status == model.UserStatusPending {
		recordLoginFailure(ctx, email, "user_pending")
		return nil, ErrUserPending
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		recordLoginFailure(ctx, email, "invalid_credentials")
		return nil, ErrInvalidCredentials
	}

	sessionID, expiresIn, err := s.sessionStore.Create(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	targetID := user.ID
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionAuthLoginSuccess,
		TargetType: audit.TargetTypeUser,
		TargetID:   &targetID,
		Metadata: map[string]any{
			"email": user.Email,
		},
	})

	return &LoginResult{
		SessionID: sessionID,
		ExpiresIn: expiresIn,
		User:      user,
	}, nil
}

// recordLoginFailure emits an audit event for a failed login attempt. The
// actor is intentionally unauthenticated: login failures are captured before
// any session is established.
func recordLoginFailure(ctx context.Context, email, reason string) {
	audit.Record(ctx, audit.Event{
		Action: audit.ActionAuthLoginFailure,
		Metadata: map[string]any{
			"email":  email,
			"reason": reason,
		},
	})
}

func (s *AuthService) RequestPasswordReset(ctx context.Context, emailAddress string) error {
	if s.setupFlows == nil || s.setupFlows.txManager == nil {
		return ErrInvalidInput
	}

	normalizedEmail := strings.TrimSpace(emailAddress)
	if normalizedEmail == "" {
		return nil
	}

	var user *model.User
	var rawToken string
	err := s.setupFlows.txManager.WithinTx(ctx, func(
		ctx context.Context,
		tx *sql.Tx,
		txUserRepo repository.SetupUserRepository,
		txTokenRepo repository.SetupTokenRepositoryWriter,
	) error {
		var err error
		user, err = txUserRepo.GetByEmail(ctx, normalizedEmail)
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if user.Status != model.UserStatusActive {
			return nil
		}

		rawToken, err = s.setupFlows.issueToken(
			ctx,
			txTokenRepo,
			user.ID,
			model.SetupTokenPurposePasswordReset,
			s.setupFlows.passwordResetTokenTTL,
		)
		if err != nil {
			return err
		}
		return s.setupFlows.enqueuePasswordResetTx(ctx, tx, user, rawToken)
	})
	if err != nil {
		return err
	}

	// user_found reflects whether an eligible (active) user exists for the
	// email. It stays server-side — admins with DB access already have this
	// information, and it is essential for detecting probing behaviour.
	audit.Record(ctx, audit.Event{
		Action: audit.ActionAuthPasswordResetRequest,
		Metadata: map[string]any{
			"email":      normalizedEmail,
			"user_found": rawToken != "",
		},
	})
	return nil
}

func (s *AuthService) PreviewSetupToken(ctx context.Context, rawToken string) (*SetupTokenPreview, error) {
	if s.setupFlows == nil || s.setupFlows.setupTokenRepo == nil {
		return nil, ErrInvalidInput
	}

	token, _, err := s.setupFlows.resolvePasswordSetupToken(ctx, s.setupFlows.setupTokenRepo, rawToken)
	if err != nil {
		return nil, err
	}

	user, err := s.userRepo.GetByID(ctx, token.UserID)
	if err != nil {
		return nil, mapRepositoryError(err)
	}

	return &SetupTokenPreview{
		Email:   user.Email,
		Name:    user.Name,
		Purpose: token.Purpose,
	}, nil
}

func (s *AuthService) SetupPassword(ctx context.Context, input SetupPasswordInput) error {
	if s.setupFlows == nil || s.setupFlows.txManager == nil {
		return ErrInvalidInput
	}

	var activatedToken *model.SetupToken
	if err := s.setupFlows.txManager.WithinTx(ctx, func(
		ctx context.Context,
		tx *sql.Tx,
		txUserRepo repository.SetupUserRepository,
		txTokenRepo repository.SetupTokenRepositoryWriter,
	) error {
		token, err := s.setupFlows.activatePassword(ctx, txUserRepo, txTokenRepo, input)
		if err != nil {
			return err
		}
		activatedToken = token
		return nil
	}); err != nil {
		return err
	}

	purpose := "invitation"
	if activatedToken.Purpose == model.SetupTokenPurposePasswordReset {
		purpose = "password_reset"
		if s.sessionStore != nil {
			if err := s.sessionStore.DeleteUserSessions(ctx, activatedToken.UserID); err != nil {
				return err
			}
		}
	}
	targetID := activatedToken.UserID
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionAuthPasswordSet,
		TargetType: audit.TargetTypeUser,
		TargetID:   &targetID,
		Metadata: map[string]any{
			"purpose": purpose,
		},
	})
	return nil
}

func (s *AuthService) ConfirmEmailChange(ctx context.Context, rawToken string) error {
	if s.setupFlows == nil || s.setupFlows.txManager == nil {
		return ErrInvalidInput
	}

	var (
		confirmedToken *model.SetupToken
		newEmail       string
		revokedCount   int
		commitError    error
	)
	err := s.setupFlows.txManager.WithinTx(ctx, func(
		ctx context.Context,
		tx *sql.Tx,
		txUserRepo repository.SetupUserRepository,
		txTokenRepo repository.SetupTokenRepositoryWriter,
	) error {
		token, _, err := s.setupFlows.resolveEmailChangeToken(ctx, txTokenRepo, rawToken)
		if err != nil {
			return err
		}
		if token.NewEmail == nil || *token.NewEmail == "" {
			return model.ErrTokenNotFound
		}

		now := s.setupFlows.clock()
		if err := txTokenRepo.MarkUsed(ctx, token.ID, now); err != nil {
			return err
		}

		existing, err := txUserRepo.GetByEmail(ctx, *token.NewEmail)
		switch {
		case errors.Is(err, repository.ErrUserNotFound):
		case err != nil:
			return err
		case existing.ID != token.UserID:
			commitError = ErrEmailAlreadyExists
			confirmedToken = token
			newEmail = *token.NewEmail
			return nil
		}

		if _, err := txUserRepo.UpdateEmail(ctx, token.UserID, *token.NewEmail); err != nil {
			return mapRepositoryError(err)
		}

		count, err := s.setupFlows.sessionRepoFactory(tx).DeleteAllSessions(ctx, token.UserID)
		if err != nil {
			return err
		}
		if err := txTokenRepo.InvalidateAllUnusedTokensExcept(ctx, token.UserID, token.ID, now); err != nil {
			return err
		}

		confirmedToken = token
		newEmail = *token.NewEmail
		revokedCount = count
		return nil
	})
	if err != nil {
		return err
	}
	if commitError != nil {
		return commitError
	}
	if confirmedToken == nil {
		return ErrInvalidInput
	}

	targetID := confirmedToken.UserID
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionUserEmailChangeConfirm,
		TargetType: audit.TargetTypeUser,
		TargetID:   &targetID,
		Metadata: map[string]any{
			"user_id":               confirmedToken.UserID,
			"new_email_normalized":  newEmail,
			"revoked_session_count": revokedCount,
		},
	})
	return nil
}

func (s *AuthService) ChangeOwnPassword(
	ctx context.Context,
	viewerID int64,
	currentSessionID string,
	currentPassword string,
	newPassword string,
) (int, error) {
	if viewerID <= 0 || currentSessionID == "" || s.passwordTxRunner == nil {
		return 0, ErrInvalidInput
	}

	revokedCount := 0
	err := s.passwordTxRunner.WithinTx(ctx, func(
		ctx context.Context,
		userRepo repository.AuthPasswordUserRepository,
		sessionRepo repository.AuthPasswordSessionRepository,
	) error {
		user, err := userRepo.GetByIDForUpdate(ctx, viewerID)
		if err != nil {
			return mapRepositoryError(err)
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
			return ErrInvalidCurrentPassword
		}
		if err := model.ValidatePassword(newPassword); err != nil {
			return err
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		if _, err := userRepo.UpdatePasswordByID(ctx, viewerID, string(hash)); err != nil {
			return mapRepositoryError(err)
		}

		count, err := sessionRepo.DeleteOtherSessions(ctx, viewerID, currentSessionID)
		if err != nil {
			return err
		}
		revokedCount = count
		return nil
	})
	if err != nil {
		return 0, err
	}

	targetID := viewerID
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionAuthPasswordChange,
		TargetType: audit.TargetTypeUser,
		TargetID:   &targetID,
		Metadata: map[string]any{
			"user_id":               viewerID,
			"revoked_session_count": revokedCount,
		},
	})
	return revokedCount, nil
}

// AuthenticateResult holds the authenticated user and the refreshed session TTL.
type AuthenticateResult struct {
	User      *model.User
	ExpiresIn int64
}

func (s *AuthService) Authenticate(ctx context.Context, sessionID string) (*AuthenticateResult, error) {
	userID, err := s.sessionStore.Get(ctx, sessionID)
	if errors.Is(err, repository.ErrSessionNotFound) {
		return nil, ErrUnauthorized
	}
	if err != nil {
		return nil, err
	}

	expiresIn, err := s.sessionStore.Refresh(ctx, sessionID)
	if errors.Is(err, repository.ErrSessionNotFound) {
		return nil, ErrUnauthorized
	}
	if err != nil {
		return nil, err
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if errors.Is(err, repository.ErrUserNotFound) {
		return nil, ErrUnauthorized
	}
	if err != nil {
		return nil, err
	}

	if user.Status == model.UserStatusDisabled {
		return nil, ErrUnauthorized
	}

	return &AuthenticateResult{
		User:      user,
		ExpiresIn: expiresIn,
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, sessionID string) error {
	if err := s.sessionStore.Delete(ctx, sessionID); err != nil {
		return err
	}

	event := audit.Event{
		Action: audit.ActionAuthLogout,
	}
	if actorID, ok := audit.ActorFromContext(ctx); ok {
		event.TargetType = audit.TargetTypeUser
		event.TargetID = &actorID
	}
	audit.Record(ctx, event)
	return nil
}
