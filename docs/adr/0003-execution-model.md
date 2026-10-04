# 0003 — Execution model

Status: accepted (2026-10-04)

## Scheduling

A run executes its steps as a DAG in-process, in one goroutine per run:

- A step starts when **all** of its dependencies have **finished** without
  failing (see ADR 0006 for skipped dependencies and continue-on-error).
- Ready steps run concurrently, capped per run by `--max-parallel`
  (`CONDUCTOR_MAX_PARALLEL`, default 8).
- When a step fails, every step downstream of it is **skipped**; branches that
  don't depend on it keep going. The run ends `failed`.
- Cancelling a run interrupts running steps and marks waiting ones
  `cancelled`. Stopping Conductor cancels active runs the same way, with the
  reason recorded.
- Each step has a timeout (default 1 hour, overridable per step as a Go
  duration such as `30s` or `10m`).
- Runs left `queued`/`running` by a crash are marked `failed` on startup,
  except runs waiting for approval, which resume (ADR 0006).

Conditions, retries, continue-on-error, loops and approvals are in ADR 0006.

## Runs are snapshots

A run stores the workflow definition it executed. Editing or deleting a
workflow never changes its history.

## Blocks execute through one interface

```go
type RunFunc func(ctx context.Context, config json.RawMessage, env Env) (map[string]any, error)
```

`env` gives a step line-buffered stdout/stderr writers and a `Logf` for
Conductor's own messages. A block returns its outputs, partly filled even on
failure (a failing command still has an exit code). A panic in a block fails
only that step.

## Shell steps

- Run with the user's **login shell** (`$SHELL -l -c`) by default, so `PATH`
  and tool setup match the terminal. bash/zsh/sh can be chosen per step, also
  as login shells. Aliases from `.zshrc` aren't available, since the shell is
  not interactive.
- The command gets its own process group; cancel and timeout send `SIGTERM`
  to the whole group, then kill after 3 seconds.
- Working directory defaults to `$HOME`; `~/` is expanded.

## Logs

Lines from all steps go through a per-run sink that writes to SQLite in
batches (every 200 ms or 500 lines). Each line has a run-wide `seq`, which is
the cursor for incremental reads — and later for live streaming (phase 3).
Captured `stdout`/`stderr` outputs are capped at 1 MiB; the log keeps
everything.
