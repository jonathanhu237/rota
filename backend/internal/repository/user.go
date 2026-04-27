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
	PasswordHash *string
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

type NullableStringField struct {
	Set   bool
	Value *string
}

type UpdateOwnProfileParams struct {
	ID                 int64
	Name               NullableStringField
	LanguagePreference NullableStringField
	ThemePreference    NullableStringField
}

type SetUserPasswordParams struct {
	ID           int64
	PasswordHash string
	Status       model.UserStatus
}

type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type UserRepository struct {
	db DBTX
}

func NewUserRepository(db DBTX) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) GetByID(ctx context.Context, id int64) (*model.User, error) {
	const query = `
		SELECT id, email, password_hash, name, is_admin, status, version, language_preference, theme_preference
		FROM users
		WHERE id = $1;
	`

	user := &model.User{}
	err := scanUser(r.db.QueryRowContext(ctx, query, id), user)
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
		SELECT id, email, password_hash, name, is_admin, status, version, language_preference, theme_preference
		FROM users
		WHERE email = $1;
	`

	user := &model.User{}
	err := scanUser(r.db.QueryRowContext(ctx, query, email), user)
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
		SELECT id, email, password_hash, name, is_admin, status, version, language_preference, theme_preference
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
		if err := scanUser(rows, user); err != nil {
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
		RETURNING id, email, password_hash, name, is_admin, status, version, language_preference, theme_preference;
	`

	user := &model.User{}
	err := scanUser(
		r.db.QueryRowContext(
			ctx,
			query,
			params.Email,
			params.PasswordHash,
			params.Name,
			params.IsAdmin,
			params.Status,
		),
		user,
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
		RETURNING id, email, password_hash, name, is_admin, status, version, language_preference, theme_preference;
	`

	user := &model.User{}
	err := scanUser(
		r.db.QueryRowContext(
			ctx,
			query,
			params.ID,
			params.Email,
			params.Name,
			params.IsAdmin,
			params.Version,
		),
		user,
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
		RETURNING id, email, password_hash, name, is_admin, status, version, language_preference, theme_preference;
	`

	user := &model.User{}
	err := scanUser(
		r.db.QueryRowContext(
			ctx,
			query,
			params.ID,
			params.Status,
			params.Version,
		),
		user,
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
		RETURNING id, email, password_hash, name, is_admin, status, version, language_preference, theme_preference;
	`

	user := &model.User{}
	err := scanUser(
		r.db.QueryRowContext(
			ctx,
			query,
			params.ID,
			params.PasswordHash,
			params.Version,
		),
		user,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, r.resolveWriteConflict(ctx, params.ID)
		}
		return nil, err
	}

	return user, nil
}

func (r *UserRepository) GetByIDForUpdate(ctx context.Context, id int64) (*model.User, error) {
	const query = `
		SELECT id, email, password_hash, name, is_admin, status, version, language_preference, theme_preference
		FROM users
		WHERE id = $1
		FOR UPDATE;
	`

	user := &model.User{}
	err := scanUser(r.db.QueryRowContext(ctx, query, id), user)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) UpdatePasswordByID(ctx context.Context, id int64, passwordHash string) (*model.User, error) {
	const query = `
		UPDATE users
		SET password_hash = $2, version = version + 1
		WHERE id = $1
		RETURNING id, email, password_hash, name, is_admin, status, version, language_preference, theme_preference;
	`

	user := &model.User{}
	err := scanUser(r.db.QueryRowContext(ctx, query, id, passwordHash), user)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) UpdateEmail(ctx context.Context, id int64, email string) (*model.User, error) {
	const query = `
		UPDATE users
		SET email = $2, version = version + 1
		WHERE id = $1
		RETURNING id, email, password_hash, name, is_admin, status, version, language_preference, theme_preference;
	`

	user := &model.User{}
	err := scanUser(r.db.QueryRowContext(ctx, query, id, email), user)
	if err != nil {
		switch {
		case isUniqueViolation(err):
			return nil, ErrEmailAlreadyExists
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrUserNotFound
		default:
			return nil, err
		}
	}
	return user, nil
}

func (r *UserRepository) UpdatePreferencesAndName(
	ctx context.Context,
	params UpdateOwnProfileParams,
) (*model.User, error) {
	if !params.Name.Set && !params.LanguagePreference.Set && !params.ThemePreference.Set {
		return r.GetByID(ctx, params.ID)
	}

	const query = `
		UPDATE users
		SET
			name = CASE WHEN $2 THEN $3 ELSE name END,
			language_preference = CASE WHEN $4 THEN $5 ELSE language_preference END,
			theme_preference = CASE WHEN $6 THEN $7 ELSE theme_preference END,
			version = version + 1
		WHERE id = $1
		RETURNING id, email, password_hash, name, is_admin, status, version, language_preference, theme_preference;
	`

	user := &model.User{}
	err := scanUser(
		r.db.QueryRowContext(
			ctx,
			query,
			params.ID,
			params.Name.Set,
			sqlNullableString(params.Name.Value),
			params.LanguagePreference.Set,
			sqlNullableString(params.LanguagePreference.Value),
			params.ThemePreference.Set,
			sqlNullableString(params.ThemePreference.Value),
		),
		user,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) SetPasswordAndStatus(ctx context.Context, params SetUserPasswordParams) (*model.User, error) {
	const query = `
		UPDATE users
		SET password_hash = $2, status = $3, version = version + 1
		WHERE id = $1
		RETURNING id, email, password_hash, name, is_admin, status, version, language_preference, theme_preference;
	`

	user := &model.User{}
	err := scanUser(
		r.db.QueryRowContext(
			ctx,
			query,
			params.ID,
			params.PasswordHash,
			params.Status,
		),
		user,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
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

type userScanner interface {
	Scan(dest ...any) error
}

func scanUser(scanner userScanner, user *model.User) error {
	var passwordHash sql.NullString
	var languagePreference sql.NullString
	var themePreference sql.NullString
	if err := scanner.Scan(
		&user.ID,
		&user.Email,
		&passwordHash,
		&user.Name,
		&user.IsAdmin,
		&user.Status,
		&user.Version,
		&languagePreference,
		&themePreference,
	); err != nil {
		return err
	}

	user.PasswordHash = ""
	if passwordHash.Valid {
		user.PasswordHash = passwordHash.String
	}
	user.LanguagePreference = nil
	if languagePreference.Valid {
		value := model.LanguagePreference(languagePreference.String)
		user.LanguagePreference = &value
	}
	user.ThemePreference = nil
	if themePreference.Valid {
		value := model.ThemePreference(themePreference.String)
		user.ThemePreference = &value
	}
	return nil
}

func sqlNullableString(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *value, Valid: true}
}
