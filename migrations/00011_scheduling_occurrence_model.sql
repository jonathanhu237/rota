-- +goose Up
-- +goose StatementBegin

ALTER TABLE publications
    ADD COLUMN description TEXT NOT NULL DEFAULT '',
    DROP COLUMN ended_at,
    ADD COLUMN planned_active_until TIMESTAMPTZ;

UPDATE publications
   SET planned_active_until = planned_active_from + INTERVAL '7 days'
 WHERE planned_active_until IS NULL;

ALTER TABLE publications
    ALTER COLUMN planned_active_until SET NOT NULL,
    DROP CONSTRAINT publications_submission_window_check,
    ADD CONSTRAINT publications_submission_window_check
        CHECK (submission_start_at < submission_end_at
               AND submission_end_at <= planned_active_from
               AND planned_active_from < planned_active_until);

CREATE TABLE assignment_overrides (
    id              BIGSERIAL PRIMARY KEY,
    assignment_id   BIGINT NOT NULL REFERENCES assignments(id) ON DELETE CASCADE,
    occurrence_date DATE NOT NULL,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (assignment_id, occurrence_date)
);

CREATE INDEX assignment_overrides_user_id_idx ON assignment_overrides (user_id);

ALTER TABLE shift_change_requests
    ADD COLUMN occurrence_date DATE,
    ADD COLUMN counterpart_occurrence_date DATE;

UPDATE shift_change_requests AS s
   SET occurrence_date = (SELECT planned_active_from::date
                            FROM publications p WHERE p.id = s.publication_id)
 WHERE occurrence_date IS NULL;

ALTER TABLE shift_change_requests
    ALTER COLUMN occurrence_date SET NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE shift_change_requests
    DROP COLUMN counterpart_occurrence_date,
    DROP COLUMN occurrence_date;

DROP INDEX assignment_overrides_user_id_idx;
DROP TABLE assignment_overrides;

ALTER TABLE publications
    DROP CONSTRAINT publications_submission_window_check,
    ADD CONSTRAINT publications_submission_window_check
        CHECK (submission_start_at < submission_end_at
               AND submission_end_at <= planned_active_from);

ALTER TABLE publications
    ADD COLUMN ended_at TIMESTAMPTZ,
    DROP COLUMN planned_active_until,
    DROP COLUMN description;

-- +goose StatementEnd
