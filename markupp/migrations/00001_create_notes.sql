-- +goose Up
CREATE TABLE notes (
    id          TEXT PRIMARY KEY,
    path        TEXT NOT NULL UNIQUE,
    content     TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS notes;
