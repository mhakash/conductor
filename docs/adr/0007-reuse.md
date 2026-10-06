# 0007 — Reuse: sub-workflows, presets and the library

Status: accepted (2026-10-06)

## Sub-workflows run as child runs

A step with the `workflow` block runs another saved workflow:

```jsonc
{
  "id": "deploy",
  "block": "workflow",
  "config": { "workflow": "<id>", "inputs": { "service": "{{ item }}" } },
  "forEach": { "items": "inputs.services", "concurrency": 2 },
}
```

Each attempt starts a **child run**: an ordinary run with its own page,
logs and snapshot of the definition, linked to the step that started it
(`parent_run_id`, `parent_step_id`, `parent_item`). Inlining the called
workflow's steps into the parent graph was rejected: step ids would clash,
the parent's page would grow without bound, and a child run is something
you want to open on its own anyway.

- The step is an ordinary step to the scheduler, so `if`, `retry`,
  `timeout`, `continueOnError` and `forEach` all apply: one child run per
  attempt, per item.
- The workflow is loaded when the step starts; editing it later doesn't
  change runs already started.
- Inputs are checked like a run started by hand (defaults, types, required).
- **Outputs** are declared by the called workflow (below). The step's outputs
  are the child run's recorded outputs, so secrets in them stay masked.
- Cancelling the parent, or the step timing out, cancels the child. A child
  cancelled on its own cancels the step (not retried).
- A child that fails fails the step with the child's reason:
  `Release failed at step "push": exit status 1`.
- Run lists show top-level runs only; a run's children appear on its page and
  stream to it live (`child` events).

**Waiting carries upwards.** A step whose child runs are all waiting for
approval shows as waiting, and so does its run when that is all it has left.
On shutdown the step is suspended with its child, and both are kept by
`FailUnfinished` (a run is kept if a run below it has a step waiting). On
startup `Recover` resumes them together; the resumed step picks up the child
runs it had started — waiting for those still going, using the result of
those that finished — rather than starting them again. A child whose parent
step is no longer running is failed.

**Secrets passed down** as inputs are stored masked in the child's inputs and
masked in everything the child records. After a restart the child only has
the masked value, as with outputs read from the store (ADR 0006).

**Loops between workflows** are refused when saving (A runs B runs A, using
the definition being saved) and checked again when a run starts; at run time
a workflow already running further up can't start, and nesting stops at 8
levels. Deleting a workflow that another workflow runs is refused (409).

## Workflow outputs

A workflow declares outputs as bare expressions, evaluated once every step
has finished, so they keep their type (a list stays a list):

```json
"outputs": [{ "name": "url", "value": "steps.deploy.outputs.stdout" }]
```

They may read any step and the inputs; they are checked on save. An output
that can't be evaluated fails the run — the same "no silent empty string"
rule as config expressions. Outputs are recorded on the run and shown on its
page, whether or not anything called it.

## Presets are copies

A preset is a saved block with its config, timeout, retries and
continue-on-error. Conditions and loops aren't saved: they refer to steps of
one particular workflow. Adding a preset **copies** it into the workflow;
editing or deleting the preset later doesn't touch existing steps. Linked
presets would make a change in one place silently alter every workflow using
it, and would need versioning to keep runs reproducible — a sub-workflow is
the tool for shared behaviour that should change everywhere at once.

Preset names are unique (ignoring case). Saving a step under a taken name
replaces that preset, which is how a preset is updated.

## The library

The editor's left panel is the library: blocks, presets and the other
workflows, searchable, each added by drag or click. The Library page manages
presets and documents every block's fields and outputs. Workflows stay on
the Workflows page.
