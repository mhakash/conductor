# 0006 — Control flow

Status: accepted (2026-10-04)

Conditions, retries, continue-on-error and loops are **per-step settings**
next to the timeout, not special blocks or edge types. Edges keep one meaning
(“runs after”), so the graph reads the same everywhere. Approval is the
exception: it is a block, because it is a step of its own in the flow.

```jsonc
{
  "id": "update_jira",
  "block": "http",
  "if": "steps.pipeline.outputs.json.status == 'success'",
  "retry": { "max": 3, "delay": "10s", "backoff": "exponential" },
  "continueOnError": true,
  "forEach": { "items": "inputs.tickets", "concurrency": 4 },
}
```

`if` and `forEach.items` are bare expressions (no `{{ }}`), checked on save
with the same ancestor rules as config expressions (ADR 0005).

## When a step runs

Once everything a step depends on has finished:

| Dependencies                                 | The step                                   |
| -------------------------------------------- | ------------------------------------------ |
| any failed (without continue-on-error)       | is skipped, and so is everything below it  |
| any cancelled                                | is cancelled, and everything below it      |
| **all** skipped                              | is skipped (“because x was skipped”)       |
| otherwise                                    | evaluates its condition, then runs         |

The “all skipped” rule makes conditions work as branches without extra
syntax: a false condition skips its whole branch, while a step that joins
two branches (`if ok` / `if !ok`) still runs because one of them ran. This
is Airflow's `none_failed_min_one_success`; GitHub Actions' stricter
“any skipped skips” would make joins impossible without `always()`.

A condition must evaluate to `true` or `false`; anything else fails the step
rather than guessing at truthiness.

Expressions can read `steps.<id>.status` as well as outputs, so a step after
a continue-on-error step can react to its failure.

## Retries

A failed attempt (including a timeout; each attempt gets the full timeout)
is retried after `delay` (default 10s), doubling each time with
`exponential` backoff, capped at 1 hour. The slot is released while waiting.
Expression errors aren't retried — they would fail the same way. The step
records its attempt count.

## Continue-on-error

The step still shows as failed, but its dependents run and the run can
succeed. The run fails only for failures that weren't allowed.

## Loops

`forEach` runs the step once per element of a list; the config sees `item`
and `index`. Every item's config is rendered **before** any item runs, so a
typo fails the step without half the items done. Items run `concurrency` at a
time (default 1, in order), each taking an engine slot and retrying on its
own. The first failure stops items that haven't started. Outputs are
`{"results": [...]}`, one entry per item in order (`null` for items that
never ran). Log lines carry the item index. At most 1000 items.

## Approvals

The `approval` block renders its message and waits. The step and, once
approvals are all it has left, the run show as `waiting`. Approving
succeeds the step; rejecting fails it (so, by default, skips what follows).
Outputs: `approved`, `comment`. Without a timeout it waits indefinitely; a
timeout counts from when it started waiting.

**Waiting survives restarts.** On shutdown, a run with an approval waiting
is not cancelled: running steps are interrupted (recorded as cancelled, since
re-running something like `git push` blindly isn't safe), queued steps stay
queued, and the run stays `waiting`. On startup, `Engine.Recover` resumes
such runs from the store — finished steps are replayed in dependency order,
steps found `running` (after a crash) are marked cancelled — and fails
every other unfinished run as before.

A resumed run reads upstream outputs from the store, where secret values are
masked; a step after the restart that needs a secret should read it from
`secrets`, not from another step's output.
