-- +goose Up
-- +goose StatementBegin
ALTER TABLE publications
    DROP CONSTRAINT publications_state_chk,
    ADD CONSTRAINT publications_state_chk
        CHECK (state IN ('DRAFT', 'COLLECTING', 'ASSIGNING', 'PUBLISHED', 'ACTIVE', 'ENDED'));

CREATE TABLE shift_change_requests (
    id                        BIGSERIAL PRIMARY KEY,
    publication_id            BIGINT NOT NULL
                                  REFERENCES publications(id) ON DELETE CASCADE,
    type                      TEXT NOT NULL
                                  CHECK (type IN ('swap', 'give_direct', 'give_pool')),
    requester_user_id         BIGINT NOT NULL
                                  REFERENCES users(id),
    requester_assignment_id   BIGINT NOT NULL,
    counterpart_user_id       BIGINT
                                  REFERENCES users(id),
    counterpart_assignment_id BIGINT,
    state                     TEXT NOT NULL DEFAULT 'pending'
                                  CHECK (state IN (
                                      'pending',
                                      'approved',
                                      'rejected',
                                      'cancelled',
                                      'expired',
                                      'invalidated'
                                  )),
    decided_by_user_id        BIGINT REFERENCES users(id),
    created_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    decided_at                TIMESTAMPTZ,
    expires_at                TIMESTAMPTZ NOT NULL
);

CREATE INDEX shift_change_requests_publication_state_idx
    ON shift_change_requests (publication_id, state, created_at DESC);

CREATE INDEX shift_change_requests_requester_idx
    ON shift_change_requests (requester_user_id, state, created_at DESC);

CREATE INDEX shift_change_requests_counterpart_idx
    ON shift_change_requests (counterpart_user_id, state, created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE shift_change_requests;

UPDATE publications
SET state = 'ENDED'
WHERE state = 'PUBLISHED';

ALTER TABLE publications
    DROP CONSTRAINT publications_state_chk,
    ADD CONSTRAINT publications_state_chk
        CHECK (state IN ('DRAFT', 'COLLECTING', 'ASSIGNING', 'ACTIVE', 'ENDED'));
-- +goose StatementEnd
