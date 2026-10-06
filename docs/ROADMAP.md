# Roadmap

Conductor is built in phases. Each phase ends in something usable and builds on
the previous one. Scope inside a phase can shift; the order is what matters.

## Concepts

| Term         | Meaning                                                                                  |
| ------------ | ---------------------------------------------------------------------------------------- |
| Block        | A reusable node type with a typed config and typed outputs (`shell`, `http`, `git`, ...) |
| Step         | A configured instance of a block inside a workflow                                       |
| Workflow     | A directed acyclic graph of steps; edges are dependencies                                |
| Sub-workflow | A saved workflow used as a single step inside another workflow                           |
| Run          | One execution of a workflow, with per-step status, logs, outputs and timings             |
| Expression   | `{{ steps.tag.outputs.version }}` — references inputs, secrets and upstream outputs      |

Steps without a dependency path between them run in parallel.

## Phases

- [x] **0 — Foundations.** Repo layout, Go API server, React app shell, design
      tokens, dev/build tooling, single-binary build with the UI embedded.
- [x] **1 — Workflow editor.** SQLite storage, workflow CRUD, React Flow canvas,
      block palette, drag/connect, step config panel, save/load.
- [x] **2 — Engine v1.** DAG validation and scheduler, parallel execution,
      cancellation, timeouts. `shell` and `http` blocks.
- [x] **3 — Live runs.** Run events over SSE; the run page shows the workflow
      as a live graph (animated edges and step states) with logs per step;
      the editor shows each step's status from the latest run.
- [x] **4 — Data flow.** Step outputs, expressions, workflow inputs, encrypted
      secrets.
- [x] **5 — Control flow.** Conditions/branches, retries, continue-on-error,
      manual approval (survives restarts), loops over lists. See ADR 0006.
- [x] **6 — Reuse.** Sub-workflows as steps (child runs, workflow outputs),
      saved block presets, a library. See ADR 0007.
- [x] **7 — Code execution.** Script blocks (Python, Node, Bash, Go) with an
      in-app editor; optional per-step Docker isolation. See ADR 0008.
- [x] **8 — YAML.** View and edit workflows as YAML, export and import them.
      See ADR 0009.

## Ideas parking lot

Things that may become phases later: workflow versioning and diffing, remote
workers, auth for multi-user, plugin blocks loaded from disk, run replay from a
failed step.
