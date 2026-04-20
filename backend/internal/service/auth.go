package service

import (
	"context"
	"errors"
	"strings"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
	"github.com/jonathanhu237/rota/backend/internal/session"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrUserPending        = errors.New("user pending")
	ErrUserDisabled       = errors.New("user disabled")
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
}

type AuthService struct {
	userRepo     authUserRepository
	sessionStore sessionStore
	setupFlows   *setupFlowHelper
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
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}

	if user.Status == model.UserStatusDisabled {
		return nil, ErrUserDisabled
	}
	if user.Status == model.UserStatusPending {
		return nil, ErrUserPending
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	sessionID, expiresIn, err := s.sessionStore.Create(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	return &LoginResult{
		SessionID: sessionID,
		ExpiresIn: expiresIn,
		User:      user,
	}, nil
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
		return err
	})
	if err != nil {
		return err
	}
	if user == nil || rawToken == "" {
		return nil
	}

	s.setupFlows.sendPasswordReset(ctx, user, rawToken)
	return nil
}

func (s *AuthService) PreviewSetupToken(ctx context.Context, rawToken string) (*SetupTokenPreview, error) {
	if s.setupFlows == nil || s.setupFlows.setupTokenRepo == nil {
		return nil, ErrInvalidInput
	}

	token, _, err := s.setupFlows.resolveToken(ctx, s.setupFlows.setupTokenRepo, rawToken)
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

	return s.setupFlows.txManager.WithinTx(ctx, func(
		ctx context.Context,
		txUserRepo repository.SetupUserRepository,
		txTokenRepo repository.SetupTokenRepositoryWriter,
	) error {
		return s.setupFlows.activatePassword(ctx, txUserRepo, txTokenRepo, input)
	})
}

// AuthenticateResult holds the authenticated user and the refreshed session TTL.
type AuthenticateResult struct {
	User      *model.User
	ExpiresIn int64
}

func (s *AuthService) Authenticate(ctx context.Context, sessionID string) (*AuthenticateResult, error) {
	userID, err := s.sessionStore.Get(ctx, sessionID)
	if errors.Is(err, session.ErrSessionNotFound) {
		return nil, ErrUnauthorized
	}
	if err != nil {
		return nil, err
	}

	expiresIn, err := s.sessionStore.Refresh(ctx, sessionID)
	if errors.Is(err, session.ErrSessionNotFound) {
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
	return s.sessionStore.Delete(ctx, sessionID)
}
