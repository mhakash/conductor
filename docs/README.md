# Docs index

| Need                                                | Read                                                     |
| --------------------------------------------------- | -------------------------------------------------------- |
| Where backend code lives, API routes                | [architecture/backend.md](architecture/backend.md)       |
| Where frontend code lives, routes, state            | [architecture/frontend.md](architecture/frontend.md)     |
| Add a block, API route, migration, run state, page  | [guides/recipes.md](guides/recipes.md)                   |
| Write or import a workflow as YAML                  | [reference/workflow-yaml.md](reference/workflow-yaml.md) |
| Why a design is the way it is                       | [adr/README.md](adr/README.md)                           |
| What's built, what's next                           | [ROADMAP.md](ROADMAP.md)                                 |

Conventions and gotchas for agents live in `.claude/rules/` (backend,
frontend, workflow format) and load only when matching files are touched.

## Writing docs

- One topic per file, under ~150 lines; link instead of repeating.
- Point at files (`internal/engine/step.go`), not line numbers.
- New decision → new ADR + a line in `adr/README.md`.
