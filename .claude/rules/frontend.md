---
paths:
  - "web/**"
---

# Frontend rules

Map: `docs/architecture/frontend.md`.

- React 19 + TS, Tailwind v4, React Router v8, TanStack Query (server state),
  Zustand (client state), `motion`, `lucide-react`, `@xyflow/react`,
  CodeMirror. Heavy pages/libs load lazily (editor route, elkjs).
- Import via `@/`. Prettier: no semicolons, single quotes, width 100.
- Colours only from tokens in `styles/index.css` (`canvas`, `surface`,
  `raised`, `hover`, `line`, `fg*`, `brass`, `sage`, `coral`, `steel`). No raw
  hex. Brass = activity; sage/coral/steel = run status only.
- Fonts: Geist (UI), Geist Mono (data, logs, block names), Instrument Serif
  italic for the wordmark only.
- Reduced motion (`MotionConfig reducedMotion="user"`, `useReducedMotion`,
  `motion-reduce:`); keep visible focus rings.
- Copy: sentence case, plain verbs, buttons name the action.

## Gotchas

- pnpm is pinned (`packageManager`); a global pnpm 11 purges `node_modules`
  in non-TTY shells without it. Vite is on 5180 (`strictPort`).
- Zustand v5: selectors returning new arrays/objects need `useShallow`.
- Radix `asChild` + NavLink: pass a string className; style active links
  with `aria-[current=page]:`.
- React Flow: styles imported before ours, overrides on `.react-flow.dark`;
  edges need `<Handle>`s (invisible on read-only nodes); in a hidden browser
  pane nodes can't be measured, so verify canvases with the pane visible.
- Block icons are a fixed map in `BlockIcon.tsx`.
- `workflow` block form uses special field types (`workflow`, `inputs`); its
  outputs come from `useWorkflowMap`, not the descriptor.
- Config text/code fields are CodeMirror (`fields/CodeField.tsx`); the
  completion scope is fed via the `setExpressionScope` effect, not render.
- A run is kept fresh by `useRunStream` (SSE → query cache), not polling;
  it reconnects itself after HTTP errors. Run lists poll.
- `sr-only` inputs are absolute: give the wrapper `relative`.
- Test/scratch runs post the draft to `POST /api/runs` (ADR 0010); scratch
  draft is in localStorage `conductor.scratch`.
- Unsaved-changes blocker reads `useEditor.getState().dirty` at navigation
  time, not a value captured at render.
