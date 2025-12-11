package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonathanhu237/rota/internal/domain/user"
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, u *user.User) error {
	sql := `
		INSERT INTO users (username, password_hash, name, email, is_admin, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at, version
	`

	args := []any{
		u.Username,
		u.PasswordHash,
		u.Name,
		u.Email,
		u.IsAdmin,
		u.IsActive,
	}

	dst := []any{
		&u.ID,
		&u.CreatedAt,
		&u.UpdatedAt,
		&u.Version,
	}

	return r.db.QueryRow(ctx, sql, args...).Scan(dst...)
}

func (r *UserRepository) HasAdmin(ctx context.Context) (bool, error) {
	sql := `
		SELECT EXISTS(SELECT 1 FROM users WHERE is_admin = TRUE AND is_active = TRUE)
	`

	var exists bool
	err := r.db.QueryRow(ctx, sql).Scan(&exists)
	return exists, err
}
