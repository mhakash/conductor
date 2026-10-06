-- +goose Up
-- A run started by a step of another run (a sub-workflow).
ALTER TABLE runs ADD COLUMN parent_run_id TEXT;
ALTER TABLE runs ADD COLUMN parent_step_id TEXT;
ALTER TABLE runs ADD COLUMN parent_item INTEGER;
-- The workflow's outputs, set when the run succeeds.
ALTER TABLE runs ADD COLUMN outputs TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(outputs));

CREATE INDEX runs_parent ON runs (parent_run_id, created_at);

-- A saved step configuration offered in the editor's library.
CREATE TABLE presets (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE COLLATE NOCASE,
    description TEXT NOT NULL DEFAULT '',
    block       TEXT NOT NULL,
    -- Config and per-step settings (timeout, retry, continueOnError).
    step        TEXT NOT NULL CHECK (json_valid(step)),
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

-- +goose Down
DROP TABLE presets;
DROP INDEX runs_parent;
ALTER TABLE runs DROP COLUMN outputs;
ALTER TABLE runs DROP COLUMN parent_item;
ALTER TABLE runs DROP COLUMN parent_step_id;
ALTER TABLE runs DROP COLUMN parent_run_id;
