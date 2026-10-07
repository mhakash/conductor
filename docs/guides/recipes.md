# Recipes

Common changes and every place they touch. Conventions: `.claude/rules/`.

## Add a block

1. `internal/block/<name>.go`: a `block.Block` var (type, fields, outputs,
   `Run`). `Run` honours ctx; commands go through `runProcess` (`process.go`).
   Mark script-code fields `Literal`.
2. Register it in `Builtin()` (`internal/block/builtin.go`).
3. Icon: add the lucide icon to `web/src/features/blocks/BlockIcon.tsx`.
   The config form is generated from the fields — no other UI change.
4. Document it under "Blocks" in `docs/reference/workflow-yaml.md`;
   `go test ./internal/wfyaml` fails until every field is listed.
5. Tests in `internal/block/*_test.go`, run with `-race`.

## Add a per-step setting (like `retry`, `if`)

`workflow.Step` (`internal/workflow/workflow.go`) → check in `validate.go`
→ apply in `internal/engine/step.go` → YAML mapping in `internal/wfyaml` →
UI in `web/src/features/editor/StepSettings.tsx` + `components/flow/StepTraits.tsx`
→ doc + ADR 0006.

## Add an API route

Route in `internal/server/server.go`, handler in the resource's file. JSON
only; errors via `writeError`; validation failures 422 with
`problems: [{message, step?, edge?}]`. Test against a real temp DB
(see `server_test.go`). Frontend hook goes in the feature's `api.ts`.

## Add a migration

`internal/store/migrations/NNNNN_name.sql` (next number, goose up/down).
Repo methods in `internal/store/*.go`.

## Add run state

Store it (`internal/run` types + `internal/store/runs.go`), publish a
`run.Event` after the write, mask it through the run's secret masker, and
make it rebuildable on restart (`Engine.Recover`). UI picks it up through
`features/runs/stream.ts`.

## Add an expression variable

Scope in `internal/tmpl`; check references at save time
(`workflow.Validate`); resolve in the engine just before the step runs;
completion scope in `web/src/features/editor/fields/expressions.ts`; doc
"Expressions" section + ADR 0005.

## Add a page

Feature folder `web/src/features/<area>/` (page, `api.ts`), route in
`app/router.tsx` (lazy if heavy), nav link in `components/layout/Sidebar.tsx`.
