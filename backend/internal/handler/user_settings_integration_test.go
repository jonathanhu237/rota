//go:build integration

package handler

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/audit"
	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
	"github.com/jonathanhu237/rota/backend/internal/service"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

const handlerIntegrationDBLockKey int64 = 2026042301

func TestUserSettingsHandlerIntegration(t *testing.T) {
	t.Run("change password preserves current session, revokes others, and audits count", func(t *testing.T) {
		db := openHandlerIntegrationDB(t)
		sessionExpires := 24 * time.Hour
		userRepo := repository.NewUserRepository(db)
		sessionStore := repository.NewSessionRepository(db, sessionExpires)
		authService := service.NewAuthService(
			userRepo,
			sessionStore,
			service.WithAuthPasswordTxRunner(repository.NewAuthPasswordTxManager(db, sessionExpires)),
		)
		authHandler := NewAuthHandler(authService)
		auditRecorder := repository.NewAuditRecorder(db, nil)

		userID := seedHandlerIntegrationUser(t, db, "worker@example.com", "Worker", "current-password")
		sessionA, _, err := sessionStore.Create(context.Background(), userID)
		if err != nil {
			t.Fatalf("create session A: %v", err)
		}
		sessionB, _, err := sessionStore.Create(context.Background(), userID)
		if err != nil {
			t.Fatalf("create session B: %v", err)
		}

		req := jsonRequest(t, http.MethodPost, "/auth/change-password", map[string]any{
			"current_password": "current-password",
			"new_password":     "changed-password",
		})
		req = req.WithContext(audit.WithRecorder(req.Context(), auditRecorder))
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionA})
		recorder := httptest.NewRecorder()

		authHandler.RequireAuth(authHandler.ChangePassword)(recorder, req)

		if recorder.Code != http.StatusNoContent {
			t.Fatalf("expected status 204, got %d body=%s", recorder.Code, recorder.Body.String())
		}
		if _, err := sessionStore.Get(context.Background(), sessionA); err != nil {
			t.Fatalf("expected current session to remain valid: %v", err)
		}
		if _, err := sessionStore.Get(context.Background(), sessionB); err == nil {
			t.Fatalf("expected stale session to be revoked")
		}
		var remainingSessions int
		if err := db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM sessions WHERE user_id = $1;`, userID).Scan(&remainingSessions); err != nil {
			t.Fatalf("count sessions: %v", err)
		}
		if remainingSessions != 1 {
			t.Fatalf("expected 1 remaining session, got %d", remainingSessions)
		}

		meReq := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
		meReq.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionA})
		meRecorder := httptest.NewRecorder()
		authHandler.RequireAuth(authHandler.Me)(meRecorder, meReq)
		if meRecorder.Code != http.StatusOK {
			t.Fatalf("expected current session GET /auth/me 200, got %d", meRecorder.Code)
		}

		var revokedCount string
		if err := db.QueryRowContext(
			context.Background(),
			`SELECT metadata->>'revoked_session_count' FROM audit_logs WHERE action = $1 ORDER BY id DESC LIMIT 1;`,
			audit.ActionAuthPasswordChange,
		).Scan(&revokedCount); err != nil {
			t.Fatalf("read password change audit row: %v", err)
		}
		if revokedCount != "1" {
			t.Fatalf("expected revoked_session_count=1, got %q", revokedCount)
		}
	})
}

func openHandlerIntegrationDB(t testing.TB) *sql.DB {
	t.Helper()

	db, err := sql.Open("postgres", handlerIntegrationDatabaseURL())
	if err != nil {
		t.Skipf("skipping integration test: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		t.Skipf("skipping integration test: %v", err)
	}

	lockConn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("open lock connection: %v", err)
	}
	if _, err := lockConn.ExecContext(ctx, `SELECT pg_advisory_lock($1);`, handlerIntegrationDBLockKey); err != nil {
		_ = lockConn.Close()
		t.Fatalf("acquire integration lock: %v", err)
	}
	if err := resetHandlerIntegrationDB(ctx, db); err != nil {
		_ = lockConn.Close()
		t.Fatalf("reset integration db: %v", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := resetHandlerIntegrationDB(ctx, db); err != nil {
			t.Fatalf("reset integration db cleanup: %v", err)
		}
		if _, err := lockConn.ExecContext(ctx, `SELECT pg_advisory_unlock($1);`, handlerIntegrationDBLockKey); err != nil {
			t.Fatalf("release integration lock: %v", err)
		}
		if err := lockConn.Close(); err != nil {
			t.Fatalf("close lock connection: %v", err)
		}
		_ = db.Close()
	})

	return db
}

func resetHandlerIntegrationDB(ctx context.Context, db *sql.DB) error {
	tables := []string{
		"audit_logs",
		"email_outbox",
		"sessions",
		"leaves",
		"shift_change_requests",
		"assignments",
		"availability_submissions",
		"template_slot_positions",
		"template_slots",
		"user_setup_tokens",
		"publications",
		"user_positions",
		"templates",
		"positions",
		"users",
	}
	_, err := db.ExecContext(ctx, "TRUNCATE TABLE "+strings.Join(tables, ", ")+" RESTART IDENTITY CASCADE;")
	return err
}

func seedHandlerIntegrationUser(t testing.TB, db *sql.DB, email, name, password string) int64 {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	var userID int64
	if err := db.QueryRowContext(
		context.Background(),
		`
			INSERT INTO users (email, password_hash, name, is_admin, status)
			VALUES ($1, $2, $3, FALSE, $4)
			RETURNING id;
		`,
		email,
		string(hash),
		name,
		model.UserStatusActive,
	).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return userID
}

func handlerIntegrationDatabaseURL() string {
	if value := strings.TrimSpace(os.Getenv("DATABASE_URL")); value != "" {
		return value
	}

	host := handlerEnvOrDefault("POSTGRES_HOST", "localhost")
	port := handlerEnvOrDefault("POSTGRES_PORT", "5432")
	user := handlerEnvOrDefault("POSTGRES_USER", "rota")
	password := handlerEnvOrDefault("POSTGRES_PASSWORD", "pa55word")
	database := handlerEnvOrDefault("POSTGRES_DB", "rota")

	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, password, host, port, database)
}

func handlerEnvOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
