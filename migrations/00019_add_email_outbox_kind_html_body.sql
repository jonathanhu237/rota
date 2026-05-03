-- +goose Up
-- +goose StatementBegin
ALTER TABLE email_outbox
    ADD COLUMN kind TEXT NOT NULL DEFAULT 'unknown',
    ADD COLUMN html_body TEXT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE email_outbox
    DROP COLUMN html_body,
    DROP COLUMN kind;
-- +goose StatementEnd
