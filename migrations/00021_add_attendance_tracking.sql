-- +goose Up
ALTER TABLE template_slot_positions
    ADD COLUMN attendance_responsible BOOLEAN NOT NULL DEFAULT FALSE;

CREATE UNIQUE INDEX template_slot_positions_one_attendance_responsible_idx
    ON template_slot_positions (slot_id)
    WHERE attendance_responsible;

ALTER TABLE template_slot_positions
    ADD CONSTRAINT template_slot_positions_responsible_headcount_chk
        CHECK (attendance_responsible = FALSE OR required_headcount = 1);

ALTER TABLE publications
    ADD COLUMN overtime_entry_window_hours NUMERIC(5,2) NOT NULL DEFAULT 24.00,
    ADD CONSTRAINT publications_overtime_entry_window_hours_chk
        CHECK (overtime_entry_window_hours >= 0 AND overtime_entry_window_hours <= 168);

CREATE TABLE attendance_records (
    id BIGSERIAL PRIMARY KEY,
    publication_id BIGINT NOT NULL REFERENCES publications(id) ON DELETE CASCADE,
    assignment_id BIGINT NOT NULL REFERENCES assignments(id) ON DELETE CASCADE,
    occurrence_date DATE NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    arrived_at TIMESTAMPTZ NOT NULL,
    recorded_by_user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by_user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (assignment_id, occurrence_date, user_id)
);

CREATE INDEX attendance_records_publication_occurrence_idx
    ON attendance_records (publication_id, occurrence_date);

CREATE INDEX attendance_records_user_idx
    ON attendance_records (user_id, occurrence_date);

CREATE TABLE attendance_overtime_records (
    id BIGSERIAL PRIMARY KEY,
    publication_id BIGINT NOT NULL REFERENCES publications(id) ON DELETE CASCADE,
    slot_id BIGINT NOT NULL REFERENCES template_slots(id) ON DELETE CASCADE,
    weekday INTEGER NOT NULL CHECK (weekday BETWEEN 1 AND 7),
    occurrence_date DATE NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    hours NUMERIC(5,2) NOT NULL CHECK (hours > 0 AND hours <= 24),
    note TEXT NOT NULL CHECK (btrim(note) = note AND char_length(note) BETWEEN 1 AND 500),
    recorded_by_user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by_user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX attendance_overtime_publication_occurrence_idx
    ON attendance_overtime_records (publication_id, occurrence_date);

CREATE INDEX attendance_overtime_user_idx
    ON attendance_overtime_records (user_id, occurrence_date);

-- +goose Down
DROP INDEX IF EXISTS attendance_overtime_user_idx;
DROP INDEX IF EXISTS attendance_overtime_publication_occurrence_idx;
DROP TABLE IF EXISTS attendance_overtime_records;

DROP INDEX IF EXISTS attendance_records_user_idx;
DROP INDEX IF EXISTS attendance_records_publication_occurrence_idx;
DROP TABLE IF EXISTS attendance_records;

ALTER TABLE publications
    DROP CONSTRAINT IF EXISTS publications_overtime_entry_window_hours_chk,
    DROP COLUMN IF EXISTS overtime_entry_window_hours;

DROP INDEX IF EXISTS template_slot_positions_one_attendance_responsible_idx;

ALTER TABLE template_slot_positions
    DROP CONSTRAINT IF EXISTS template_slot_positions_responsible_headcount_chk,
    DROP COLUMN IF EXISTS attendance_responsible;
