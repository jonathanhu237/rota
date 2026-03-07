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
	ErrUserDisabled       = errors.New("user disabled")
)

type authUserRepository interface {
	GetByUsername(ctx context.Context, username string) (*model.User, error)
}

type tokenManager interface {
	IssueAccessToken(identity token.Identity) (string, int64, error)
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

func (s *AuthService) Login(ctx context.Context, username, password string) (*LoginResult, error) {
	user, err := s.userRepo.GetByUsername(ctx, username)
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
		UserID:   user.ID,
		Username: user.Username,
		IsAdmin:  user.IsAdmin,
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
