package user

import (
	"context"
	"errors"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrConcurrentUpdate  = errors.New("concurrent update detected")
	ErrEmailAlreadyExists = errors.New("email already exists")
)

type Repository interface {
	Create(ctx context.Context, user *User) error
	HasAdmin(ctx context.Context) (bool, error)
	List(ctx context.Context, limit, offset int) ([]User, error)
	Count(ctx context.Context) (int, error)
	GetByID(ctx context.Context, id string) (*User, error)
	GetByUsername(ctx context.Context, username string) (*User, error)
	Update(ctx context.Context, user *User) error
}
