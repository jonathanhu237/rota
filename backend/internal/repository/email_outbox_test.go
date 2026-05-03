package repository

import (
	"database/sql"
	"testing"

	"github.com/jonathanhu237/rota/backend/internal/email"
)

func TestScanOutboxJobIncludesKindAndHTMLBody(t *testing.T) {
	t.Parallel()

	userID := int64(42)
	job, err := scanOutboxJob(outboxJobScannerStub{
		id:         7,
		userID:     sql.NullInt64{Int64: userID, Valid: true},
		kind:       email.KindInvitation,
		recipient:  "worker@example.com",
		subject:    "Subject",
		body:       "Text body",
		htmlBody:   sql.NullString{String: "<p>HTML body</p>", Valid: true},
		retryCount: 2,
	})
	if err != nil {
		t.Fatalf("scanOutboxJob returned error: %v", err)
	}

	if job.ID != 7 ||
		job.UserID == nil ||
		*job.UserID != userID ||
		job.Kind != email.KindInvitation ||
		job.Recipient != "worker@example.com" ||
		job.Subject != "Subject" ||
		job.Body != "Text body" ||
		job.HTMLBody != "<p>HTML body</p>" ||
		job.RetryCount != 2 {
		t.Fatalf("unexpected job: %+v", job)
	}
}

func TestNormalizeOutboxKindDefaultsUnknown(t *testing.T) {
	t.Parallel()

	if got := normalizeOutboxKind(""); got != email.KindUnknown {
		t.Fatalf("normalizeOutboxKind empty = %q, want %q", got, email.KindUnknown)
	}
	if got := normalizeOutboxKind(email.KindPasswordReset); got != email.KindPasswordReset {
		t.Fatalf("normalizeOutboxKind password reset = %q", got)
	}
}

type outboxJobScannerStub struct {
	id         int64
	userID     sql.NullInt64
	kind       string
	recipient  string
	subject    string
	body       string
	htmlBody   sql.NullString
	retryCount int
}

func (s outboxJobScannerStub) Scan(dest ...any) error {
	*(dest[0].(*int64)) = s.id
	*(dest[1].(*sql.NullInt64)) = s.userID
	*(dest[2].(*string)) = s.kind
	*(dest[3].(*string)) = s.recipient
	*(dest[4].(*string)) = s.subject
	*(dest[5].(*string)) = s.body
	*(dest[6].(*sql.NullString)) = s.htmlBody
	*(dest[7].(*int)) = s.retryCount
	return nil
}
