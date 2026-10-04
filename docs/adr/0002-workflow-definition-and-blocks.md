# 0002 — Workflow definitions and block descriptors

Status: accepted (2026-10-04)

## Workflow definition is one JSON document

A workflow's graph (steps, edges, canvas positions) is stored as a single JSON
column with a `schemaVersion`, next to a few indexed columns (`id`, `name`,
timestamps).

- Saves are atomic: the editor sends the whole graph, the server validates it
  and replaces it. No partial-update bookkeeping.
- The same document is what YAML import/export (phase 9) will read and write.
- Runs (phase 2+) will snapshot the definition they executed, so editing a
  workflow never rewrites history.

Normalised tables for steps/edges were rejected: nothing queries individual
steps across workflows, and they would make saves multi-statement.

**Saves are conflict-checked.** The editor sends the `updatedAt` it loaded;
if the workflow was saved elsewhere since, the save fails with 409 and the
editor offers to load the latest version or overwrite it. (Added after a real
lost update: an editor tab opened earlier silently replaced a newer save.)

**Validation on save covers structure, not completeness** — names, step ids,
known blocks, edges, cycles, inputs and expression references (ADR 0005).
Missing required config is allowed so half-built drafts can be saved; it is
reported when the workflow is run.

**Edges mean dependency.** `{from: a, to: b}` means `b` starts after `a`
finishes. Steps with no path between them run in parallel.

## Blocks describe their own config

Each block declares a list of typed fields (`text`, `textarea`, `code`,
`number`, `boolean`, `select`, `keyvalue`) and the outputs it produces. The
frontend renders config forms from `GET /api/blocks`, so adding a block is a
backend-only change.

Full JSON Schema with a generic form renderer was considered and rejected for
now: it is heavier, harder to make look good, and our field set is small. If
blocks need nested or conditional config, revisit this.

Per-step execution settings (timeout, retries, continue-on-error) are not block
fields; they belong to every step and arrive with the engine (phases 2 and 5).
