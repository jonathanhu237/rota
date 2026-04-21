//go:build integration

package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/jonathanhu237/rota/backend/internal/audit"
)

func TestAuditRecorderRoundtripsAllColumns(t *testing.T) {
	db := openIntegrationDB(t)
	recorder := NewAuditRecorder(db, slog.New(slog.DiscardHandler))

	actor := int64(42)
	target := int64(7)
	ctx := context.Background()

	recorder.Record(ctx, audit.RecordedEvent{
		ActorID:    &actor,
		ActorIP:    "10.0.0.5",
		Action:     audit.ActionUserCreate,
		TargetType: audit.TargetTypeUser,
		TargetID:   &target,
		Metadata: map[string]any{
			"email":    "new@example.com",
			"is_admin": false,
			"nested": map[string]any{
				"key": "value",
			},
		},
	})

	row, err := readLatestAuditRow(ctx, db)
	if err != nil {
		t.Fatalf("read audit row: %v", err)
	}

	if row.ActorID != 42 {
		t.Fatalf("expected actor_id 42, got %d", row.ActorID)
	}
	if row.ActorIP != "10.0.0.5" {
		t.Fatalf("expected actor_ip 10.0.0.5, got %q", row.ActorIP)
	}
	if row.Action != audit.ActionUserCreate {
		t.Fatalf("unexpected action %q", row.Action)
	}
	if row.TargetType != audit.TargetTypeUser {
		t.Fatalf("unexpected target_type %q", row.TargetType)
	}
	if row.TargetID != 7 {
		t.Fatalf("unexpected target_id %d", row.TargetID)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(row.Metadata), &decoded); err != nil {
		t.Fatalf("decode metadata: %v", err)
	}
	if decoded["email"] != "new@example.com" {
		t.Fatalf("unexpected metadata email: %v", decoded["email"])
	}
	if decoded["is_admin"] != false {
		t.Fatalf("unexpected metadata is_admin: %v", decoded["is_admin"])
	}
	nested, ok := decoded["nested"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested map in metadata, got %T", decoded["nested"])
	}
	if nested["key"] != "value" {
		t.Fatalf("unexpected nested key: %v", nested["key"])
	}
}

func TestAuditRecorderHandlesNilActorAndTarget(t *testing.T) {
	db := openIntegrationDB(t)
	recorder := NewAuditRecorder(db, slog.New(slog.DiscardHandler))

	recorder.Record(context.Background(), audit.RecordedEvent{
		Action:  audit.ActionAuthLoginFailure,
		ActorIP: "203.0.113.9",
		Metadata: map[string]any{
			"email":  "ghost@example.com",
			"reason": "invalid_credentials",
		},
	})

	row, err := readLatestAuditRow(context.Background(), db)
	if err != nil {
		t.Fatalf("read audit row: %v", err)
	}

	if row.ActorIDValid {
		t.Fatalf("expected actor_id NULL, got %d", row.ActorID)
	}
	if row.TargetIDValid {
		t.Fatalf("expected target_id NULL, got %d", row.TargetID)
	}
	if row.TargetTypeValid {
		t.Fatalf("expected target_type NULL, got %q", row.TargetType)
	}
	if row.Action != audit.ActionAuthLoginFailure {
		t.Fatalf("unexpected action %q", row.Action)
	}
}

type auditRow struct {
	ActorID         int64
	ActorIDValid    bool
	ActorIP         string
	Action          string
	TargetType      string
	TargetTypeValid bool
	TargetID        int64
	TargetIDValid   bool
	Metadata        string
}

func readLatestAuditRow(ctx context.Context, db *sql.DB) (auditRow, error) {
	const query = `
		SELECT
			actor_id,
			COALESCE(actor_ip, ''),
			action,
			target_type,
			target_id,
			metadata::text
		FROM audit_logs
		ORDER BY id DESC
		LIMIT 1;
	`

	var (
		row        auditRow
		actorID    sql.NullInt64
		targetType sql.NullString
		targetID   sql.NullInt64
	)
	err := db.QueryRowContext(ctx, query).Scan(
		&actorID,
		&row.ActorIP,
		&row.Action,
		&targetType,
		&targetID,
		&row.Metadata,
	)
	if err != nil {
		return row, err
	}
	if actorID.Valid {
		row.ActorID = actorID.Int64
		row.ActorIDValid = true
	}
	if targetType.Valid {
		row.TargetType = targetType.String
		row.TargetTypeValid = true
	}
	if targetID.Valid {
		row.TargetID = targetID.Int64
		row.TargetIDValid = true
	}
	return row, nil
}
