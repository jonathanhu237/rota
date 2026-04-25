package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
)

var ErrSetupTokenAlreadyExists = errors.New("setup token already exists")

type CreateSetupTokenParams struct {
	UserID    int64
	TokenHash string
	Purpose   model.SetupTokenPurpose
	ExpiresAt time.Time
}

type SetupTokenRepository struct {
	db DBTX
}

func NewSetupTokenRepository(db DBTX) *SetupTokenRepository {
	return &SetupTokenRepository{db: db}
}

func (r *SetupTokenRepository) Create(ctx context.Context, params CreateSetupTokenParams) (*model.SetupToken, error) {
	const query = `
		INSERT INTO user_setup_tokens (user_id, token_hash, purpose, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, token_hash, purpose, expires_at, used_at, created_at;
	`

	token := &model.SetupToken{}
	err := scanSetupToken(
		r.db.QueryRowContext(
			ctx,
			query,
			params.UserID,
			params.TokenHash,
			params.Purpose,
			params.ExpiresAt,
		),
		token,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrSetupTokenAlreadyExists
		}
		return nil, err
	}

	return token, nil
}

func (r *SetupTokenRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*model.SetupToken, error) {
	const query = `
		SELECT id, user_id, token_hash, purpose, expires_at, used_at, created_at
		FROM user_setup_tokens
		WHERE token_hash = $1;
	`

	token := &model.SetupToken{}
	err := scanSetupToken(r.db.QueryRowContext(ctx, query, tokenHash), token)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrTokenNotFound
	}
	if err != nil {
		return nil, err
	}

	return token, nil
}

func (r *SetupTokenRepository) InvalidateUnusedTokens(ctx context.Context, userID int64, purpose model.SetupTokenPurpose, usedAt time.Time) error {
	const query = `
		UPDATE user_setup_tokens
		SET used_at = $3
		WHERE user_id = $1
			AND purpose = $2
			AND used_at IS NULL;
	`

	_, err := r.db.ExecContext(ctx, query, userID, purpose, usedAt)
	return err
}

func (r *SetupTokenRepository) InvalidateAllUnusedTokens(ctx context.Context, userID int64, usedAt time.Time) error {
	const query = `
		UPDATE user_setup_tokens
		SET used_at = $2
		WHERE user_id = $1
			AND used_at IS NULL;
	`

	_, err := r.db.ExecContext(ctx, query, userID, usedAt)
	return err
}

func (r *SetupTokenRepository) MarkUsed(ctx context.Context, id int64, usedAt time.Time) error {
	const query = `
		UPDATE user_setup_tokens
		SET used_at = $2
		WHERE id = $1
			AND used_at IS NULL;
	`

	result, err := r.db.ExecContext(ctx, query, id, usedAt)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return model.ErrTokenUsed
	}

	return nil
}

type setupTokenScanner interface {
	Scan(dest ...any) error
}

func scanSetupToken(scanner setupTokenScanner, token *model.SetupToken) error {
	var usedAt sql.NullTime
	if err := scanner.Scan(
		&token.ID,
		&token.UserID,
		&token.TokenHash,
		&token.Purpose,
		&token.ExpiresAt,
		&usedAt,
		&token.CreatedAt,
	); err != nil {
		return err
	}

	token.UsedAt = nil
	if usedAt.Valid {
		token.UsedAt = &usedAt.Time
	}

	return nil
}
