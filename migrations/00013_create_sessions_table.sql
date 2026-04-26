-- +goose Up
-- +goose StatementBegin

CREATE TABLE sessions (
    id          TEXT PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX sessions_user_id_idx ON sessions(user_id);
CREATE INDEX sessions_expires_at_idx ON sessions(expires_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS sessions_expires_at_idx;
DROP INDEX IF EXISTS sessions_user_id_idx;
DROP TABLE IF EXISTS sessions;

-- +goose StatementEnd
