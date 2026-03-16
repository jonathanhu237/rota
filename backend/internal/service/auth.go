package service

import (
	"context"
	"errors"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
	"github.com/jonathanhu237/rota/backend/internal/session"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnauthorized       = errors.New("unauthorized")
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
}

type LoginResult struct {
	SessionID string
	ExpiresIn int64
	User      *model.User
}

func NewAuthService(userRepo authUserRepository, sessionStore sessionStore) *AuthService {
	return &AuthService{
		userRepo:     userRepo,
		sessionStore: sessionStore,
	}
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
