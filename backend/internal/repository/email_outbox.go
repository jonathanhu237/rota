package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/email"
)

type OutboxJob struct {
	ID         int64
	UserID     *int64
	Kind       string
	Recipient  string
	Subject    string
	Body       string
	HTMLBody   string
	RetryCount int
}

type OutboxRepository struct {
	db *sql.DB
}

const outboxClaimLease = 5 * time.Minute

type outboxEnqueueOptions struct {
	userID *int64
}

type OutboxOption func(*outboxEnqueueOptions)

func WithOutboxUserID(userID int64) OutboxOption {
	return func(opts *outboxEnqueueOptions) {
		opts.userID = &userID
	}
}

func NewOutboxRepository(db *sql.DB) *OutboxRepository {
	return &OutboxRepository{db: db}
}

func (r *OutboxRepository) EnqueueTx(ctx context.Context, tx *sql.Tx, msg email.Message, opts ...OutboxOption) error {
	options := outboxEnqueueOptions{}
	for _, opt := range opts {
		opt(&options)
	}

	const query = `
		INSERT INTO email_outbox (user_id, kind, recipient, subject, body, html_body)
		VALUES ($1, $2, $3, $4, $5, $6);
	`

	var userID sql.NullInt64
	if options.userID != nil {
		userID = sql.NullInt64{Int64: *options.userID, Valid: true}
	}

	_, err := tx.ExecContext(ctx, query, userID, normalizeOutboxKind(msg.Kind), msg.To, msg.Subject, msg.Body, nullableOutboxHTMLBody(msg.HTMLBody))
	return err
}

func (r *OutboxRepository) Claim(ctx context.Context, batchSize int) ([]OutboxJob, error) {
	if batchSize <= 0 {
		return nil, nil
	}

	const query = `
		UPDATE email_outbox
		SET next_attempt_at = NOW() + ($2 * INTERVAL '1 second')
		WHERE id IN (
			SELECT id
			FROM email_outbox
			WHERE status = 'pending'
				AND next_attempt_at <= NOW()
			ORDER BY next_attempt_at ASC, id ASC
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, user_id, kind, recipient, subject, body, html_body, retry_count;
	`

	rows, err := r.db.QueryContext(ctx, query, batchSize, int(outboxClaimLease/time.Second))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := make([]OutboxJob, 0)
	for rows.Next() {
		job, err := scanOutboxJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return jobs, nil
}

func (r *OutboxRepository) MarkSent(ctx context.Context, id int64) error {
	const query = `
		UPDATE email_outbox
		SET status = 'sent',
			sent_at = NOW(),
			last_error = NULL
		WHERE id = $1;
	`

	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *OutboxRepository) MarkRetryable(ctx context.Context, id int64, lastError string, nextAttemptAt time.Time) error {
	const query = `
		UPDATE email_outbox
		SET status = 'pending',
			retry_count = retry_count + 1,
			last_error = $2,
			next_attempt_at = $3
		WHERE id = $1;
	`

	_, err := r.db.ExecContext(ctx, query, id, lastError, nextAttemptAt)
	return err
}

func (r *OutboxRepository) MarkFailed(ctx context.Context, id int64, lastError string) error {
	const query = `
		UPDATE email_outbox
		SET status = 'failed',
			retry_count = retry_count + 1,
			last_error = $2,
			failed_at = NOW()
		WHERE id = $1;
	`

	_, err := r.db.ExecContext(ctx, query, id, lastError)
	return err
}

type outboxJobScanner interface {
	Scan(dest ...any) error
}

func scanOutboxJob(scanner outboxJobScanner) (OutboxJob, error) {
	var (
		job      OutboxJob
		userID   sql.NullInt64
		htmlBody sql.NullString
	)
	if err := scanner.Scan(
		&job.ID,
		&userID,
		&job.Kind,
		&job.Recipient,
		&job.Subject,
		&job.Body,
		&htmlBody,
		&job.RetryCount,
	); err != nil {
		return OutboxJob{}, err
	}
	if userID.Valid {
		job.UserID = &userID.Int64
	}
	if htmlBody.Valid {
		job.HTMLBody = htmlBody.String
	}
	return job, nil
}

func normalizeOutboxKind(kind string) string {
	if kind == "" {
		return email.KindUnknown
	}
	return kind
}

func nullableOutboxHTMLBody(body string) sql.NullString {
	if body == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: body, Valid: true}
}
