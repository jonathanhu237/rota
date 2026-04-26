-- +goose Up
-- +goose StatementBegin
ALTER TABLE availability_submissions
    DROP CONSTRAINT IF EXISTS availability_submissions_slot_position_fkey;

ALTER TABLE availability_submissions
    DROP CONSTRAINT IF EXISTS availability_submissions_position_id_fkey;

DROP INDEX IF EXISTS availability_submissions_publication_user_slot_position_uidx;

ALTER TABLE availability_submissions
    DROP COLUMN IF EXISTS position_id;

ALTER TABLE availability_submissions
    ADD CONSTRAINT availability_submissions_publication_user_slot_uidx
        UNIQUE (publication_id, user_id, slot_id);

CREATE INDEX IF NOT EXISTS availability_submissions_publication_slot_idx
    ON availability_submissions (publication_id, slot_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE availability_submissions
    DROP CONSTRAINT IF EXISTS availability_submissions_publication_user_slot_uidx;

ALTER TABLE availability_submissions
    ADD COLUMN position_id BIGINT;

ALTER TABLE availability_submissions
    ADD CONSTRAINT availability_submissions_position_id_fkey
        FOREIGN KEY (position_id) REFERENCES positions (id) ON DELETE RESTRICT;

ALTER TABLE availability_submissions
    ADD CONSTRAINT availability_submissions_slot_position_fkey
        FOREIGN KEY (slot_id, position_id)
        REFERENCES template_slot_positions (slot_id, position_id)
        ON DELETE CASCADE;

CREATE UNIQUE INDEX availability_submissions_publication_user_slot_position_uidx
    ON availability_submissions (publication_id, user_id, slot_id, position_id);

CREATE INDEX IF NOT EXISTS availability_submissions_publication_slot_idx
    ON availability_submissions (publication_id, slot_id);
-- +goose StatementEnd
