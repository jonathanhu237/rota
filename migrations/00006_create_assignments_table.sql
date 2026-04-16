-- +goose Up
-- +goose StatementBegin
CREATE TABLE assignments (
    id                BIGSERIAL PRIMARY KEY,
    publication_id    BIGINT NOT NULL,
    user_id           BIGINT NOT NULL,
    template_shift_id BIGINT NOT NULL,
    created_at        TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
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

-- +goose Down
-- +goose StatementBegin
DROP INDEX assignments_publication_user_idx;
DROP INDEX assignments_publication_shift_idx;
DROP INDEX assignments_publication_user_shift_uidx;
DROP TABLE assignments;
-- +goose StatementEnd
