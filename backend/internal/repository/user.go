package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/lib/pq"
)

var (
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrUserNotFound       = errors.New("user not found")
	ErrVersionConflict    = errors.New("version conflict")
)

type CreateUserParams struct {
	Email        string
	PasswordHash string
	Name         string
	IsAdmin      bool
	Status       model.UserStatus
}

type ListUsersParams struct {
	Offset int
	Limit  int
}

type UpdateUserParams struct {
	ID      int64
	Email   string
	Name    string
	IsAdmin bool
	Version int
}

type UpdateUserStatusParams struct {
	ID      int64
	Status  model.UserStatus
	Version int
}

type UpdateUserPasswordParams struct {
	ID           int64
	PasswordHash string
	Version      int
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
	users, _, err := r.ListPaginated(ctx, ListUsersParams{})
	return users, err
}

func (r *UserRepository) ListPaginated(ctx context.Context, params ListUsersParams) ([]*model.User, int, error) {
	const countQuery = `
		SELECT COUNT(*)
		FROM users;
	`

	var total int
	if err := r.db.QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, err
	}

	const query = `
		SELECT id, email, password_hash, name, is_admin, status, version
		FROM users
		ORDER BY id ASC
		LIMIT $1 OFFSET $2;
	`

	limit := params.Limit
	if limit <= 0 {
		limit = total
		if limit == 0 {
			limit = 1
		}
	}

	rows, err := r.db.QueryContext(ctx, query, limit, params.Offset)
	if err != nil {
		return nil, 0, err
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
			return nil, 0, err
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return users, total, nil
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
		if isUniqueViolation(err) {
			return nil, ErrEmailAlreadyExists
		}
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) Update(ctx context.Context, params UpdateUserParams) (*model.User, error) {
	const query = `
		UPDATE users
		SET email = $2, name = $3, is_admin = $4, version = version + 1
		WHERE id = $1 AND version = $5
		RETURNING id, email, password_hash, name, is_admin, status, version;
	`

	user := &model.User{}
	err := r.db.QueryRowContext(
		ctx,
		query,
		params.ID,
		params.Email,
		params.Name,
		params.IsAdmin,
		params.Version,
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
		switch {
		case isUniqueViolation(err):
			return nil, ErrEmailAlreadyExists
		case errors.Is(err, sql.ErrNoRows):
			return nil, r.resolveWriteConflict(ctx, params.ID)
		default:
			return nil, err
		}
	}

	return user, nil
}

func (r *UserRepository) UpdateStatus(ctx context.Context, params UpdateUserStatusParams) (*model.User, error) {
	const query = `
		UPDATE users
		SET status = $2, version = version + 1
		WHERE id = $1 AND version = $3
		RETURNING id, email, password_hash, name, is_admin, status, version;
	`

	user := &model.User{}
	err := r.db.QueryRowContext(
		ctx,
		query,
		params.ID,
		params.Status,
		params.Version,
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
		if errors.Is(err, sql.ErrNoRows) {
			return nil, r.resolveWriteConflict(ctx, params.ID)
		}
		return nil, err
	}

	return user, nil
}

func (r *UserRepository) UpdatePassword(ctx context.Context, params UpdateUserPasswordParams) (*model.User, error) {
	const query = `
		UPDATE users
		SET password_hash = $2, version = version + 1
		WHERE id = $1 AND version = $3
		RETURNING id, email, password_hash, name, is_admin, status, version;
	`

	user := &model.User{}
	err := r.db.QueryRowContext(
		ctx,
		query,
		params.ID,
		params.PasswordHash,
		params.Version,
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
		if errors.Is(err, sql.ErrNoRows) {
			return nil, r.resolveWriteConflict(ctx, params.ID)
		}
		return nil, err
	}

	return user, nil
}

func (r *UserRepository) resolveWriteConflict(ctx context.Context, id int64) error {
	_, err := r.GetByID(ctx, id)
	switch {
	case errors.Is(err, ErrUserNotFound):
		return ErrUserNotFound
	case err != nil:
		return err
	default:
		return ErrVersionConflict
	}
}

func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505"
}
