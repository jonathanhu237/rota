-- +goose Up
-- +goose StatementBegin
CREATE TABLE templates (
    id          BIGSERIAL PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    is_locked   BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE template_shifts (
    id                 BIGSERIAL PRIMARY KEY,
    template_id        BIGINT NOT NULL,
    weekday            INTEGER NOT NULL CHECK (weekday BETWEEN 1 AND 7),
    start_time         TIME NOT NULL,
    end_time           TIME NOT NULL CHECK (end_time > start_time),
    position_id        BIGINT NOT NULL,
    required_headcount INTEGER NOT NULL CHECK (required_headcount > 0),
    created_at         TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT template_shifts_template_id_fkey
        FOREIGN KEY (template_id) REFERENCES templates (id) ON DELETE CASCADE,
    CONSTRAINT template_shifts_position_id_fkey
        FOREIGN KEY (position_id) REFERENCES positions (id) ON DELETE RESTRICT
);

CREATE INDEX template_shifts_template_id_weekday_start_time_idx
    ON template_shifts (template_id, weekday, start_time);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX template_shifts_template_id_weekday_start_time_idx;
DROP TABLE template_shifts;
DROP TABLE templates;
-- +goose StatementEnd
