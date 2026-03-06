-- +goose Up
-- +goose StatementBegin
CREATE TABLE users (
    id       BIGSERIAL PRIMARY KEY,
    username TEXT UNIQUE NOT NULL,
    password TEXT NOT NULL,
    name     TEXT NOT NULL,
    is_admin BOOLEAN NOT NULL DEFAULT FALSE,
    version  INTEGER NOT NULL DEFAULT 1
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE users;
-- +goose StatementEnd
