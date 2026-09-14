package service

import (
	"context"
	"database/sql"

	"github.com/jonathanhu237/rota/api/internal/rota/email"
	"github.com/jonathanhu237/rota/api/internal/rota/repository"
)

type setupOutboxRepository interface {
	EnqueueTx(ctx context.Context, tx *sql.Tx, msg email.Message, opts ...repository.OutboxOption) error
}
