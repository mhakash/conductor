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
make dev        # Go API :7070 + Vite :5180 (proxies /api)
make test       # go test ./... + tsc
make lint       # go vet + oxlint
make fmt        # gofmt + prettier
make build      # ./bin/conductor with UI embedded
```

Frontend-only: `cd web && pnpm <dev|build|typecheck|lint|format>`.

## Layout

```
cmd/conductor      entrypoint: config, logger, HTTP server, graceful shutdown
internal/config    flags + CONDUCTOR_* env vars
internal/server    chi routes, middleware, SPA handler; tests use httptest
internal/version   ldflags-injected build info
web/embed*.go      `embedui` build tag embeds web/dist; without it UI() is nil
web/src/
  app/             router
  components/ui    small primitives (Button, Tooltip on Radix)
  components/layout app shell, sidebar, page header
  features/<area>  feature folders: pages, hooks, components for one area
  lib/             api client, cn()
  styles/index.css Tailwind v4 @theme design tokens
```

## Backend conventions

- Go module path is `conductor`. Standard library first; `chi` for routing,
  `log/slog` for logging.
- API lives under `/api`, returns JSON; errors are `{"error": "..."}` via
  `writeError`. Unknown `/api/*` routes are JSON 404s, never the SPA.
- Server binds to `127.0.0.1` by default. Steps run on the host (Docker opt-in
  later).

## Frontend conventions

- React 19 + TS, Tailwind v4, React Router v7, TanStack Query for server state,
  Zustand for client state, `motion` for animation, `lucide-react` icons.
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
