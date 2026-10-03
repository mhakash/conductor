package server

import (
	"bufio"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"conductor/internal/run"
	"conductor/internal/workflow"
)

// readEvents collects SSE data messages until the server closes the stream.
func readEvents(t *testing.T, url string) []run.Event {
	t.Helper()
	client := &http.Client{Timeout: 10 * time.Second}
	res, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if ct := res.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content-type = %q", ct)
	}

	var out []run.Event
	sc := bufio.NewScanner(res.Body)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		data, ok := strings.CutPrefix(sc.Text(), "data: ")
		if !ok {
			continue
		}
		var e run.Event
		if err := json.Unmarshal([]byte(data), &e); err != nil {
			t.Fatalf("bad event %q: %v", data, err)
		}
		out = append(out, e)
	}
	return out
}

func TestRunEventStream(t *testing.T) {
	srv := newTestServer(t)
	base := srv.URL + "/api"

	def := workflow.EmptyDefinition()
	def.Steps = []workflow.Step{{ID: "count", Block: "shell",
		Config: json.RawMessage(`{"command":"echo one; sleep 0.3; echo two","shell":"sh"}`)}}
	var wf workflow.Workflow
	do(t, "POST", base+"/workflows", map[string]any{"name": "Stream", "definition": def}, &wf)

	var started run.Run
	do(t, "POST", base+"/workflows/"+wf.ID+"/runs", nil, &started)

	evs := readEvents(t, base+"/runs/"+started.ID+"/events")
	if len(evs) < 3 || evs[0].Type != run.EventSnapshot || evs[0].Run == nil {
		t.Fatalf("first event = %+v", evs[0])
	}
	if evs[len(evs)-1].Type != run.EventEnd {
		t.Fatalf("stream did not end with an end event: %v", evs[len(evs)-1].Type)
	}

	// Snapshot plus deltas must describe the whole run.
	var texts []string
	status := evs[0].Run.Status
	for _, l := range evs[0].Lines {
		texts = append(texts, l.Text)
	}
	for _, e := range evs[1:] {
		for _, l := range e.Lines {
			texts = append(texts, l.Text)
		}
		if e.State != nil {
			status = e.State.Status
		}
	}
	if status != run.StatusSucceeded {
		t.Errorf("final status from stream = %s", status)
	}
	if joined := strings.Join(texts, "|"); !strings.Contains(joined, "one") || !strings.Contains(joined, "two") {
		t.Errorf("log lines from stream = %q", joined)
	}

	// A finished run streams its snapshot and ends immediately.
	again := readEvents(t, base+"/runs/"+started.ID+"/events")
	if len(again) != 2 || again[0].Run.Status != run.StatusSucceeded || again[1].Type != run.EventEnd {
		t.Errorf("replay of finished run = %+v", again)
	}
}

func TestRunEventStreamNotFound(t *testing.T) {
	srv := newTestServer(t)
	res, err := http.Get(srv.URL + "/api/runs/nope/events")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", res.StatusCode)
	}
}
