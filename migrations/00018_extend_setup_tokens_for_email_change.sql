-- +goose Up
-- +goose StatementBegin
ALTER TABLE user_setup_tokens
    DROP CONSTRAINT user_setup_tokens_purpose_check;

ALTER TABLE user_setup_tokens
    ADD CONSTRAINT user_setup_tokens_purpose_check
    CHECK (purpose IN ('invitation', 'password_reset', 'email_change'));

ALTER TABLE user_setup_tokens
    ADD COLUMN new_email TEXT NULL;

ALTER TABLE user_setup_tokens
    ADD CONSTRAINT user_setup_tokens_email_change_has_new_email
    CHECK (
        (purpose = 'email_change' AND new_email IS NOT NULL)
        OR (purpose <> 'email_change' AND new_email IS NULL)
    );
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE user_setup_tokens
    DROP CONSTRAINT user_setup_tokens_email_change_has_new_email;

-- Drop in-flight email_change tokens before re-narrowing the purpose CHECK.
-- Without this, the re-added constraint below would fail validation against
-- existing rows, and the rollback would abort. Any pending email-change
-- requests are forfeit; users who clicked the confirmation link mid-rollback
-- get TOKEN_NOT_FOUND and re-issue from /settings.
DELETE FROM user_setup_tokens
    WHERE purpose = 'email_change';

ALTER TABLE user_setup_tokens
    DROP COLUMN new_email;

ALTER TABLE user_setup_tokens
    DROP CONSTRAINT user_setup_tokens_purpose_check;

ALTER TABLE user_setup_tokens
    ADD CONSTRAINT user_setup_tokens_purpose_check
    CHECK (purpose IN ('invitation', 'password_reset'));
-- +goose StatementEnd
