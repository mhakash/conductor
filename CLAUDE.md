# Conductor

Local, single-user workflow builder: workflows are DAGs of steps run on the
host, edited and watched on a canvas. Go API (`internal/`) + React UI (`web/`).

Vocabulary (code, UI, docs): **block** = node type, **step** = configured
block in a workflow, **workflow**, **run**, **expression**.

## Where to look

Start at [docs/README.md](docs/README.md); open only the doc you need.

- Code map: [backend](docs/architecture/backend.md), [frontend](docs/architecture/frontend.md)
- How-to (add a block, route, migration, page): [docs/guides/recipes.md](docs/guides/recipes.md)
- Workflow YAML format (read before writing a workflow file): [docs/reference/workflow-yaml.md](docs/reference/workflow-yaml.md)
- Why things are the way they are: [docs/adr/README.md](docs/adr/README.md)

Backend/frontend conventions and gotchas load automatically from
`.claude/rules/` when you touch matching files.

## Commands

```sh
make dev     # API :7070 (data in ./.data) + Vite :5180
make test    # go test ./... + tsc + vitest
make lint    # go vet + oxlint
make fmt     # gofmt + prettier
make build   # ./bin/conductor with UI embedded
```

Narrow first: `go test ./internal/<pkg>`, `cd web && pnpm test <file>`.

## Working style

- Record non-obvious choices (storage, libraries, execution model) as a new
  ADR.
- Prefer established libraries. No placeholder features or speculative
  abstractions.
- Changing blocks, fields, step settings, expressions or the YAML format:
  update `docs/reference/workflow-yaml.md` in the same change
  (`go test ./internal/wfyaml` enforces it).
- Commit messages describe the change; no "Phase N" prefixes.
- Keep replies short; don't paste plans, diffs or file contents the user can
  open in their editor. No screenshots or subagents unless needed or asked.
