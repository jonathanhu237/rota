package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

var ErrConfigInvalid = errors.New("config invalid")

type bootstrapUserRepository interface {
	CountAdmins(ctx context.Context) (int, error)
	Create(ctx context.Context, params repository.CreateUserParams) (*model.User, error)
}

type BootstrapAdminInput struct {
	Email    string
	Password string
	Name     string
}

func EnsureBootstrapAdmin(ctx context.Context, input BootstrapAdminInput, userRepo bootstrapUserRepository) error {
	adminCount, err := userRepo.CountAdmins(ctx)
	if err != nil {
		return err
	}
	if adminCount > 0 {
		return nil
	}

	email := input.Email
	password := input.Password
	name := input.Name
	if email == "" || password == "" || name == "" {
		return fmt.Errorf("%w: bootstrap admin credentials are required", ErrConfigInvalid)
	}
	if err := model.ValidatePassword(password); err != nil {
		return fmt.Errorf("%w: %v", ErrConfigInvalid, err)
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	_, err = userRepo.Create(ctx, repository.CreateUserParams{
		Email:        email,
		PasswordHash: ptr(string(passwordHash)),
		Name:         name,
		IsAdmin:      true,
		Status:       model.UserStatusActive,
	})
	return err
}

func ptr[T any](value T) *T {
	return &value
}
