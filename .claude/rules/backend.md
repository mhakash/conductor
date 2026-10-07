---
paths:
  - "internal/**"
  - "cmd/**"
  - "web/*.go"
---

# Backend rules

Map: `docs/architecture/backend.md`. Recipes: `docs/guides/recipes.md`.

- Standard library first; `chi` routing, `log/slog` logging.
- API under `/api`, JSON only. Errors `{"error": "..."}` via `writeError`;
  unknown `/api/*` is a JSON 404, never the SPA. Validation failures: 422 with
  `problems: [{message, step?, edge?}]`.
- Workflow graph is one versioned JSON doc (ADR 0002). Migrations:
  `internal/store/migrations/NNNNN_name.sql`.
- Blocks: `Run` honours ctx; commands go through `runProcess` (process.go).
  `Literal` fields (script code) are never expressions — pass
  `blocks.Literal(type)` to `tmpl.RenderConfig`/`ConfigRefs`;
  `workflow.Validate` takes the registry.
- Per-step settings (`if`, `retry`, `continueOnError`, `forEach`, `timeout`)
  live on `workflow.Step`, not block config. `approval` and `workflow` blocks
  have no `RunFunc`; the engine runs them (ADR 0006, 0007).
- Saves also run `workflow.ValidateCalls` (needs other workflows). Run lists
  show top-level runs unless filtered by workflow.
- Anything a step reads: checked at save (`workflow.Validate`), resolved in
  the engine just before the step runs (ADR 0005).
- Anything stored or published about a run goes through its secret masker.
- New run state must be stored **and** published as a `run.Event` (ADR 0004),
  and rebuildable from the store: approvals survive restarts via
  `Engine.Recover`.
- Engine semantics changes → update ADR 0003/0006.
- Engine and block tests use `-race`; keep them race-clean. Docker tests need
  `CONDUCTOR_TEST_DOCKER=1` + `node:24-alpine`.
- gitleaks scans fixtures: use obviously fake secrets.
- `go build` without `-tags embedui` serves API only (expected in dev).
- Server binds `127.0.0.1` by default.
