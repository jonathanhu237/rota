-- +goose Up
-- +goose StatementBegin
ALTER TABLE users
    ALTER COLUMN password_hash DROP NOT NULL;

ALTER TABLE users
    DROP CONSTRAINT users_status_check,
    ADD CONSTRAINT users_status_check CHECK (status IN ('active', 'disabled', 'pending'));

CREATE TABLE user_setup_tokens (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL,
    purpose    TEXT NOT NULL CHECK (purpose IN ('invitation', 'password_reset')),
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX user_setup_tokens_token_hash_idx ON user_setup_tokens (token_hash);
CREATE INDEX user_setup_tokens_user_purpose_idx ON user_setup_tokens (user_id, purpose);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE user_setup_tokens;

UPDATE users
SET
    password_hash = COALESCE(password_hash, 'invalid'),
    status = CASE
        WHEN status = 'pending' THEN 'disabled'
        ELSE status
    END;

ALTER TABLE users
    DROP CONSTRAINT users_status_check,
    ADD CONSTRAINT users_status_check CHECK (status IN ('active', 'disabled'));

ALTER TABLE users
    ALTER COLUMN password_hash SET NOT NULL;
-- +goose StatementEnd
