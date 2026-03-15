package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jonathanhu237/rota/backend/internal/model"
)

var ErrUserNotFound = errors.New("user not found")

type CreateUserParams struct {
	Email        string
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

func (r *UserRepository) GetByID(ctx context.Context, id int64) (*model.User, error) {
	const query = `
		SELECT id, email, password_hash, name, is_admin, status, version
		FROM users
		WHERE id = $1;
	`

	user := &model.User{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
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

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	const query = `
		SELECT id, email, password_hash, name, is_admin, status, version
		FROM users
		WHERE email = $1;
	`

	user := &model.User{}
	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
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

func (r *UserRepository) List(ctx context.Context) ([]*model.User, error) {
	const query = `
		SELECT id, email, password_hash, name, is_admin, status, version
		FROM users
		ORDER BY id ASC;
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]*model.User, 0)
	for rows.Next() {
		user := &model.User{}
		if err := rows.Scan(
			&user.ID,
			&user.Email,
			&user.PasswordHash,
			&user.Name,
			&user.IsAdmin,
			&user.Status,
			&user.Version,
		); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
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
		INSERT INTO users (email, password_hash, name, is_admin, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, email, password_hash, name, is_admin, status, version;
	`

	user := &model.User{}
	err := r.db.QueryRowContext(
		ctx,
		query,
		params.Email,
		params.PasswordHash,
		params.Name,
		params.IsAdmin,
		params.Status,
	).Scan(
		&user.ID,
		&user.Email,
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
