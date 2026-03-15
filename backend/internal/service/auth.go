package service

import (
	"context"
	"errors"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
	"github.com/jonathanhu237/rota/backend/internal/token"
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

type tokenManager interface {
	IssueAccessToken(identity token.Identity) (string, int64, error)
	ParseAccessToken(accessToken string) (*token.Identity, error)
}

type AuthService struct {
	userRepo     authUserRepository
	tokenManager tokenManager
}

type LoginResult struct {
	AccessToken string
	ExpiresIn   int64
	User        *model.User
}

func NewAuthService(userRepo authUserRepository, tokenManager tokenManager) *AuthService {
	return &AuthService{
		userRepo:     userRepo,
		tokenManager: tokenManager,
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

	accessToken, expiresIn, err := s.tokenManager.IssueAccessToken(token.Identity{
		UserID:  user.ID,
		IsAdmin: user.IsAdmin,
	})
	if err != nil {
		return nil, err
	}

	return &LoginResult{
		AccessToken: accessToken,
		ExpiresIn:   expiresIn,
		User:        user,
	}, nil
}

func (s *AuthService) Authenticate(ctx context.Context, accessToken string) (*model.User, error) {
	identity, err := s.tokenManager.ParseAccessToken(accessToken)
	if err != nil {
		return nil, ErrUnauthorized
	}

	user, err := s.userRepo.GetByID(ctx, identity.UserID)
	if errors.Is(err, repository.ErrUserNotFound) {
		return nil, ErrUnauthorized
	}
	if err != nil {
		return nil, err
	}

	if user.Status == model.UserStatusDisabled {
		return nil, ErrUnauthorized
	}

	return user, nil
}
