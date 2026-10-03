-- +goose Up
CREATE TABLE runs (
    id            TEXT PRIMARY KEY,
    -- No foreign key: deleting a workflow keeps its run history.
    workflow_id   TEXT NOT NULL,
    workflow_name TEXT NOT NULL,
    definition    TEXT NOT NULL CHECK (json_valid(definition)),
    status        TEXT NOT NULL,
    error         TEXT NOT NULL DEFAULT '',
    created_at    TEXT NOT NULL,
    started_at    TEXT,
    finished_at   TEXT
);

CREATE INDEX runs_created_at ON runs (created_at DESC);
CREATE INDEX runs_workflow ON runs (workflow_id, created_at DESC);

CREATE TABLE step_runs (
    run_id      TEXT NOT NULL REFERENCES runs (id) ON DELETE CASCADE,
    step_id     TEXT NOT NULL,
    position    INTEGER NOT NULL,
    status      TEXT NOT NULL,
    error       TEXT NOT NULL DEFAULT '',
    outputs     TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(outputs)),
    started_at  TEXT,
    finished_at TEXT,
    PRIMARY KEY (run_id, step_id)
);

CREATE TABLE run_logs (
    seq     INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id  TEXT NOT NULL REFERENCES runs (id) ON DELETE CASCADE,
    step_id TEXT NOT NULL,
    stream  TEXT NOT NULL,
    text    TEXT NOT NULL,
    time    TEXT NOT NULL
);

CREATE INDEX run_logs_run ON run_logs (run_id, seq);

-- +goose Down
DROP TABLE run_logs;
DROP TABLE step_runs;
DROP TABLE runs;
