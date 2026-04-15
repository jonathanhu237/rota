-- +goose Up
-- +goose StatementBegin
CREATE TABLE user_positions (
    user_id     BIGINT NOT NULL,
    position_id BIGINT NOT NULL,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, position_id),
    CONSTRAINT user_positions_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT user_positions_position_id_fkey
        FOREIGN KEY (position_id) REFERENCES positions (id) ON DELETE CASCADE
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE user_positions;
-- +goose StatementEnd
