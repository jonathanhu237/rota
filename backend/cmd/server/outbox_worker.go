package main

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/audit"
	"github.com/jonathanhu237/rota/backend/internal/email"
	"github.com/jonathanhu237/rota/backend/internal/repository"
)

const (
	outboxPollInterval = 5 * time.Second
	outboxBatchSize    = 10
	outboxMaxRetries   = 8
	outboxMaxBackoff   = time.Hour
)

type outboxWorkerRepository interface {
	Claim(ctx context.Context, batchSize int) ([]repository.OutboxJob, error)
	MarkSent(ctx context.Context, id int64) error
	MarkRetryable(ctx context.Context, id int64, lastError string, nextAttemptAt time.Time) error
	MarkFailed(ctx context.Context, id int64, lastError string) error
}

func RunOutboxWorker(
	ctx context.Context,
	outboxRepo outboxWorkerRepository,
	emailer email.Emailer,
	logger *slog.Logger,
) {
	if logger == nil {
		logger = slog.Default()
	}
	if outboxRepo == nil || emailer == nil {
		logger.Warn("outbox worker disabled: missing dependency")
		return
	}

	logger.Info("outbox worker started", "poll_interval", outboxPollInterval.String(), "batch_size", outboxBatchSize)
	go func() {
		ticker := time.NewTicker(outboxPollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				logger.Info("outbox worker stopped")
				return
			case <-ticker.C:
				processOutboxTick(ctx, outboxRepo, emailer, logger)
			}
		}
	}()
}

func processOutboxTick(
	ctx context.Context,
	outboxRepo outboxWorkerRepository,
	emailer email.Emailer,
	logger *slog.Logger,
) {
	jobs, err := outboxRepo.Claim(ctx, outboxBatchSize)
	if err != nil {
		logger.Error("outbox claim failed", "error", err)
		return
	}

	for _, job := range jobs {
		processOutboxJob(ctx, outboxRepo, emailer, logger, job)
	}
}

func processOutboxJob(
	ctx context.Context,
	outboxRepo outboxWorkerRepository,
	emailer email.Emailer,
	logger *slog.Logger,
	job repository.OutboxJob,
) {
	var sendErr error
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				sendErr = fmt.Errorf("panic sending email: %v", recovered)
			}
		}()
		sendErr = emailer.Send(ctx, email.Message{
			To:      job.Recipient,
			Subject: job.Subject,
			Body:    job.Body,
		})
	}()

	if sendErr == nil {
		if err := outboxRepo.MarkSent(ctx, job.ID); err != nil {
			logger.Error("outbox mark sent failed", "outbox_id", job.ID, "error", err)
		}
		return
	}

	lastError := truncateOutboxError(sendErr.Error())
	nextRetryCount := job.RetryCount + 1
	if nextRetryCount >= outboxMaxRetries {
		if err := outboxRepo.MarkFailed(ctx, job.ID, lastError); err != nil {
			logger.Error("outbox mark failed failed", "outbox_id", job.ID, "error", err)
			return
		}
		recordOutboxFailureAudit(ctx, job, lastError)
		return
	}

	nextAttemptAt := time.Now().Add(outboxBackoff(nextRetryCount))
	if err := outboxRepo.MarkRetryable(ctx, job.ID, lastError, nextAttemptAt); err != nil {
		logger.Error("outbox mark retryable failed", "outbox_id", job.ID, "error", err)
	}
}

func outboxBackoff(retryCount int) time.Duration {
	if retryCount <= 0 {
		return 0
	}
	minutes := math.Pow(2, float64(retryCount))
	backoff := time.Duration(minutes) * time.Minute
	if backoff > outboxMaxBackoff {
		return outboxMaxBackoff
	}
	return backoff
}

func truncateOutboxError(value string) string {
	const maxLength = 1000
	if len(value) <= maxLength {
		return value
	}
	return value[:maxLength]
}

func recordOutboxFailureAudit(ctx context.Context, job repository.OutboxJob, lastError string) {
	if job.UserID == nil || !isInvitationOutboxJob(job) {
		return
	}

	targetID := *job.UserID
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionUserInvitationEmailFailed,
		TargetType: audit.TargetTypeUser,
		TargetID:   &targetID,
		Metadata: map[string]any{
			"email": job.Recipient,
			"error": lastError,
		},
	})
}

func isInvitationOutboxJob(job repository.OutboxJob) bool {
	return job.Subject == email.BuildInvitationMessage(email.TemplateData{}).Subject
}
