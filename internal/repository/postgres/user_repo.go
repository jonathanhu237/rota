package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
		var u user.User
		dst := []any{
			&u.ID,
			&u.Username,
			&u.PasswordHash,
			&u.Name,
			&u.Email,
			&u.IsAdmin,
			&u.IsActive,
			&u.Version,
			&u.CreatedAt,
			&u.UpdatedAt,
		}
		if err := rows.Scan(dst...); err != nil {
			return nil, err
		}
		users = append(users, u)
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

func (r *UserRepository) GetByID(ctx context.Context, id string) (*user.User, error) {
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
		WHERE id = $1
	`

	var u user.User
	dst := []any{
		&u.ID,
		&u.Username,
		&u.PasswordHash,
		&u.Name,
		&u.Email,
		&u.IsAdmin,
		&u.IsActive,
		&u.Version,
		&u.CreatedAt,
		&u.UpdatedAt,
	}
	if err := r.db.QueryRow(ctx, sql, id).Scan(dst...); err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return nil, user.ErrUserNotFound
		default:
			return nil, err
		}
	}

	return &u, nil
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*user.User, error) {
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
		WHERE username = $1
	`

	var u user.User
	dst := []any{
		&u.ID,
		&u.Username,
		&u.PasswordHash,
		&u.Name,
		&u.Email,
		&u.IsAdmin,
		&u.IsActive,
		&u.Version,
		&u.CreatedAt,
		&u.UpdatedAt,
	}
	if err := r.db.QueryRow(ctx, sql, username).Scan(dst...); err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return nil, user.ErrUserNotFound
		default:
			return nil, err
		}
	}

	return &u, nil
}

func (r *UserRepository) userExistsByID(ctx context.Context, id string) (bool, error) {
	sql := `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`

	var exists bool
	if err := r.db.QueryRow(ctx, sql, id).Scan(&exists); err != nil {
		return false, err
	}

	return exists, nil
}

func (r *UserRepository) Update(ctx context.Context, u *user.User) error {
	sql := `
		UPDATE users
		SET
			name = $1,
			email = $2,
			password_hash = $3,
			is_admin = $4,
			is_active = $5,
			version = version + 1
		WHERE id = $6 AND version = $7
		RETURNING version, updated_at
	`

	args := []any{
		u.Name,
		u.Email,
		u.PasswordHash,
		u.IsAdmin,
		u.IsActive,
		u.ID,
		u.Version,
	}

	err := r.db.QueryRow(ctx, sql, args...).Scan(&u.Version, &u.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		switch {
		case errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation && pgErr.ConstraintName == "users_email_key":
			return user.ErrEmailAlreadyExists
		case errors.Is(err, pgx.ErrNoRows):
			// check if user not found or version conflict
			exists, err := r.userExistsByID(ctx, u.ID)
			if err != nil {
				return err
			}
			if exists {
				return user.ErrConcurrentUpdate
			}
			return user.ErrUserNotFound
		default:
			return err
		}
	}

	return nil
}
