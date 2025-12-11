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

func (r *UserRepository) List(ctx context.Context, limit, offset int) ([]user.User, error) {
	sql := `
		SELECT
			id,
			username,
			password_hash,
			name,
			email,
			is_admin,
			is_active,
			version,
			created_at,
			updated_at
		FROM users
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.Query(ctx, sql, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []user.User{}
	for rows.Next() {
		var user user.User
		dst := []any{
			&user.ID,
			&user.Username,
			&user.PasswordHash,
			&user.Name,
			&user.Email,
			&user.IsAdmin,
			&user.IsActive,
			&user.Version,
			&user.CreatedAt,
			&user.UpdatedAt,
		}
		if err := rows.Scan(dst...); err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

func (r *UserRepository) Count(ctx context.Context) (int, error) {
	sql := `
		SELECT COUNT(*) FROM users
	`

	var count int
	if err := r.db.QueryRow(ctx, sql).Scan(&count); err != nil {
		return 0, err
	}

	return count, nil
}
