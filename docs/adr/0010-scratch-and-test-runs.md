# 0010 — Scratch workflow and test runs

Status: accepted (2026-10-06)

Trying something shouldn't mean saving it, or leaving the editor to see what
happened.

## Runs of unsaved workflows

`POST /api/runs` takes a workflow definition (and inputs) and runs it without
saving anything. It is validated like a save. With `workflowId`, the run
belongs to that saved workflow — a **test run** of the editor's draft, listed
with its other runs — but the saved workflow is unchanged. Without one it is
a **scratch run**, with an empty workflow id. Either way the run snapshots
what it ran, as every run does (ADR 0002), so history stays accurate.

## Scratch workflow

`/scratch` is the editor on a workflow that is never saved to Conductor. Its
draft and its latest run are kept in the browser's local storage, so leaving
the page loses nothing; **Clear** starts again and **Save as workflow**
creates a real one from it. One scratch per browser: it is a workbench, not
a second list of workflows.

## Running in place

**Test run** in the editor, and **Run** on the scratch page, start the run
and stay: steps show their status on the canvas and a panel under it shows
the run's status and the selected step's logs. **Run** on a saved workflow
still saves first and opens the run page.
