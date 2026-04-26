-- +goose Up
-- +goose StatementBegin

TRUNCATE TABLE
    leaves,
    shift_change_requests,
    assignment_overrides,
    assignments,
    availability_submissions,
    template_slot_positions,
    template_slots
RESTART IDENTITY CASCADE;

CREATE TABLE template_slot_weekdays (
    slot_id BIGINT NOT NULL,
    weekday INTEGER NOT NULL CHECK (weekday BETWEEN 1 AND 7),
    PRIMARY KEY (slot_id, weekday),
    CONSTRAINT template_slot_weekdays_slot_id_fkey
        FOREIGN KEY (slot_id) REFERENCES template_slots (id) ON DELETE CASCADE
);

ALTER TABLE template_slots
    DROP CONSTRAINT IF EXISTS template_slots_template_weekday_start_end_key,
    DROP CONSTRAINT IF EXISTS template_slots_no_overlap_excl;

DROP INDEX IF EXISTS template_slots_template_weekday_idx;

ALTER TABLE template_slots
    DROP COLUMN weekday;

CREATE INDEX template_slots_template_start_idx
    ON template_slots (template_id, start_time);

ALTER TABLE availability_submissions
    DROP CONSTRAINT IF EXISTS availability_submissions_publication_user_slot_uidx;

ALTER TABLE availability_submissions
    ADD COLUMN weekday INTEGER NOT NULL CHECK (weekday BETWEEN 1 AND 7),
    ADD CONSTRAINT availability_submissions_publication_user_slot_weekday_key
        UNIQUE (publication_id, user_id, slot_id, weekday),
    ADD CONSTRAINT availability_submissions_slot_weekday_fkey
        FOREIGN KEY (slot_id, weekday)
        REFERENCES template_slot_weekdays (slot_id, weekday)
        ON DELETE CASCADE;

DROP INDEX IF EXISTS availability_submissions_publication_slot_idx;

CREATE INDEX availability_submissions_publication_slot_weekday_idx
    ON availability_submissions (publication_id, slot_id, weekday);

ALTER TABLE assignments
    DROP CONSTRAINT IF EXISTS assignments_publication_user_slot_key;

ALTER TABLE assignments
    ADD COLUMN weekday INTEGER NOT NULL CHECK (weekday BETWEEN 1 AND 7),
    ADD CONSTRAINT assignments_publication_user_slot_weekday_key
        UNIQUE (publication_id, user_id, slot_id, weekday),
    ADD CONSTRAINT assignments_slot_weekday_fkey
        FOREIGN KEY (slot_id, weekday)
        REFERENCES template_slot_weekdays (slot_id, weekday)
        ON DELETE CASCADE;

DROP INDEX IF EXISTS assignments_publication_slot_idx;

CREATE INDEX assignments_publication_slot_weekday_idx
    ON assignments (publication_id, slot_id, weekday);

CREATE OR REPLACE FUNCTION template_slot_weekday_no_overlap()
RETURNS trigger AS $$
DECLARE
    conflict_slot_id BIGINT;
BEGIN
    SELECT other.id INTO conflict_slot_id
    FROM template_slots me
    INNER JOIN template_slots other
        ON other.template_id = me.template_id
       AND other.id <> me.id
       AND tsrange(
               ('2000-01-01'::date + me.start_time)::timestamp,
               ('2000-01-01'::date + me.end_time)::timestamp,
               '[)'
           ) &&
           tsrange(
               ('2000-01-01'::date + other.start_time)::timestamp,
               ('2000-01-01'::date + other.end_time)::timestamp,
               '[)'
           )
    INNER JOIN template_slot_weekdays other_wd
        ON other_wd.slot_id = other.id
       AND other_wd.weekday = NEW.weekday
    WHERE me.id = NEW.slot_id
    LIMIT 1;

    IF conflict_slot_id IS NOT NULL THEN
        RAISE EXCEPTION 'overlapping slot weekday'
            USING ERRCODE = '23P01';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER template_slot_weekdays_no_overlap_trg
    BEFORE INSERT OR UPDATE ON template_slot_weekdays
    FOR EACH ROW
    EXECUTE FUNCTION template_slot_weekday_no_overlap();

DO $$
BEGIN
    RAISE NOTICE 'decouple-weekday-from-slot: tables truncated; run "make seed SCENARIO=..." to repopulate.';
END $$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

TRUNCATE TABLE
    leaves,
    shift_change_requests,
    assignment_overrides,
    assignments,
    availability_submissions,
    template_slot_positions,
    template_slots
RESTART IDENTITY CASCADE;

DROP TRIGGER IF EXISTS template_slot_weekdays_no_overlap_trg ON template_slot_weekdays;
DROP FUNCTION IF EXISTS template_slot_weekday_no_overlap();

ALTER TABLE assignments
    DROP CONSTRAINT IF EXISTS assignments_slot_weekday_fkey,
    DROP CONSTRAINT IF EXISTS assignments_publication_user_slot_weekday_key,
    DROP COLUMN IF EXISTS weekday,
    ADD CONSTRAINT assignments_publication_user_slot_key
        UNIQUE (publication_id, user_id, slot_id);

DROP INDEX IF EXISTS assignments_publication_slot_weekday_idx;

CREATE INDEX assignments_publication_slot_idx
    ON assignments (publication_id, slot_id);

ALTER TABLE availability_submissions
    DROP CONSTRAINT IF EXISTS availability_submissions_slot_weekday_fkey,
    DROP CONSTRAINT IF EXISTS availability_submissions_publication_user_slot_weekday_key,
    DROP COLUMN IF EXISTS weekday,
    ADD CONSTRAINT availability_submissions_publication_user_slot_uidx
        UNIQUE (publication_id, user_id, slot_id);

DROP INDEX IF EXISTS availability_submissions_publication_slot_weekday_idx;

CREATE INDEX availability_submissions_publication_slot_idx
    ON availability_submissions (publication_id, slot_id);

DROP INDEX IF EXISTS template_slots_template_start_idx;

ALTER TABLE template_slots
    DROP CONSTRAINT IF EXISTS template_slots_template_start_end_key,
    ADD COLUMN weekday INTEGER NOT NULL DEFAULT 1 CHECK (weekday BETWEEN 1 AND 7);

ALTER TABLE template_slots
    ALTER COLUMN weekday DROP DEFAULT,
    ADD CONSTRAINT template_slots_template_weekday_start_end_key
        UNIQUE (template_id, weekday, start_time, end_time),
    ADD CONSTRAINT template_slots_no_overlap_excl
        EXCLUDE USING gist (
            template_id WITH =,
            weekday WITH =,
            tsrange(
                ('2000-01-01'::date + start_time)::timestamp,
                ('2000-01-01'::date + end_time)::timestamp,
                '[)'
            ) WITH &&
        );

CREATE INDEX template_slots_template_weekday_idx
    ON template_slots (template_id, weekday, start_time);

DROP TABLE template_slot_weekdays;

-- +goose StatementEnd
