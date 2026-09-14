package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jonathanhu237/rota/api/internal/rota/model"
)

var (
	ErrUserNotFound = errors.New("user not found")
)

type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type userScanner interface{ Scan(dest ...any) error }

func scanUser(scanner userScanner, user *model.User) error {
	return scanner.Scan(
		&user.ID,
		&user.Email,
		&user.Name,
		&user.IsAdmin,
		&user.Status,
		&user.Version,
		&user.LanguagePreference,
		&user.ThemePreference,
	)
}
