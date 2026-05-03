package main

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/audit"
	"github.com/jonathanhu237/rota/backend/internal/audit/audittest"
	"github.com/jonathanhu237/rota/backend/internal/email"
	"github.com/jonathanhu237/rota/backend/internal/repository"
)

func TestOutboxBackoff(t *testing.T) {
	tests := []struct {
		retryCount int
		want       time.Duration
	}{
		{retryCount: 1, want: 2 * time.Minute},
		{retryCount: 3, want: 8 * time.Minute},
		{retryCount: 6, want: time.Hour},
		{retryCount: 8, want: time.Hour},
	}

	for _, tt := range tests {
		got := outboxBackoff(tt.retryCount)
		if got != tt.want {
			t.Fatalf("outboxBackoff(%d) = %s, want %s", tt.retryCount, got, tt.want)
		}
	}
}

func TestOutboxWorkerProcessJobSuccessMarksSent(t *testing.T) {
	repo := &outboxWorkerRepoMock{}
	emailer := &workerEmailerMock{}

	processOutboxJob(
		context.Background(),
		repo,
		emailer,
		slog.New(slog.DiscardHandler),
		repository.OutboxJob{
			ID:        11,
			Kind:      email.KindPasswordReset,
			Recipient: "user@example.com",
			Subject:   "Subject",
			Body:      "Body",
			HTMLBody:  "<p>Body</p>",
		},
	)

	if repo.sentID != 11 {
		t.Fatalf("sentID = %d, want 11", repo.sentID)
	}
	if repo.retryableID != 0 || repo.failedID != 0 {
		t.Fatalf("unexpected retry/failed calls: retryable=%d failed=%d", repo.retryableID, repo.failedID)
	}
	msg := emailer.lastMessage()
	if msg.Kind != email.KindPasswordReset || msg.HTMLBody != "<p>Body</p>" {
		t.Fatalf("unexpected message passed to emailer: %+v", msg)
	}
}

func TestOutboxWorkerProcessJobFailureSchedulesRetry(t *testing.T) {
	repo := &outboxWorkerRepoMock{}
	emailer := &workerEmailerMock{err: errors.New("smtp down")}
	before := time.Now()

	processOutboxJob(
		context.Background(),
		repo,
		emailer,
		slog.New(slog.DiscardHandler),
		repository.OutboxJob{ID: 12, Recipient: "user@example.com", Subject: "Subject", Body: "Body", RetryCount: 2},
	)

	if repo.retryableID != 12 {
		t.Fatalf("retryableID = %d, want 12", repo.retryableID)
	}
	if repo.lastError != "smtp down" {
		t.Fatalf("lastError = %q, want smtp down", repo.lastError)
	}
	minNext := before.Add(8 * time.Minute)
	maxNext := time.Now().Add(8*time.Minute + time.Second)
	if repo.nextAttemptAt.Before(minNext) || repo.nextAttemptAt.After(maxNext) {
		t.Fatalf("nextAttemptAt = %s, want around +8m", repo.nextAttemptAt)
	}
}

func TestOutboxWorkerFailureMarksFailedAndAuditsInvitation(t *testing.T) {
	repo := &outboxWorkerRepoMock{}
	emailer := &workerEmailerMock{err: errors.New("smtp rejected")}
	stub := audittest.New()
	ctx := stub.ContextWith(context.Background())
	userID := int64(42)

	processOutboxJob(
		ctx,
		repo,
		emailer,
		slog.New(slog.DiscardHandler),
		repository.OutboxJob{
			ID:         13,
			UserID:     &userID,
			Kind:       email.KindInvitation,
			Recipient:  "invitee@example.com",
			Subject:    "[Rota] Rota 邀请",
			Body:       "Body",
			RetryCount: 7,
		},
	)

	if repo.failedID != 13 {
		t.Fatalf("failedID = %d, want 13", repo.failedID)
	}
	event := stub.FindByAction(audit.ActionUserInvitationEmailFailed)
	if event == nil {
		t.Fatalf("expected invitation failure audit, got actions=%v", stub.Actions())
	}
	if event.TargetID == nil || *event.TargetID != userID {
		t.Fatalf("targetID = %v, want %d", event.TargetID, userID)
	}
	if event.Metadata["email"] != "invitee@example.com" || event.Metadata["error"] != "smtp rejected" {
		t.Fatalf("unexpected metadata: %+v", event.Metadata)
	}
}

func TestOutboxWorkerTimeoutSchedulesRetry(t *testing.T) {
	repo := &outboxWorkerRepoMock{}
	emailer := &workerEmailerMock{waitForContext: true}

	processOutboxJobWithTimeout(
		context.Background(),
		repo,
		emailer,
		slog.New(slog.DiscardHandler),
		repository.OutboxJob{ID: 15, Recipient: "user@example.com", Subject: "Subject", Body: "Body"},
		time.Millisecond,
	)

	if repo.retryableID != 15 {
		t.Fatalf("retryableID = %d, want 15", repo.retryableID)
	}
	if !strings.Contains(repo.lastError, "deadline exceeded") {
		t.Fatalf("lastError = %q, want deadline exceeded", repo.lastError)
	}
}

func TestOutboxWorkerProcessJobRecoversPanic(t *testing.T) {
	repo := &outboxWorkerRepoMock{}
	emailer := &workerEmailerMock{panicValue: "boom"}

	processOutboxJob(
		context.Background(),
		repo,
		emailer,
		slog.New(slog.DiscardHandler),
		repository.OutboxJob{ID: 14, Recipient: "user@example.com", Subject: "Subject", Body: "Body"},
	)

	if repo.retryableID != 14 {
		t.Fatalf("retryableID = %d, want 14", repo.retryableID)
	}
}

type outboxWorkerRepoMock struct {
	mu            sync.Mutex
	sentID        int64
	retryableID   int64
	failedID      int64
	lastError     string
	nextAttemptAt time.Time
}

func (m *outboxWorkerRepoMock) Claim(ctx context.Context, batchSize int) ([]repository.OutboxJob, error) {
	return nil, nil
}

func (m *outboxWorkerRepoMock) MarkSent(ctx context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentID = id
	return nil
}

func (m *outboxWorkerRepoMock) MarkRetryable(
	ctx context.Context,
	id int64,
	lastError string,
	nextAttemptAt time.Time,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.retryableID = id
	m.lastError = lastError
	m.nextAttemptAt = nextAttemptAt
	return nil
}

func (m *outboxWorkerRepoMock) MarkFailed(ctx context.Context, id int64, lastError string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failedID = id
	m.lastError = lastError
	return nil
}

type workerEmailerMock struct {
	mu             sync.Mutex
	err            error
	panicValue     any
	waitForContext bool
	messages       []email.Message
}

func (m *workerEmailerMock) Send(ctx context.Context, msg email.Message) error {
	m.mu.Lock()
	m.messages = append(m.messages, msg)
	m.mu.Unlock()

	if m.panicValue != nil {
		panic(m.panicValue)
	}
	if m.waitForContext {
		<-ctx.Done()
		return ctx.Err()
	}
	return m.err
}

func (m *workerEmailerMock) lastMessage() email.Message {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.messages) == 0 {
		return email.Message{}
	}
	return m.messages[len(m.messages)-1]
}
