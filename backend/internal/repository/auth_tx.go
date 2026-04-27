package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
)

type AuthPasswordUserRepository interface {
	GetByIDForUpdate(ctx context.Context, id int64) (*model.User, error)
	UpdatePasswordByID(ctx context.Context, id int64, passwordHash string) (*model.User, error)
}

type AuthPasswordSessionRepository interface {
	DeleteOtherSessions(ctx context.Context, userID int64, currentSessionID string) (int, error)
}

type AuthPasswordTxManager struct {
	db             *sql.DB
	sessionExpires time.Duration
}

func NewAuthPasswordTxManager(db *sql.DB, sessionExpires time.Duration) *AuthPasswordTxManager {
	return &AuthPasswordTxManager{db: db, sessionExpires: sessionExpires}
}

func (m *AuthPasswordTxManager) WithinTx(
	ctx context.Context,
	fn func(
		ctx context.Context,
		userRepo AuthPasswordUserRepository,
		sessionRepo AuthPasswordSessionRepository,
	) error,
) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := fn(
		ctx,
		NewUserRepository(tx),
		NewSessionRepositoryWithDBTX(tx, m.sessionExpires),
	); err != nil {
		return err
	}

	return tx.Commit()
}
