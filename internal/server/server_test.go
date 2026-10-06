package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"testing/fstest"

	"conductor/internal/block"
	"conductor/internal/engine"
	"conductor/internal/events"
	"conductor/internal/secrets"
	"conductor/internal/store"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	ui := fstest.MapFS{
		"index.html":    {Data: []byte("<html>app</html>")},
		"assets/app.js": {Data: []byte("console.log(1)")},
	}
	log := slog.New(slog.DiscardHandler)
	db, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"), log)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	runs := store.NewRuns(db)
	blocks := block.Builtin()
	broker := events.NewBroker()
	cipher, err := secrets.NewCipher(make([]byte, secrets.KeySize))
	if err != nil {
		t.Fatal(err)
	}
	secretStore := secrets.NewStore(db, cipher)
	workflows := store.NewWorkflows(db)
	eng := engine.New(engine.Options{Runs: runs, Workflows: workflows.Get, Blocks: blocks, Events: broker, Secrets: secretStore, Log: log})
	t.Cleanup(func() { _ = eng.Shutdown(context.Background()) })

	srv := httptest.NewServer(New(Deps{
		Log:       log,
		Workflows: workflows,
		Presets:   store.NewPresets(db),
		Runs:      runs,
		Engine:    eng,
		Broker:    broker,
		Secrets:   secretStore,
		Blocks:    blocks,
		UI:        ui,
	}).Handler())
	t.Cleanup(srv.Close)
	return srv
}

func TestHealth(t *testing.T) {
	srv := newTestServer(t)

	res, err := http.Get(srv.URL + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	var body healthResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "ok" {
		t.Errorf("status = %q, want ok", body.Status)
	}
}

func TestUnknownAPIRouteIsJSON404(t *testing.T) {
	srv := newTestServer(t)

	res, err := http.Get(srv.URL + "/api/nope")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %q, want application/json", ct)
	}
}

func TestSPAFallback(t *testing.T) {
	srv := newTestServer(t)

	for _, path := range []string{"/", "/workflows/abc"} {
		res, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if string(body) != "<html>app</html>" {
			t.Errorf("GET %s served %q, want index.html", path, body)
		}
	}
}
