# Conductor

Personal workflow builder and task runner: workflows are DAGs of steps that run
on the host with dependencies and parallelism, edited and watched on a visual
canvas. Local, single user, no auth.

- Plan and progress: `docs/ROADMAP.md`. Decisions: `docs/adr/`.
- Vocabulary (use it in code, UI and docs): **block** = node type, **step** =
  configured block in a workflow, **workflow**, **run**, **expression**. See the
  table in the roadmap.

## Working style

- Build one roadmap phase at a time. Each phase ends in something visible and
  usable; stop for review before starting the next.
- When a phase lands, tick it in `docs/ROADMAP.md`. Record non-obvious choices
  (storage format, libraries, execution model) as a new ADR.
- Prefer established libraries over hand-rolled code. No placeholder features,
  filler copy or speculative abstractions.

## Commands

```sh
make dev        # Go API :7070 (data in ./.data) + Vite :5180 (proxies /api)
make test       # go test ./... + tsc + vitest
make lint       # go vet + oxlint
make fmt        # gofmt + prettier
make build      # ./bin/conductor with UI embedded
```

Frontend-only: `cd web && pnpm <dev|build|typecheck|test|lint|format>`.

## Layout

```
cmd/conductor      entrypoint: config, logger, HTTP server, graceful shutdown
internal/config    flags + CONDUCTOR_* env vars
internal/workflow  domain types, Repository interface, structural validation
internal/block     block descriptors (fields, outputs) + executors (RunFunc); builtin registry
internal/run       run/step/log types and the run Repository interface
internal/engine    DAG scheduler: parallel steps, cancel, timeouts, batched logs
internal/events    in-process broker fanning run events out to SSE streams
internal/store     SQLite (modernc) + goose migrations in store/migrations
internal/server    chi routes, middleware, SPA handler; tests hit a real temp DB
internal/version   ldflags-injected build info
web/embed*.go      `embedui` build tag embeds web/dist; without it UI() is nil
web/src/
  app/             router
  components/ui    small primitives (Button, Tooltip on Radix)
  components/layout app shell, sidebar, page header
  components/flow  StepCard: the step visual shared by editor and run graph
  features/<area>  feature folders: pages, hooks, components for one area
    workflows/     list page, query hooks + API types (api.ts)
    blocks/        block descriptors from /api/blocks, icons
    editor/        canvas editor: Zustand store, graph helpers (tested), ELK
                   layout, palette, inspector, generated config fields
    runs/          runs list, run page (live graph, steps, logs, outputs),
                   SSE stream hook (stream.ts), status icons
  lib/             api client, cn()
  styles/index.css Tailwind v4 @theme design tokens
```

## Backend conventions

- Go module path is `conductor`. Standard library first; `chi` for routing,
  `log/slog` for logging.
- API lives under `/api`, returns JSON; errors are `{"error": "..."}` via
  `writeError`. Unknown `/api/*` routes are JSON 404s, never the SPA.
  Validation failures are 422 with `problems: [{message, step?, edge?}]`.
- A workflow's graph is one versioned JSON document (see ADR 0002). New
  migrations go in `internal/store/migrations` as `NNNNN_name.sql`.
- Adding a block = a `block.Block` (fields, outputs, `Run`) in
  `internal/block`; the UI renders its config form from the field list, no
  frontend change needed. `Run` must honour ctx cancellation.
- Execution semantics (skip downstream on failure, timeouts, login shell,
  process groups) are in ADR 0003; keep them in sync when changing the engine.
- Engine and block tests run with `go test -race`; keep them race-clean.
- Live updates: the engine publishes a `run.Event` after every store write;
  `/api/runs/{id}/events` streams snapshot + deltas (ADR 0004). New run state
  must be both stored and published.
- Server binds to `127.0.0.1` by default. Steps run on the host (Docker opt-in
  later).

## Frontend conventions

- React 19 + TS, Tailwind v4, React Router v8, TanStack Query for server state,
  Zustand for client state, `motion` for animation, `lucide-react` icons,
  React Flow (`@xyflow/react`) for the canvas, CodeMirror for code fields.
- Heavy pages and libraries load lazily (editor route, elkjs).
- Import via the `@/` alias. Prettier: no semicolons, single quotes, width 100,
  Tailwind class sorting.
- Colours come only from tokens in `styles/index.css` (`canvas`, `surface`,
  `raised`, `hover`, `line`, `fg*`, `brass`, `sage`, `coral`, `steel`). No raw
  hex in components. Brass marks activity; sage/coral/steel are reserved for
  run status.
- Fonts: Geist (UI), Geist Mono (data, logs, block names), Instrument Serif
  italic for the wordmark only.
- Respect reduced motion (`MotionConfig reducedMotion="user"`,
  `useReducedMotion`, `motion-reduce:`). Keep visible focus rings.
- UI copy: sentence case, plain verbs, buttons name the action.

## Gotchas

- **pnpm version is pinned** (`packageManager` in `web/package.json`). The
  machine has a global pnpm 11 that, without the pin, tries to purge
  `node_modules` in non-TTY shells (e.g. under `make`).
- **Vite runs on 5180** (`strictPort`); 5173 is used by another project.
- **Radix `asChild` + NavLink:** Slot can't merge function `className`s. Pass a
  string and style active links with `aria-[current=page]:` variants.
- Prettier's Tailwind plugin needs `tailwindStylesheet` to know custom tokens;
  it's set in `.prettierrc.json`.
- `go build` without `-tags embedui` serves API only — that's expected in dev.
- Zustand v5: a selector that builds a new array/object (e.g. `.filter`) must
  be wrapped in `useShallow`, or the component re-renders forever.
- React Flow's built-in styles are imported before ours; theme overrides go
  on `.react-flow.dark` in `styles/index.css`.
- Block icons are a fixed map in `BlockIcon.tsx`; add new lucide icons there.
- A single run is kept fresh by `useRunStream` (SSE) writing into the query
  cache, not by polling. Run lists still poll, and TanStack pauses that in
  hidden tabs.
- React Flow only draws edges between nodes that render `<Handle>`s; read-only
  nodes still need (invisible) handles.
- In a hidden browser pane React Flow can't measure nodes, so they stay
  `visibility: hidden` and edges don't render. Verify canvases with the pane
  visible.
- The editor's unsaved-changes blocker reads `useEditor.getState().dirty` at
  navigation time; a boolean captured at render would block Run's
  save-then-navigate.
