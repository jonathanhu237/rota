package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
)

type SetupTxManager struct {
	db *sql.DB
}

type SetupUserRepository interface {
	GetByID(ctx context.Context, id int64) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	Create(ctx context.Context, params CreateUserParams) (*model.User, error)
	SetPasswordAndStatus(ctx context.Context, params SetUserPasswordParams) (*model.User, error)
}

type SetupTokenRepositoryWriter interface {
	Create(ctx context.Context, params CreateSetupTokenParams) (*model.SetupToken, error)
	GetByTokenHash(ctx context.Context, tokenHash string) (*model.SetupToken, error)
	InvalidateUnusedTokens(ctx context.Context, userID int64, purpose model.SetupTokenPurpose, usedAt time.Time) error
	InvalidateAllUnusedTokens(ctx context.Context, userID int64, usedAt time.Time) error
	MarkUsed(ctx context.Context, id int64, usedAt time.Time) error
}

type SetupTxRunner interface {
	WithinTx(
		ctx context.Context,
		fn func(
			ctx context.Context,
			tx *sql.Tx,
			userRepo SetupUserRepository,
			tokenRepo SetupTokenRepositoryWriter,
		) error,
	) error
}

func NewSetupTxManager(db *sql.DB) *SetupTxManager {
	return &SetupTxManager{db: db}
}

func (m *SetupTxManager) WithinTx(
	ctx context.Context,
	fn func(
		ctx context.Context,
		tx *sql.Tx,
		userRepo SetupUserRepository,
		tokenRepo SetupTokenRepositoryWriter,
	) error,
) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := fn(ctx, tx, NewUserRepository(tx), NewSetupTokenRepository(tx)); err != nil {
		return err
	}

	return tx.Commit()
}
