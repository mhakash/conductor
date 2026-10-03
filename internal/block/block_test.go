package block

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type testEnv struct {
	stdout, stderr bytes.Buffer
	system         []string
}

func (e *testEnv) env() Env {
	return Env{
		Stdout: &e.stdout,
		Stderr: &e.stderr,
		Logf:   func(f string, a ...any) { e.system = append(e.system, fmt.Sprintf(f, a...)) },
	}
}

func cfg(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestShell(t *testing.T) {
	dir := t.TempDir()
	var e testEnv
	out, err := runShell(context.Background(), cfg(t, map[string]any{
		"command": `echo "hello $NAME"; pwd; echo oops >&2`,
		"workdir": dir,
		"shell":   "sh",
		"env":     map[string]string{"NAME": "conductor"},
	}), e.env())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out["stdout"].(string), "hello conductor") {
		t.Errorf("stdout = %q", out["stdout"])
	}
	if !strings.Contains(out["stdout"].(string), dir[len(dir)-10:]) {
		t.Errorf("command did not run in %s: %q", dir, out["stdout"])
	}
	if out["stderr"] != "oops" || out["exitCode"] != 0 {
		t.Errorf("outputs = %+v", out)
	}
	if !strings.Contains(e.stdout.String(), "hello conductor") {
		t.Errorf("stdout not streamed to the log: %q", e.stdout.String())
	}
}

func TestShellExitCode(t *testing.T) {
	var e testEnv
	out, err := runShell(context.Background(), cfg(t, map[string]any{"command": "exit 3", "shell": "sh"}), e.env())
	if err == nil || !strings.Contains(err.Error(), "code 3") {
		t.Fatalf("err = %v, want exit code 3", err)
	}
	if out["exitCode"] != 3 {
		t.Errorf("exitCode = %v", out["exitCode"])
	}
}

func TestShellCancelKillsProcessGroup(t *testing.T) {
	var e testEnv
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	// The child sleep would keep the pipes open if only the shell were killed.
	_, err := runShell(ctx, cfg(t, map[string]any{"command": "sleep 30 & wait", "shell": "sh"}), e.env())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want deadline exceeded", err)
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Errorf("cancel took %s; process group was not killed", took)
	}
}

func TestHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Token") != "t" || r.Header.Get("Content-Type") != "application/json" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"pipeline":{"status":"success"}}`)
	}))
	defer srv.Close()

	var e testEnv
	out, err := runHTTP(context.Background(), cfg(t, map[string]any{
		"method":  "POST",
		"url":     srv.URL,
		"headers": map[string]string{"X-Token": "t"},
		"body":    `{"ref":"v1"}`,
	}), e.env())
	if err != nil {
		t.Fatal(err)
	}
	if out["status"] != 200 {
		t.Errorf("status = %v", out["status"])
	}
	parsed, _ := out["json"].(map[string]any)
	if parsed["pipeline"].(map[string]any)["status"] != "success" {
		t.Errorf("json = %+v", out["json"])
	}
}

func TestHTTPNon2xxFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer srv.Close()

	var e testEnv
	out, err := runHTTP(context.Background(), cfg(t, map[string]any{"url": srv.URL}), e.env())
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("err = %v, want 404 failure", err)
	}
	if out["status"] != 404 {
		t.Errorf("status output = %v", out["status"])
	}
}

func TestMissing(t *testing.T) {
	missing, err := Shell.Missing(json.RawMessage(`{"command":"  "}`))
	if err != nil || len(missing) != 1 || missing[0] != "Command" {
		t.Errorf("Missing = %v, %v", missing, err)
	}
	missing, _ = HTTP.Missing(json.RawMessage(`{"url":"https://x"}`))
	if len(missing) != 0 {
		t.Errorf("Missing = %v, want none", missing)
	}
}
