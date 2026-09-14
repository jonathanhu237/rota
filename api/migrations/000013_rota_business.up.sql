-- Rota business schema. Numeric IDs remain local to scheduling entities;
-- account references use canonical UUID values from Temvia auth identities.

CREATE TABLE positions (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE templates (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    is_locked BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

-- Qualifications are keyed by canonical account UUIDs. User rows are not
-- foreign-keyed to auth_users because deleted identities must remain visible
-- for historical schedules and audit views; repository writes still require a
-- live or preserved identity in the users view.
CREATE TABLE user_positions (
    user_id UUID NOT NULL,
    position_id BIGINT NOT NULL REFERENCES positions(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (user_id, position_id)
);
CREATE INDEX user_positions_position_idx ON user_positions(position_id, user_id);

CREATE TABLE template_slots (
    id BIGSERIAL PRIMARY KEY,
    template_id BIGINT NOT NULL REFERENCES templates(id) ON DELETE CASCADE,
    start_time TIME NOT NULL,
    end_time TIME NOT NULL CHECK (end_time > start_time),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE template_slot_weekdays (
    slot_id BIGINT NOT NULL REFERENCES template_slots(id) ON DELETE CASCADE,
    weekday INTEGER NOT NULL CHECK (weekday BETWEEN 1 AND 7),
    PRIMARY KEY (slot_id, weekday)
);

CREATE OR REPLACE FUNCTION template_slot_weekday_no_overlap()
RETURNS TRIGGER AS $$
DECLARE
    conflict_slot_id BIGINT;
BEGIN
    SELECT other.id INTO conflict_slot_id
    FROM template_slots me
    JOIN template_slots other
      ON other.template_id = me.template_id
     AND other.id <> me.id
     AND tsrange(
           ('2000-01-01'::date + me.start_time)::timestamp,
           ('2000-01-01'::date + me.end_time)::timestamp,
           '[)'
         ) && tsrange(
           ('2000-01-01'::date + other.start_time)::timestamp,
           ('2000-01-01'::date + other.end_time)::timestamp,
           '[)'
         )
    JOIN template_slot_weekdays other_wd
      ON other_wd.slot_id = other.id AND other_wd.weekday = NEW.weekday
    WHERE me.id = NEW.slot_id
    LIMIT 1;
    IF conflict_slot_id IS NOT NULL THEN
        RAISE EXCEPTION 'overlapping slot weekday' USING ERRCODE = '23P01';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER template_slot_weekdays_no_overlap_trg
    BEFORE INSERT OR UPDATE ON template_slot_weekdays
    FOR EACH ROW EXECUTE FUNCTION template_slot_weekday_no_overlap();

CREATE INDEX template_slots_template_start_idx ON template_slots(template_id, start_time);

CREATE TABLE template_slot_positions (
    id BIGSERIAL PRIMARY KEY,
    slot_id BIGINT NOT NULL REFERENCES template_slots(id) ON DELETE CASCADE,
    position_id BIGINT NOT NULL REFERENCES positions(id) ON DELETE RESTRICT,
    required_headcount INTEGER NOT NULL CHECK (required_headcount > 0),
    attendance_responsible BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT template_slot_positions_slot_position_key UNIQUE(slot_id, position_id),
    CONSTRAINT template_slot_positions_responsible_headcount_chk
        CHECK (attendance_responsible = FALSE OR required_headcount = 1)
);

CREATE UNIQUE INDEX template_slot_positions_one_attendance_responsible_idx
    ON template_slot_positions(slot_id) WHERE attendance_responsible;

CREATE TABLE publications (
    id BIGSERIAL PRIMARY KEY,
    template_id BIGINT NOT NULL REFERENCES templates(id) ON DELETE RESTRICT,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL CHECK (state IN ('DRAFT','COLLECTING','ASSIGNING','PUBLISHED','ACTIVE','ENDED')),
    submission_start_at TIMESTAMPTZ NOT NULL,
    submission_end_at TIMESTAMPTZ NOT NULL,
    planned_active_from TIMESTAMPTZ NOT NULL,
    planned_active_until TIMESTAMPTZ NOT NULL,
    overtime_entry_window_hours NUMERIC(5,2) NOT NULL DEFAULT 24.00
        CONSTRAINT publications_overtime_entry_window_hours_chk
        CHECK (overtime_entry_window_hours >= 0 AND overtime_entry_window_hours <= 168),
    activated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT publications_submission_window_check CHECK (
        submission_start_at < submission_end_at
        AND submission_end_at <= planned_active_from
        AND planned_active_from < planned_active_until
    )
);

CREATE UNIQUE INDEX publications_single_non_ended_idx
    ON publications((TRUE)) WHERE state <> 'ENDED';

CREATE TABLE availability_submissions (
    id BIGSERIAL PRIMARY KEY,
    publication_id BIGINT NOT NULL REFERENCES publications(id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    slot_id BIGINT NOT NULL REFERENCES template_slots(id) ON DELETE CASCADE,
    weekday INTEGER NOT NULL CHECK (weekday BETWEEN 1 AND 7),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT availability_submissions_publication_user_slot_weekday_key
        UNIQUE(publication_id, user_id, slot_id, weekday),
    CONSTRAINT availability_submissions_slot_weekday_fkey
        FOREIGN KEY(slot_id, weekday) REFERENCES template_slot_weekdays(slot_id, weekday)
        ON DELETE CASCADE
);
CREATE INDEX availability_submissions_publication_user_idx
    ON availability_submissions(publication_id, user_id);
CREATE INDEX availability_submissions_publication_slot_weekday_idx
    ON availability_submissions(publication_id, slot_id, weekday);

CREATE TABLE assignments (
    id BIGSERIAL PRIMARY KEY,
    publication_id BIGINT NOT NULL REFERENCES publications(id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    slot_id BIGINT NOT NULL REFERENCES template_slots(id) ON DELETE CASCADE,
    weekday INTEGER NOT NULL CHECK (weekday BETWEEN 1 AND 7),
    position_id BIGINT NOT NULL REFERENCES positions(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT assignments_publication_user_slot_weekday_key
        UNIQUE(publication_id, user_id, slot_id, weekday),
    CONSTRAINT assignments_slot_weekday_fkey
        FOREIGN KEY(slot_id, weekday) REFERENCES template_slot_weekdays(slot_id, weekday)
        ON DELETE CASCADE
);
CREATE INDEX assignments_publication_slot_weekday_idx ON assignments(publication_id, slot_id, weekday);
CREATE INDEX assignments_publication_user_idx ON assignments(publication_id, user_id);

CREATE OR REPLACE FUNCTION assignments_position_belongs_to_slot()
RETURNS TRIGGER AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM template_slot_positions
        WHERE slot_id = NEW.slot_id AND position_id = NEW.position_id
    ) THEN
        RAISE EXCEPTION 'position % is not part of slot %', NEW.position_id, NEW.slot_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER assignments_position_belongs_to_slot_trigger
    BEFORE INSERT OR UPDATE ON assignments
    FOR EACH ROW EXECUTE FUNCTION assignments_position_belongs_to_slot();

CREATE TABLE assignment_overrides (
    id BIGSERIAL PRIMARY KEY,
    assignment_id BIGINT NOT NULL REFERENCES assignments(id) ON DELETE CASCADE,
    occurrence_date DATE NOT NULL,
    user_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    UNIQUE(assignment_id, occurrence_date)
);
CREATE INDEX assignment_overrides_user_id_idx ON assignment_overrides(user_id);

CREATE TABLE shift_change_requests (
    id BIGSERIAL PRIMARY KEY,
    publication_id BIGINT NOT NULL REFERENCES publications(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK (type IN ('swap','give_direct','give_pool')),
    requester_user_id UUID NOT NULL,
    requester_assignment_id BIGINT NOT NULL,
    occurrence_date DATE NOT NULL,
    counterpart_user_id UUID,
    counterpart_assignment_id BIGINT,
    counterpart_occurrence_date DATE,
    state TEXT NOT NULL DEFAULT 'pending' CHECK (state IN ('pending','approved','rejected','cancelled','expired','invalidated')),
    leave_id BIGINT,
    decided_by_user_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    decided_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX shift_change_requests_publication_state_idx
    ON shift_change_requests(publication_id, state, created_at DESC);
CREATE INDEX shift_change_requests_requester_idx
    ON shift_change_requests(requester_user_id, state, created_at DESC);
CREATE INDEX shift_change_requests_counterpart_idx
    ON shift_change_requests(counterpart_user_id, state, created_at DESC);

CREATE TABLE leaves (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID NOT NULL,
    publication_id BIGINT NOT NULL REFERENCES publications(id) ON DELETE CASCADE,
    shift_change_request_id BIGINT NOT NULL UNIQUE REFERENCES shift_change_requests(id) ON DELETE CASCADE,
    category TEXT NOT NULL CHECK (category IN ('sick','personal','bereavement')),
    reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX leaves_user_id_idx ON leaves(user_id);
CREATE INDEX leaves_publication_id_idx ON leaves(publication_id);
ALTER TABLE shift_change_requests
    ADD CONSTRAINT shift_change_requests_leave_id_fkey
    FOREIGN KEY(leave_id) REFERENCES leaves(id) ON DELETE SET NULL;
CREATE INDEX shift_change_requests_leave_id_idx ON shift_change_requests(leave_id);
CREATE UNIQUE INDEX shift_change_requests_active_leave_occurrence_uidx
    ON shift_change_requests(requester_user_id, requester_assignment_id, occurrence_date)
    WHERE leave_id IS NOT NULL AND state IN ('pending','approved');

CREATE TABLE attendance_records (
    id BIGSERIAL PRIMARY KEY,
    publication_id BIGINT NOT NULL REFERENCES publications(id) ON DELETE CASCADE,
    assignment_id BIGINT NOT NULL REFERENCES assignments(id) ON DELETE CASCADE,
    occurrence_date DATE NOT NULL,
    user_id UUID NOT NULL,
    arrived_at TIMESTAMPTZ NOT NULL,
    recorded_by_user_id UUID,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_by_user_id UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT attendance_records_assignment_id_occurrence_date_user_id_key
        UNIQUE(assignment_id, occurrence_date, user_id)
);
CREATE INDEX attendance_records_publication_occurrence_idx ON attendance_records(publication_id, occurrence_date);
CREATE INDEX attendance_records_user_idx ON attendance_records(user_id, occurrence_date);

CREATE TABLE attendance_overtime_records (
    id BIGSERIAL PRIMARY KEY,
    publication_id BIGINT NOT NULL REFERENCES publications(id) ON DELETE CASCADE,
    slot_id BIGINT NOT NULL REFERENCES template_slots(id) ON DELETE CASCADE,
    weekday INTEGER NOT NULL CHECK (weekday BETWEEN 1 AND 7),
    occurrence_date DATE NOT NULL,
    user_id UUID NOT NULL,
    hours NUMERIC(5,2) NOT NULL CHECK (hours > 0 AND hours <= 24),
    note TEXT NOT NULL CHECK (btrim(note) = note AND char_length(note) BETWEEN 1 AND 500),
    recorded_by_user_id UUID,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_by_user_id UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX attendance_overtime_publication_occurrence_idx ON attendance_overtime_records(publication_id, occurrence_date);
CREATE INDEX attendance_overtime_user_idx ON attendance_overtime_records(user_id, occurrence_date);

-- Business queries retain deleted identities for historical display. Live
-- writes still filter status='active', while deleted identities have no
-- credentials or roles and can never become a scheduling candidate.
CREATE OR REPLACE VIEW users AS
SELECT
    u.id,
    u.email,
    u.name,
    EXISTS(SELECT 1 FROM auth_user_roles ur JOIN auth_roles r ON r.id = ur.role_id WHERE ur.user_id = u.id AND r.system_key = 'super_admin') AS is_admin,
    CASE WHEN u.disabled_at IS NULL THEN 'active' ELSE 'disabled' END AS status,
    1 AS version,
    CASE WHEN u.locale = 'zh-CN' THEN 'zh' WHEN u.locale = 'en' THEN 'en' ELSE NULL END AS language_preference,
    NULL::text AS theme_preference
FROM auth_users u
UNION ALL
SELECT d.user_id, d.email, d.name, false, 'disabled'::text, 1, NULL::text, NULL::text
FROM auth_deleted_user_identities d;
