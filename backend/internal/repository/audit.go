package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"

	"github.com/jonathanhu237/rota/backend/internal/audit"
)

// AuditRecorder is a PostgreSQL-backed audit.Recorder. Writes are
// synchronous for simplicity; at this project's scale the extra few
// milliseconds per mutation are negligible and durability is guaranteed
// without any graceful-shutdown coordination. Failures are logged at
// warning level and swallowed so the primary operation is never affected.
type AuditRecorder struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewAuditRecorder returns a Recorder backed by the given database. If
// logger is nil, slog.Default() is used.
func NewAuditRecorder(db *sql.DB, logger *slog.Logger) *AuditRecorder {
	if logger == nil {
		logger = slog.Default()
	}
	return &AuditRecorder{db: db, logger: logger}
}

// Record implements audit.Recorder by inserting one row into audit_logs.
// Errors are logged but not returned.
func (r *AuditRecorder) Record(ctx context.Context, event audit.RecordedEvent) {
	const query = `
		INSERT INTO audit_logs (
			actor_id,
			actor_ip,
			action,
			target_type,
			target_id,
			metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6);
	`

	metadata, err := encodeAuditMetadata(event.Metadata)
	if err != nil {
		r.logger.Warn(
			"audit: failed to encode metadata; dropping event",
			"action", event.Action,
			"error", err,
		)
		return
	}

	var actorID sql.NullInt64
	if event.ActorID != nil {
		actorID = sql.NullInt64{Int64: *event.ActorID, Valid: true}
	}

	var actorIP sql.NullString
	if event.ActorIP != "" {
		actorIP = sql.NullString{String: event.ActorIP, Valid: true}
	}

	var targetType sql.NullString
	if event.TargetType != "" {
		targetType = sql.NullString{String: event.TargetType, Valid: true}
	}

	var targetID sql.NullInt64
	if event.TargetID != nil {
		targetID = sql.NullInt64{Int64: *event.TargetID, Valid: true}
	}

	if _, err := r.db.ExecContext(
		ctx,
		query,
		actorID,
		actorIP,
		event.Action,
		targetType,
		targetID,
		metadata,
	); err != nil {
		r.logger.Warn(
			"audit: failed to insert event",
			"action", event.Action,
			"actor_id", event.ActorID,
			"error", err,
		)
	}
}

func encodeAuditMetadata(metadata map[string]any) ([]byte, error) {
	if len(metadata) == 0 {
		return []byte("{}"), nil
	}
	return json.Marshal(metadata)
}
