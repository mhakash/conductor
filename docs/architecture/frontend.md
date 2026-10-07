# Frontend map

React 19 + TS in `web/src`, Vite on :5180 proxying `/api` to :7070. Router:
`app/router.tsx`. Server state: TanStack Query hooks in each feature's
`api.ts`; HTTP client in `lib/api.ts`. Client state: Zustand (editor only).

## Routes

| Path             | Page                                    |
| ---------------- | --------------------------------------- |
| `/workflows`     | `features/workflows/WorkflowsPage.tsx`  |
| `/workflows/:id` | `features/editor/EditorPage.tsx` (lazy) |
| `/scratch`       | `features/editor/ScratchPage.tsx` (lazy) |
| `/runs`, `/runs/:id` | `features/runs/RunsPage.tsx`, `RunPage.tsx` |
| `/library`       | `features/library/LibraryPage.tsx`      |
| `/secrets`       | `features/secrets/SecretsPage.tsx`      |

## Features (`features/<area>`)

| Area        | What's there                                                                 |
| ----------- | ---------------------------------------------------------------------------- |
| `workflows` | list, `ImportDialog`, `FlowPreview`; `api.ts` holds workflow API types       |
| `editor`    | `store.ts` (Zustand), `Canvas`, `Inspector`, `StepSettings`, `Palette` + `library.ts`, `RunPanel` (test runs), `YamlDialog`, `scratch.ts` (localStorage draft), `graph.ts` (tested), `layout.ts` (ELK), `runOverlay.ts` |
| `editor/fields` | generated config form: `ConfigField` dispatches; `CodeField` (CodeMirror + `{{ }}`), `KeyValueField`, `WorkflowFields`, `expressions.ts` (completion scope) |
| `blocks`    | `/api/blocks` descriptors, `BlockIcon.tsx` (fixed icon map)                  |
| `runs`      | `RunPage`, `RunGraph`, `LogView`, `StartRunDialog`, `stream.ts` (`useRunStream` SSE → query cache), `order.ts`, `summary.ts`, `useRunWorkflow` |
| `presets`   | preset hooks, `PresetDialog`                                                 |
| `library`   | presets + block reference page                                               |
| `secrets`   | secrets page (values write-only)                                             |
| `engine`    | `useHealth`, `EngineStatus`                                                  |

## Shared

- `components/flow`: `StepCard` + `StepTraits`, the step visual shared by
  editor and run graph.
- `components/layout`: `AppShell`, `Sidebar`, `PageHeader`.
- `components/ui`: `Button`, `Input`, `Menu`, `Tooltip`, `ConfirmDialog` (Radix).
- `styles/index.css`: Tailwind v4 `@theme` tokens; React Flow dark overrides.
- `lib/`: `api.ts`, `cn.ts`, `time.ts`, `useNow.ts`.

## Tests

Vitest next to the code (`*.test.ts`): `pnpm test <file>`. Typecheck:
`pnpm typecheck`.
