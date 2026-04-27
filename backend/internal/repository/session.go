package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"
)

var ErrSessionNotFound = errors.New("session not found")

type SessionRepository struct {
	db      DBTX
	expires time.Duration
}

func NewSessionRepository(db *sql.DB, expires time.Duration) *SessionRepository {
	return &SessionRepository{
		db:      db,
		expires: expires,
	}
}

func NewSessionRepositoryWithDBTX(db DBTX, expires time.Duration) *SessionRepository {
	return &SessionRepository{
		db:      db,
		expires: expires,
	}
}

func (r *SessionRepository) Create(ctx context.Context, userID int64) (string, int64, error) {
	sessionID, err := generateSessionID()
	if err != nil {
		return "", 0, err
	}

	const query = `
		INSERT INTO sessions (id, user_id, expires_at)
		VALUES ($1, $2, NOW() + ($3::bigint * INTERVAL '1 second'));
	`

	expiresInSeconds := int64(r.expires.Seconds())
	if _, err := r.db.ExecContext(ctx, query, sessionID, userID, expiresInSeconds); err != nil {
		return "", 0, err
	}

	return sessionID, expiresInSeconds, nil
}

func (r *SessionRepository) Get(ctx context.Context, sessionID string) (int64, error) {
	const query = `
		SELECT user_id
		FROM sessions
		WHERE id = $1 AND expires_at > NOW();
	`

	var userID int64
	err := r.db.QueryRowContext(ctx, query, sessionID).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrSessionNotFound
	}
	if err != nil {
		return 0, err
	}
	return userID, nil
}

func (r *SessionRepository) Refresh(ctx context.Context, sessionID string) (int64, error) {
	const query = `
		UPDATE sessions
		SET expires_at = NOW() + ($1::bigint * INTERVAL '1 second')
		WHERE id = $2 AND expires_at > NOW()
		RETURNING EXTRACT(EPOCH FROM expires_at - NOW())::bigint;
	`

	var expiresInSeconds int64
	err := r.db.QueryRowContext(ctx, query, int64(r.expires.Seconds()), sessionID).Scan(&expiresInSeconds)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrSessionNotFound
	}
	if err != nil {
		return 0, err
	}
	return expiresInSeconds, nil
}

func (r *SessionRepository) Delete(ctx context.Context, sessionID string) error {
	const query = `
		DELETE FROM sessions
		WHERE id = $1;
	`

	_, err := r.db.ExecContext(ctx, query, sessionID)
	return err
}

func (r *SessionRepository) DeleteUserSessions(ctx context.Context, userID int64) error {
	const query = `
		DELETE FROM sessions
		WHERE user_id = $1;
	`

	_, err := r.db.ExecContext(ctx, query, userID)
	return err
}

func (r *SessionRepository) DeleteOtherSessions(ctx context.Context, userID int64, currentSessionID string) (int, error) {
	const query = `
		DELETE FROM sessions
		WHERE user_id = $1 AND id != $2;
	`

	result, err := r.db.ExecContext(ctx, query, userID, currentSessionID)
	if err != nil {
		return 0, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func generateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
