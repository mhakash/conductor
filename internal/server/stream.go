package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"conductor/internal/run"
)

const heartbeatEvery = 15 * time.Second

// handleRunEvents streams a run as Server-Sent Events: a snapshot of the run
// and its logs first, then every change until the run finishes. Each message
// is a JSON run.Event in the default "message" event.
//
// A client that reconnects simply gets a fresh snapshot; events are complete
// values (whole step states, logs with seq), so overlap is harmless.
func (s *Server) handleRunEvents(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Subscribe before reading the snapshot so nothing falls in between.
	events, unsubscribe := s.broker.Subscribe(id)
	defer unsubscribe()

	snapshot, err := s.runs.Get(r.Context(), id)
	if err != nil {
		s.runError(w, r, err)
		return
	}
	logs, err := s.runs.Logs(r.Context(), id, "", 0, 0)
	if err != nil {
		s.internalError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	rc := http.NewResponseController(w)

	send := func(e run.Event) bool {
		data, err := json.Marshal(e)
		if err != nil {
			s.log.Error("encode run event", "err", err)
			return false
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
			return false
		}
		return rc.Flush() == nil
	}

	if !send(run.Event{Type: run.EventSnapshot, Run: &snapshot, Lines: logs}) {
		return
	}
	if snapshot.Status.Done() {
		send(run.Event{Type: run.EventEnd})
		return
	}

	heartbeat := time.NewTicker(heartbeatEvery)
	defer heartbeat.Stop()
	for {
		select {
		case e, ok := <-events:
			// Closed without an end event means we fell behind; the client
			// reconnects and resyncs from a new snapshot.
			if !ok || !send(e) || e.Type == run.EventEnd {
				return
			}
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil || rc.Flush() != nil {
				return
			}
		case <-r.Context().Done():
			return
		case <-s.closing:
			return
		}
	}
}
