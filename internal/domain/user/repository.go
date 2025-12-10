package user

import "context"

type Repository interface {
	Create(ctx context.Context, user *User) error
	HasAdmin(ctx context.Context) (bool, error)
}
