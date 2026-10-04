-- +goose Up
CREATE TABLE secrets (
    name       TEXT PRIMARY KEY,
    -- AES-256-GCM: 12-byte nonce followed by ciphertext; the name is the AAD.
    ciphertext BLOB NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

ALTER TABLE runs ADD COLUMN inputs TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(inputs));
-- The step's config after expressions were evaluated, with secrets masked.
ALTER TABLE step_runs ADD COLUMN config TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(config));

-- +goose Down
ALTER TABLE step_runs DROP COLUMN config;
ALTER TABLE runs DROP COLUMN inputs;
DROP TABLE secrets;
