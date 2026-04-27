-- +goose Up
-- +goose StatementBegin

ALTER TABLE users
    ADD COLUMN language_preference TEXT NULL
        CHECK (language_preference IS NULL OR language_preference IN ('zh', 'en')),
    ADD COLUMN theme_preference TEXT NULL
        CHECK (theme_preference IS NULL OR theme_preference IN ('light', 'dark', 'system'));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE users
    DROP COLUMN theme_preference,
    DROP COLUMN language_preference;

-- +goose StatementEnd
