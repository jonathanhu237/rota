//go:build integration

package repository

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"
)

func TestSessionRepositoryIntegration(t *testing.T) {
	ctx := context.Background()

	t.Run("Create and Get round-trip", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewSessionRepository(db, 2*time.Hour)
		user := seedUser(t, db, userSeed{})

		sessionID, expiresIn, err := repo.Create(ctx, user.ID)
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		if expiresIn != int64((2 * time.Hour).Seconds()) {
			t.Fatalf("expiresIn = %d, want %d", expiresIn, int64((2 * time.Hour).Seconds()))
		}
		if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(sessionID) {
			t.Fatalf("session id %q does not match 32-byte lowercase hex format", sessionID)
		}

		gotUserID, err := repo.Get(ctx, sessionID)
		if err != nil {
			t.Fatalf("Get returned error: %v", err)
		}
		if gotUserID != user.ID {
			t.Fatalf("Get userID = %d, want %d", gotUserID, user.ID)
		}
	})

	t.Run("Refresh extends expiry", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewSessionRepository(db, 3*time.Hour)
		user := seedUser(t, db, userSeed{})
		sessionID := "refresh-session"
		insertSession(t, db, sessionID, user.ID, time.Now().Add(time.Minute))

		expiresIn, err := repo.Refresh(ctx, sessionID)
		if err != nil {
			t.Fatalf("Refresh returned error: %v", err)
		}
		if expiresIn < int64((3*time.Hour-10*time.Second).Seconds()) || expiresIn > int64((3*time.Hour).Seconds()) {
			t.Fatalf("expiresIn = %d, want close to %d", expiresIn, int64((3 * time.Hour).Seconds()))
		}

		var expiresAt time.Time
		if err := db.QueryRowContext(ctx, `SELECT expires_at FROM sessions WHERE id = $1;`, sessionID).Scan(&expiresAt); err != nil {
			t.Fatalf("read refreshed expiry: %v", err)
		}
		if time.Until(expiresAt) < 2*time.Hour {
			t.Fatalf("expires_at was not extended enough: %s", expiresAt)
		}
	})

	t.Run("Delete removes row", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewSessionRepository(db, time.Hour)
		user := seedUser(t, db, userSeed{})
		sessionID, _, err := repo.Create(ctx, user.ID)
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}

		if err := repo.Delete(ctx, sessionID); err != nil {
			t.Fatalf("Delete returned error: %v", err)
		}
		if _, err := repo.Get(ctx, sessionID); !errors.Is(err, ErrSessionNotFound) {
			t.Fatalf("Get after Delete error = %v, want ErrSessionNotFound", err)
		}
	})

	t.Run("DeleteUserSessions removes only one user's rows", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewSessionRepository(db, time.Hour)
		firstUser := seedUser(t, db, userSeed{})
		secondUser := seedUser(t, db, userSeed{})

		firstSession, _, err := repo.Create(ctx, firstUser.ID)
		if err != nil {
			t.Fatalf("Create first session: %v", err)
		}
		secondFirstSession, _, err := repo.Create(ctx, firstUser.ID)
		if err != nil {
			t.Fatalf("Create second first-user session: %v", err)
		}
		otherSession, _, err := repo.Create(ctx, secondUser.ID)
		if err != nil {
			t.Fatalf("Create other user session: %v", err)
		}

		if err := repo.DeleteUserSessions(ctx, firstUser.ID); err != nil {
			t.Fatalf("DeleteUserSessions returned error: %v", err)
		}

		for _, sessionID := range []string{firstSession, secondFirstSession} {
			if _, err := repo.Get(ctx, sessionID); !errors.Is(err, ErrSessionNotFound) {
				t.Fatalf("Get deleted session %q error = %v, want ErrSessionNotFound", sessionID, err)
			}
		}
		gotUserID, err := repo.Get(ctx, otherSession)
		if err != nil {
			t.Fatalf("Get other user's session returned error: %v", err)
		}
		if gotUserID != secondUser.ID {
			t.Fatalf("other session userID = %d, want %d", gotUserID, secondUser.ID)
		}
	})

	t.Run("Expired rows are invisible to Get and Refresh", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewSessionRepository(db, time.Hour)
		user := seedUser(t, db, userSeed{})
		sessionID := "expired-session"
		insertSession(t, db, sessionID, user.ID, time.Now().Add(-time.Minute))

		if _, err := repo.Get(ctx, sessionID); !errors.Is(err, ErrSessionNotFound) {
			t.Fatalf("Get expired session error = %v, want ErrSessionNotFound", err)
		}
		if _, err := repo.Refresh(ctx, sessionID); !errors.Is(err, ErrSessionNotFound) {
			t.Fatalf("Refresh expired session error = %v, want ErrSessionNotFound", err)
		}
		if got := countSessions(t, db, sessionID); got != 1 {
			t.Fatalf("expired row should remain for lazy cleanup, count = %d", got)
		}
	})

	t.Run("User delete cascades sessions", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewSessionRepository(db, time.Hour)
		user := seedUser(t, db, userSeed{})
		sessionID, _, err := repo.Create(ctx, user.ID)
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}

		if _, err := db.ExecContext(ctx, `DELETE FROM users WHERE id = $1;`, user.ID); err != nil {
			t.Fatalf("delete user: %v", err)
		}
		if got := countSessions(t, db, sessionID); got != 0 {
			t.Fatalf("session count after user delete = %d, want 0", got)
		}
	})
}

func insertSession(t testing.TB, db *sql.DB, sessionID string, userID int64, expiresAt time.Time) {
	t.Helper()

	_, err := db.ExecContext(
		context.Background(),
		`INSERT INTO sessions (id, user_id, expires_at) VALUES ($1, $2, $3);`,
		sessionID,
		userID,
		expiresAt,
	)
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}
}

func countSessions(t testing.TB, db *sql.DB, sessionID string) int {
	t.Helper()

	var count int
	if err := db.QueryRowContext(
		context.Background(),
		`SELECT COUNT(*) FROM sessions WHERE id = $1;`,
		sessionID,
	).Scan(&count); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	return count
}
