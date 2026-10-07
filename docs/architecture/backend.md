# Backend map

Go module `conductor`. Entry: `cmd/conductor/main.go` (config, slog, HTTP
server, graceful shutdown, `Engine.Recover` at startup).

## Packages

| Package              | Owns                                                        | Key files                                                 |
| -------------------- | ----------------------------------------------------------- | --------------------------------------------------------- |
| `internal/config`    | flags + `CONDUCTOR_*` env vars                              | `config.go`                                               |
| `internal/workflow`  | domain types, Repository iface, validation, workflow inputs | `workflow.go`, `validate.go`, `inputs.go`, `calls.go`     |
| `internal/block`     | block descriptors (fields, outputs) + executors             | `builtin.go` (registry), `shell.go`, `script.go`, `http.go`, `process.go` |
| `internal/preset`    | saved step configs + validation                             | `preset.go`                                               |
| `internal/run`       | run/step/log types, run Repository iface, `run.Event`       | `run.go`                                                  |
| `internal/engine`    | DAG scheduler, attempts/loops/approvals, child runs, logs   | `execute.go`, `step.go`, `subrun.go`, `logs.go`, `engine.go` |
| `internal/events`    | in-process broker → SSE subscribers                         | `broker.go`                                               |
| `internal/tmpl`      | `{{ expr }}` compile/render, reference extraction           | `tmpl.go`, `config.go`                                    |
| `internal/secrets`   | AES-GCM cipher, keychain key, store, output masker          | `cipher.go`, `key.go`, `store.go`, `mask.go`              |
| `internal/store`     | SQLite (modernc) repos + goose migrations                   | `store.go`, `workflows.go`, `runs.go`, `presets.go`, `migrations/` |
| `internal/wfyaml`    | YAML ↔ definition, `needs` ↔ edges, layout                  | `wfyaml.go`, `doc_test.go` (checks the YAML doc)          |
| `internal/server`    | chi routes, middleware, SPA handler                         | `server.go` (routes), one file per resource               |
| `internal/version`   | ldflags build info                                          | `version.go`                                              |
| `web` (Go)           | `embedui` tag embeds `web/dist`; else `UI()` is nil         | `embed.go`, `embed_dev.go`                                |

## Request → run flow

`server/runs.go` validates (`workflow.Validate` + `ValidateCalls`) →
`engine` creates the run in `store` → `execute.go` schedules ready steps →
`step.go` renders config (`tmpl`), resolves secrets, calls the block's
`RunFunc` (commands via `block/process.go`) → each store write publishes a
`run.Event` via `events` → `server/stream.go` sends SSE.

## API (`internal/server/server.go`)

| Route                                           | Handler file   |
| ----------------------------------------------- | -------------- |
| `GET /api/health`, `GET /api/blocks`            | `server.go`    |
| `/api/workflows` CRUD, `/{id}/runs`             | `workflows.go`, `runs.go` |
| `POST /api/workflows/import`, `/to-yaml`, `/from-yaml`, `GET /{id}/export` | `yaml.go` |
| `/api/presets` CRUD                             | `presets.go`   |
| `/api/secrets` list/put/delete (write-only values) | `secrets.go` |
| `/api/runs` list, `POST` (draft run), `/{id}`, `/logs`, `/events` (SSE), `/cancel`, `/steps/{step}/decision` | `runs.go`, `stream.go` |

## Tests

- Server tests hit a real temp SQLite DB (`server_test.go` helpers).
- Engine and block tests: `go test -race ./internal/engine ./internal/block`.
- Docker tests need `CONDUCTOR_TEST_DOCKER=1` and `node:24-alpine` pulled.
