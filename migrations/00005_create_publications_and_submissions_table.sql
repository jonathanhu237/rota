-- +goose Up
-- +goose StatementBegin
CREATE TABLE publications (
    id                  BIGSERIAL PRIMARY KEY,
    template_id         BIGINT NOT NULL,
    name                TEXT NOT NULL,
    state               TEXT NOT NULL,
    submission_start_at TIMESTAMP WITH TIME ZONE NOT NULL,
    submission_end_at   TIMESTAMP WITH TIME ZONE NOT NULL,
    planned_active_from TIMESTAMP WITH TIME ZONE NOT NULL,
    activated_at        TIMESTAMP WITH TIME ZONE NULL,
    ended_at            TIMESTAMP WITH TIME ZONE NULL,
    created_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT publications_template_id_fkey
        FOREIGN KEY (template_id) REFERENCES templates (id) ON DELETE RESTRICT,
    CONSTRAINT publications_state_chk
        CHECK (state IN ('DRAFT', 'COLLECTING', 'ASSIGNING', 'ACTIVE', 'ENDED')),
    CONSTRAINT publications_submission_window_check
        CHECK (submission_start_at < submission_end_at AND submission_end_at <= planned_active_from)
);

CREATE UNIQUE INDEX publications_single_non_ended_idx
    ON publications ((TRUE))
    WHERE state != 'ENDED';

CREATE TABLE availability_submissions (
    id                BIGSERIAL PRIMARY KEY,
    publication_id    BIGINT NOT NULL,
    user_id           BIGINT NOT NULL,
    template_shift_id BIGINT NOT NULL,
    created_at        TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
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
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX availability_submissions_publication_shift_idx;
DROP INDEX availability_submissions_publication_user_idx;
DROP INDEX availability_submissions_publication_user_shift_uidx;
DROP TABLE availability_submissions;
DROP INDEX publications_single_non_ended_idx;
DROP TABLE publications;
-- +goose StatementEnd
