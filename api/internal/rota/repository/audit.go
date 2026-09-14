package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/jonathanhu237/rota/api/internal/rota/audit"
)

// AuditRecorder writes business history into Temvia's operation log. The
// common log stores object IDs as text, so both numeric Rota entities and UUID
// account identities can be represented without a second history table.
type AuditRecorder struct {
	db     *sql.DB
	logger *slog.Logger
}

func NewAuditRecorder(db *sql.DB, logger *slog.Logger) *AuditRecorder {
	if logger == nil {
		logger = slog.Default()
	}
	return &AuditRecorder{db: db, logger: logger}
}

func (r *AuditRecorder) Record(ctx context.Context, event audit.RecordedEvent) {
	if r == nil || r.db == nil {
		return
	}
	metadata, err := encodeAuditMetadata(event.Metadata)
	if err != nil {
		r.logger.Warn("rota audit: failed to encode metadata", "action", event.Action, "error", err)
		return
	}
	objectID := ""
	if event.TargetObjectID != nil {
		objectID = *event.TargetObjectID
	} else if event.TargetID != nil {
		objectID = strconv.FormatInt(*event.TargetID, 10)
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO auth_operation_logs (
			actor_user_id, actor_kind, action, object_type, object_id, result,
			source_ip, details
		)
		VALUES (NULLIF($1, '')::uuid, CASE WHEN $1 = '' THEN 'system' ELSE 'authenticated' END,
			$2, COALESCE(NULLIF($3, ''), 'rota'), NULLIF($4, ''), 'success', $5, $6::jsonb)`,
		actorID(event), event.Action, event.TargetType, objectID, event.ActorIP, metadata)
	if err != nil {
		r.logger.Warn("rota audit: failed to insert operation log", "action", event.Action, "error", err)
	}
}

func actorID(event audit.RecordedEvent) string {
	if event.ActorID == nil {
		return ""
	}
	return *event.ActorID
}

func encodeAuditMetadata(metadata map[string]any) ([]byte, error) {
	if len(metadata) == 0 {
		return []byte("{}"), nil
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal metadata: %w", err)
	}
	return encoded, nil
}
