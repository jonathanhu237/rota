-- +goose Up
-- +goose StatementBegin

CREATE TABLE leaves (
    id                       BIGSERIAL PRIMARY KEY,
    user_id                  BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    publication_id           BIGINT NOT NULL REFERENCES publications(id) ON DELETE CASCADE,
    shift_change_request_id  BIGINT NOT NULL UNIQUE
                                 REFERENCES shift_change_requests(id) ON DELETE CASCADE,
    category                 TEXT NOT NULL CHECK (category IN ('sick','personal','bereavement')),
    reason                   TEXT NOT NULL DEFAULT '',
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX leaves_user_id_idx ON leaves(user_id);
CREATE INDEX leaves_publication_id_idx ON leaves(publication_id);

ALTER TABLE shift_change_requests
    ADD COLUMN leave_id BIGINT REFERENCES leaves(id) ON DELETE SET NULL;

CREATE INDEX shift_change_requests_leave_id_idx ON shift_change_requests(leave_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX shift_change_requests_leave_id_idx;
ALTER TABLE shift_change_requests DROP COLUMN leave_id;

DROP INDEX leaves_publication_id_idx;
DROP INDEX leaves_user_id_idx;
DROP TABLE leaves;

-- +goose StatementEnd
