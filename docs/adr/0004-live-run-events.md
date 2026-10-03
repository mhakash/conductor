# 0004 — Live run events

Status: accepted (2026-10-04)

## Decision

Runs stream to the browser over **Server-Sent Events** at
`GET /api/runs/{id}/events`. Each message is one JSON `run.Event`.

- The first message is always a `snapshot`: the whole run plus its logs so
  far. The server subscribes to the run *before* reading the snapshot, so no
  change can fall between them.
- After that come deltas: `run` (run state), `step` (a step's complete
  state), `logs` (new lines with their `seq`), and finally `end`, after which
  the server closes the stream. A finished run gets `snapshot` + `end`.
- Deltas are complete values, so applying one twice is harmless; the client
  appends only log lines with a higher `seq` than it has.
- Reconnecting is just connecting again: a fresh snapshot resyncs the client.
  This is also how slow clients recover — the broker drops a subscriber whose
  buffer (512 events) fills rather than ever blocking the engine.
- The engine publishes only after it has written to the store, and flushes
  logs before the final `run` state, so a stream never ends with lines unsent.

## Why SSE

One-way server→client updates are all we need; SSE runs over plain HTTP,
reconnects automatically in the browser (`EventSource`), and needs no extra
library on either side. WebSockets would add a protocol for no benefit.

## Consequences

- Single-run views (run page, editor overlay) no longer poll. Run *lists*
  still poll every 2 s while any listed run is active; a global event stream
  can replace that if it ever matters.
- Open streams end when Conductor shuts down (`RegisterOnShutdown`), so they
  don't hold `http.Server.Shutdown` open.
