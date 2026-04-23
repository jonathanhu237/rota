-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE template_slots (
    id         BIGSERIAL PRIMARY KEY,
    template_id BIGINT NOT NULL,
    weekday    INTEGER NOT NULL CHECK (weekday BETWEEN 1 AND 7),
    start_time TIME NOT NULL,
    end_time   TIME NOT NULL CHECK (end_time > start_time),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT template_slots_template_id_fkey
        FOREIGN KEY (template_id) REFERENCES templates (id) ON DELETE CASCADE,
    CONSTRAINT template_slots_template_weekday_start_end_key
        UNIQUE (template_id, weekday, start_time, end_time),
    CONSTRAINT template_slots_no_overlap_excl
        EXCLUDE USING gist (
            template_id WITH =,
            weekday WITH =,
            tsrange(
                ('2000-01-01'::date + start_time)::timestamp,
                ('2000-01-01'::date + end_time)::timestamp,
                '[)'
            ) WITH &&
        )
);

CREATE INDEX template_slots_template_weekday_idx
    ON template_slots (template_id, weekday, start_time);

CREATE TABLE template_slot_positions (
    id                 BIGSERIAL PRIMARY KEY,
    slot_id            BIGINT NOT NULL,
    position_id        BIGINT NOT NULL,
    required_headcount INTEGER NOT NULL CHECK (required_headcount > 0),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT template_slot_positions_slot_id_fkey
        FOREIGN KEY (slot_id) REFERENCES template_slots (id) ON DELETE CASCADE,
    CONSTRAINT template_slot_positions_position_id_fkey
        FOREIGN KEY (position_id) REFERENCES positions (id) ON DELETE RESTRICT,
    CONSTRAINT template_slot_positions_slot_position_key
        UNIQUE (slot_id, position_id)
);

DROP TABLE assignments;

CREATE TABLE assignments (
    id             BIGSERIAL PRIMARY KEY,
    publication_id BIGINT NOT NULL,
    user_id        BIGINT NOT NULL,
    slot_id        BIGINT NOT NULL,
    position_id    BIGINT NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT assignments_publication_id_fkey
        FOREIGN KEY (publication_id) REFERENCES publications (id) ON DELETE CASCADE,
    CONSTRAINT assignments_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT assignments_slot_id_fkey
        FOREIGN KEY (slot_id) REFERENCES template_slots (id) ON DELETE CASCADE,
    CONSTRAINT assignments_position_id_fkey
        FOREIGN KEY (position_id) REFERENCES positions (id) ON DELETE RESTRICT,
    CONSTRAINT assignments_publication_user_slot_key
        UNIQUE (publication_id, user_id, slot_id)
);

CREATE INDEX assignments_publication_slot_idx
    ON assignments (publication_id, slot_id);

CREATE INDEX assignments_publication_user_idx
    ON assignments (publication_id, user_id);

CREATE OR REPLACE FUNCTION assignments_position_belongs_to_slot()
RETURNS TRIGGER AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM template_slot_positions
        WHERE slot_id = NEW.slot_id
          AND position_id = NEW.position_id
    ) THEN
        RAISE EXCEPTION 'position % is not part of slot %', NEW.position_id, NEW.slot_id;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER assignments_position_belongs_to_slot_trigger
    BEFORE INSERT OR UPDATE ON assignments
    FOR EACH ROW
    EXECUTE FUNCTION assignments_position_belongs_to_slot();

ALTER TABLE availability_submissions
    DROP COLUMN template_shift_id;

ALTER TABLE availability_submissions
    ADD COLUMN slot_id BIGINT NOT NULL,
    ADD COLUMN position_id BIGINT NOT NULL,
    ADD CONSTRAINT availability_submissions_slot_id_fkey
        FOREIGN KEY (slot_id) REFERENCES template_slots (id) ON DELETE CASCADE,
    ADD CONSTRAINT availability_submissions_position_id_fkey
        FOREIGN KEY (position_id) REFERENCES positions (id) ON DELETE RESTRICT,
    ADD CONSTRAINT availability_submissions_slot_position_fkey
        FOREIGN KEY (slot_id, position_id)
        REFERENCES template_slot_positions (slot_id, position_id)
        ON DELETE CASCADE;

CREATE UNIQUE INDEX availability_submissions_publication_user_slot_position_uidx
    ON availability_submissions (publication_id, user_id, slot_id, position_id);

CREATE INDEX availability_submissions_publication_slot_idx
    ON availability_submissions (publication_id, slot_id);

DROP TABLE template_shifts;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS assignments_position_belongs_to_slot_trigger ON assignments;
DROP FUNCTION IF EXISTS assignments_position_belongs_to_slot();

DROP TABLE assignments;
DROP TABLE availability_submissions;

DROP TABLE template_slot_positions;
DROP INDEX IF EXISTS template_slots_template_weekday_idx;
DROP TABLE template_slots;
DROP EXTENSION IF EXISTS btree_gist;

CREATE TABLE template_shifts (
    id                 BIGSERIAL PRIMARY KEY,
    template_id        BIGINT NOT NULL,
    weekday            INTEGER NOT NULL CHECK (weekday BETWEEN 1 AND 7),
    start_time         TIME NOT NULL,
    end_time           TIME NOT NULL CHECK (end_time > start_time),
    position_id        BIGINT NOT NULL,
    required_headcount INTEGER NOT NULL CHECK (required_headcount > 0),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT template_shifts_template_id_fkey
        FOREIGN KEY (template_id) REFERENCES templates (id) ON DELETE CASCADE,
    CONSTRAINT template_shifts_position_id_fkey
        FOREIGN KEY (position_id) REFERENCES positions (id) ON DELETE RESTRICT
);

CREATE INDEX template_shifts_template_id_weekday_start_time_idx
    ON template_shifts (template_id, weekday, start_time);

CREATE TABLE availability_submissions (
    id                BIGSERIAL PRIMARY KEY,
    publication_id    BIGINT NOT NULL,
    user_id           BIGINT NOT NULL,
    template_shift_id BIGINT NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT availability_submissions_publication_id_fkey
        FOREIGN KEY (publication_id) REFERENCES publications (id) ON DELETE CASCADE,
    CONSTRAINT availability_submissions_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT availability_submissions_template_shift_id_fkey
        FOREIGN KEY (template_shift_id) REFERENCES template_shifts (id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX availability_submissions_publication_user_shift_uidx
    ON availability_submissions (publication_id, user_id, template_shift_id);

CREATE INDEX availability_submissions_publication_user_idx
    ON availability_submissions (publication_id, user_id);

CREATE INDEX availability_submissions_publication_shift_idx
    ON availability_submissions (publication_id, template_shift_id);

CREATE TABLE assignments (
    id                BIGSERIAL PRIMARY KEY,
    publication_id    BIGINT NOT NULL,
    user_id           BIGINT NOT NULL,
    template_shift_id BIGINT NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT assignments_publication_id_fkey
        FOREIGN KEY (publication_id) REFERENCES publications (id) ON DELETE CASCADE,
    CONSTRAINT assignments_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT assignments_template_shift_id_fkey
        FOREIGN KEY (template_shift_id) REFERENCES template_shifts (id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX assignments_publication_user_shift_uidx
    ON assignments (publication_id, user_id, template_shift_id);

CREATE INDEX assignments_publication_shift_idx
    ON assignments (publication_id, template_shift_id);

CREATE INDEX assignments_publication_user_idx
    ON assignments (publication_id, user_id);
-- +goose StatementEnd
