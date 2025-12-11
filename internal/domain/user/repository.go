package user

import "context"

type Repository interface {
	Create(ctx context.Context, user *User) error
	HasAdmin(ctx context.Context) (bool, error)
	List(ctx context.Context, limit, offset int) ([]User, error)
	Count(ctx context.Context) (int, error)
}
