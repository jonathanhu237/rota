package user

import (
	"context"
	"errors"
	"time"

	"github.com/jonathanhu237/rota/internal/domain/auth"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
)

type Service struct {
	repo Repository
	jwt  *auth.JWT
}

func NewService(repo Repository, jwt *auth.JWT) *Service {
	return &Service{repo: repo, jwt: jwt}
}

func (s *Service) Login(ctx context.Context, username, password string) (string, time.Time, error) {
	user, err := s.repo.GetByUsername(ctx, username)
	if err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound):
			return "", time.Time{}, ErrInvalidCredentials
		default:
			return "", time.Time{}, err
		}
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", time.Time{}, ErrInvalidCredentials
	}

	token, expiresAt, err := s.jwt.Generate(user.ID, user.IsAdmin)
	if err != nil {
		return "", time.Time{}, err
	}

	return token, expiresAt, nil
}

func (s *Service) List(ctx context.Context, page, pageSize int) ([]User, int, error) {
	offset := (page - 1) * pageSize

	users, err := s.repo.List(ctx, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}

	total, err := s.repo.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

func (s *Service) GetByID(ctx context.Context, id string) (*User, error) {
	return s.repo.GetByID(ctx, id)
}
