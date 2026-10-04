-- +goose Up
-- Progress of a step that retries or runs once per item.
ALTER TABLE step_runs ADD COLUMN attempt INTEGER NOT NULL DEFAULT 0;
ALTER TABLE step_runs ADD COLUMN items INTEGER NOT NULL DEFAULT 0;
ALTER TABLE step_runs ADD COLUMN items_done INTEGER NOT NULL DEFAULT 0;
-- The loop index of a line from a step that runs once per item.
ALTER TABLE run_logs ADD COLUMN item INTEGER;

-- Finding runs to resume or fail at startup, and waiting runs for the sidebar.
CREATE INDEX runs_status ON runs (status);

-- +goose Down
DROP INDEX runs_status;
ALTER TABLE run_logs DROP COLUMN item;
ALTER TABLE step_runs DROP COLUMN items_done;
ALTER TABLE step_runs DROP COLUMN items;
ALTER TABLE step_runs DROP COLUMN attempt;
