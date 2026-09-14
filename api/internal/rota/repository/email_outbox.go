package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/jonathanhu237/rota/api/internal/auth/application"
	"github.com/jonathanhu237/rota/api/internal/auth/domain"
	"github.com/jonathanhu237/rota/api/internal/rota/email"
)

// OutboxRepository adapts Rota's already-rendered notifications to Temvia's
// durable mail-task store. The template dispatcher remains the sole worker;
// this adapter only performs the transactional insert.
type OutboxRepository struct {
	db        *sql.DB
	secretBox application.SecretBox
	expiresIn time.Duration
}

type outboxEnqueueOptions struct {
	userID *string
}

type OutboxOption func(*outboxEnqueueOptions)

func WithOutboxUserID(userID string) OutboxOption {
	return func(options *outboxEnqueueOptions) {
		if strings.TrimSpace(userID) != "" {
			options.userID = &userID
		}
	}
}

func NewOutboxRepository(db *sql.DB, secretBox application.SecretBox) *OutboxRepository {
	return &OutboxRepository{db: db, secretBox: secretBox, expiresIn: 30 * 24 * time.Hour}
}

func (r *OutboxRepository) EnqueueTx(ctx context.Context, tx *sql.Tx, msg email.Message, opts ...OutboxOption) error {
	if r == nil || tx == nil || r.secretBox == nil {
		return application.ErrDependencyUnavailable
	}
	options := outboxEnqueueOptions{}
	for _, option := range opts {
		if option != nil {
			option(&options)
		}
	}
	createdAt := time.Now().UTC()
	expiresAt := createdAt.Add(r.expiresIn)
	language := email.NormalizeLanguage(msg.Language)
	locale := domain.LocaleEnglish
	if language == "zh" {
		locale = domain.LocaleChinese
	}
	systemName := strings.TrimSpace(msg.SystemName)
	if systemName == "" {
		systemName = email.DefaultProductName
	}
	material := application.MailTaskMaterial{
		Version:          1,
		Kind:             application.MailTest,
		Name:             msg.Name,
		Email:            msg.To,
		Locale:           locale,
		CreatedAt:        createdAt,
		ExpiresAt:        expiresAt,
		SystemName:       systemName,
		OrganizationName: strings.TrimSpace(msg.OrganizationName),
		Message: &application.OutgoingMail{
			Kind:             application.MailTest,
			SystemName:       systemName,
			OrganizationName: strings.TrimSpace(msg.OrganizationName),
			Name:             msg.Name,
			To:               msg.To,
			Locale:           locale,
			Subject:          msg.Subject,
			Text:             msg.Body,
			HTML:             msg.HTMLBody,
		},
	}
	ciphertext, err := application.SealMailTaskMaterial(r.secretBox, material)
	if err != nil {
		return err
	}
	// Rota notifications use the test_email material form: unlike credential
	// mail, the complete rendered body is immutable and has no account
	// authority pair. The optional user ID is intentionally not persisted as a
	// foreign key because the template task constraint forbids one for this
	// material kind; it remains useful to callers for API parity only.
	_, err = tx.ExecContext(ctx, `
		INSERT INTO auth_mail_outbox (
			kind, recipient_email, recipient_name, locale, system_name,
			organization_name, material_ciphertext, round_number, round_attempt_count, expires_at, created_at
		)
		VALUES ('test_email', $1, NULLIF($8, ''), $4, $2, $3, $5, 1, 0, $6, $7)`,
		msg.To, systemName, strings.TrimSpace(msg.OrganizationName), locale, ciphertext, expiresAt, createdAt, msg.Name)
	return err
}
