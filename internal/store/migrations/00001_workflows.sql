-- +goose Up
CREATE TABLE workflows (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    definition  TEXT NOT NULL CHECK (json_valid(definition)),
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE INDEX workflows_updated_at ON workflows (updated_at DESC);

-- +goose Down
DROP TABLE workflows;
