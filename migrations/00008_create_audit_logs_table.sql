-- +goose Up
-- +goose StatementBegin
CREATE TABLE audit_logs (
    id          BIGSERIAL PRIMARY KEY,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    actor_id    BIGINT,
    actor_ip    TEXT,
    action      TEXT NOT NULL,
    target_type TEXT,
    target_id   BIGINT,
    metadata    JSONB NOT NULL DEFAULT '{}'
);

CREATE INDEX audit_logs_occurred_at_idx ON audit_logs (occurred_at DESC);
CREATE INDEX audit_logs_actor_idx ON audit_logs (actor_id, occurred_at DESC);
CREATE INDEX audit_logs_target_idx ON audit_logs (target_type, target_id, occurred_at DESC);
CREATE INDEX audit_logs_action_idx ON audit_logs (action, occurred_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE audit_logs;
-- +goose StatementEnd
