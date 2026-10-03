# 0001 — Foundations

Status: accepted (2026-10-04)

## Context

Conductor is a personal automation tool: it runs git, CLI and OS tasks with the
owner's own credentials, on the owner's machine.

## Decisions

- **Local, single user.** No authentication. The server binds to `127.0.0.1` by
  default. Auth can be added if the tool ever leaves the laptop.
- **SQLite** via `modernc.org/sqlite` (pure Go, no CGO) with `goose` migrations.
  Storage sits behind repository interfaces so Postgres stays possible.
- **Steps run on the host** by default, so they see the same `git`, `ssh` and
  CLI configuration as the user. Docker isolation becomes an opt-in per step
  (phase 7).
- **One binary.** The Go server embeds the built frontend (`-tags embedui`).
  In development Vite serves the UI and proxies `/api` to Go.
- **Backend libraries:** `chi` (routing), `log/slog` (logging),
  `expr-lang/expr` (expressions, phase 4), SSE for run events (phase 3).
- **Frontend libraries:** Vite, React, TypeScript, Tailwind v4, `@xyflow/react`,
  Radix primitives, TanStack Query, Zustand, `motion`, `lucide-react`.

## Visual direction

Dark first. Cool graphite surfaces with a single brass accent that marks
activity (running steps, active route, engine heartbeat). Geist for UI text,
Geist Mono for logs and data, Instrument Serif italic only for the wordmark.
