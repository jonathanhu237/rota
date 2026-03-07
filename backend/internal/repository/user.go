package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jonathanhu237/rota/backend/internal/model"
)

var ErrUserNotFound = errors.New("user not found")

type CreateUserParams struct {
	Username     string
	PasswordHash string
	Name         string
	IsAdmin      bool
	Status       model.UserStatus
}

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	const query = `
		SELECT id, username, password_hash, name, is_admin, status, version
		FROM users
		WHERE username = $1;
	`

	user := &model.User{}
	err := r.db.QueryRowContext(ctx, query, username).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.Name,
		&user.IsAdmin,
		&user.Status,
		&user.Version,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) CountAdmins(ctx context.Context) (int, error) {
	const query = `
		SELECT COUNT(*)
		FROM users
		WHERE is_admin = TRUE;
	`

	var count int
	if err := r.db.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *UserRepository) Create(ctx context.Context, params CreateUserParams) (*model.User, error) {
	const query = `
		INSERT INTO users (username, password_hash, name, is_admin, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, username, password_hash, name, is_admin, status, version;
	`

	user := &model.User{}
	err := r.db.QueryRowContext(
		ctx,
		query,
		params.Username,
		params.PasswordHash,
		params.Name,
		params.IsAdmin,
		params.Status,
	).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.Name,
		&user.IsAdmin,
		&user.Status,
		&user.Version,
	)
	if err != nil {
		return nil, err
	}
	return user, nil
}
