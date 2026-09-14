//go:build integration

package repository

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/jonathanhu237/rota/api/internal/auth/application"
	"github.com/jonathanhu237/rota/api/internal/rota/email"
)

func TestRotaMigrationPreservesUUIDIdentityAndSchedulingShape(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN is not set")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}

	var migrationVersion int64
	var dirty bool
	if err := db.QueryRowContext(ctx, `SELECT version, dirty FROM schema_migrations`).Scan(&migrationVersion, &dirty); err != nil {
		t.Fatal(err)
	}
	if migrationVersion != 14 || dirty {
		t.Fatalf("schema version = %d dirty=%t, want 14/false", migrationVersion, dirty)
	}

	for _, table := range []string{
		"positions",
		"templates",
		"template_slots",
		"template_slot_weekdays",
		"user_positions",
		"publications",
		"availability_submissions",
		"assignments",
		"assignment_overrides",
		"shift_change_requests",
		"leaves",
		"attendance_records",
		"attendance_overtime_records",
	} {
		var relation sql.NullString
		if err := db.QueryRowContext(ctx, `SELECT to_regclass($1)`, "public."+table).Scan(&relation); err != nil {
			t.Fatalf("relation %s: %v", table, err)
		}
		if !relation.Valid {
			t.Fatalf("relation %s is missing", table)
		}
	}

	var userID string
	if err := db.QueryRowContext(ctx, `SELECT udt_name FROM information_schema.columns WHERE table_schema='public' AND table_name='user_positions' AND column_name='user_id'`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if userID != "uuid" {
		t.Fatalf("user_positions.user_id type = %q, want uuid", userID)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	const historicalID = "019535d9-3df7-79fb-b466-fa907fa17f9e"
	if _, err := tx.ExecContext(ctx, `INSERT INTO positions (name) VALUES ('migration-test-position')`); err != nil {
		t.Fatal(err)
	}
	var positionID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM positions WHERE name='migration-test-position'`).Scan(&positionID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO auth_deleted_user_identities (user_id, name, email) VALUES ($1, 'Deleted worker', 'deleted-worker@example.com')`, historicalID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO user_positions (user_id, position_id) VALUES ($1, $2)`, historicalID, positionID); err != nil {
		t.Fatal(err)
	}

	var name, status string
	if err := tx.QueryRowContext(ctx, `SELECT name, status FROM users WHERE id=$1`, historicalID).Scan(&name, &status); err != nil {
		t.Fatal(err)
	}
	if name != "Deleted worker" || status != "disabled" {
		t.Fatalf("historical user = %q/%q, want Deleted worker/disabled", name, status)
	}
	var qualificationCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_positions WHERE user_id=$1`, historicalID).Scan(&qualificationCount); err != nil {
		t.Fatal(err)
	}
	if qualificationCount != 1 {
		t.Fatalf("historical qualification count = %d, want 1", qualificationCount)
	}

	mailTaskSecretBox, err := application.NewMailTaskSecretBox(bytes.Repeat([]byte{0x42}, 32))
	if err != nil {
		t.Fatal(err)
	}
	mail := email.Message{
		Kind:             email.KindShiftChangeRequestReceived,
		To:               "migration-test-mail@example.com",
		Name:             "Migration Recipient",
		Language:         "zh-CN",
		SystemName:       "Migration Rota",
		OrganizationName: "Migration Org",
		Subject:          "Migration test",
		Body:             "Text body",
		HTMLBody:         "<p>HTML body</p>",
	}
	outbox := NewOutboxRepository(db, mailTaskSecretBox)
	if err := outbox.EnqueueTx(ctx, tx, mail); err != nil {
		t.Fatalf("enqueue Rota notification: %v", err)
	}
	var recipientName, locale, systemName, organizationName string
	var ciphertext []byte
	if err := tx.QueryRowContext(ctx, `
		SELECT recipient_name, locale, system_name, organization_name, material_ciphertext
		FROM auth_mail_outbox
		WHERE recipient_email = 'migration-test-mail@example.com'
		ORDER BY created_at DESC
		LIMIT 1`).Scan(&recipientName, &locale, &systemName, &organizationName, &ciphertext); err != nil {
		t.Fatal(err)
	}
	if recipientName != mail.Name || locale != "zh-CN" || systemName != mail.SystemName || organizationName != mail.OrganizationName || len(ciphertext) == 0 {
		t.Fatalf("outbox projection = name %q locale %q system %q ciphertext=%d", recipientName, locale, systemName, len(ciphertext))
	}
	material, err := application.OpenMailTaskMaterial(mailTaskSecretBox, ciphertext)
	if err != nil {
		t.Fatalf("open encrypted Rota notification: %v", err)
	}
	if material.Locale != "zh-CN" || material.Name != mail.Name || material.Email != mail.To || material.OrganizationName != mail.OrganizationName || material.Message == nil || material.Message.Text != mail.Body || material.Message.HTML != mail.HTMLBody {
		t.Fatalf("encrypted material = %#v", material)
	}
}
