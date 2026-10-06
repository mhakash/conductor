package block

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestScriptLanguages(t *testing.T) {
	scripts := map[string]string{
		"python": `import json, os
print("hello", os.environ["NAME"])
json.dump({"greeting": "hi " + os.environ["NAME"], "n": {"a": 1}}, open(os.environ["CONDUCTOR_OUTPUTS"], "w"))`,
		"node": `import { writeFileSync } from 'node:fs'
console.log('hello', process.env.NAME)
writeFileSync(process.env.CONDUCTOR_OUTPUTS, JSON.stringify({ greeting: 'hi ' + process.env.NAME, n: { a: 1 } }))`,
		"bash": `echo "hello $NAME"
printf '{"greeting": "hi %s", "n": {"a": 1}}' "$NAME" > "$CONDUCTOR_OUTPUTS"`,
		"go": `package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	fmt.Println("hello", os.Getenv("NAME"))
	out, _ := json.Marshal(map[string]any{"greeting": "hi " + os.Getenv("NAME"), "n": map[string]int{"a": 1}})
	os.WriteFile(os.Getenv("CONDUCTOR_OUTPUTS"), out, 0o644)
}`,
	}
	bins := map[string]string{"python": "python3", "node": "node", "bash": "bash", "go": "go"}
	for lang, code := range scripts {
		t.Run(lang, func(t *testing.T) {
			if _, err := exec.LookPath(bins[lang]); err != nil {
				t.Skipf("%s isn't installed", bins[lang])
			}
			var e testEnv
			out, err := runScript(context.Background(), cfg(t, map[string]any{
				"language": lang, "code": code, "env": map[string]string{"NAME": "Ada"},
			}), e.env())
			if err != nil {
				t.Fatalf("%v; stderr %q", err, e.stderr.String())
			}
			if out["stdout"] != "hello Ada" {
				t.Errorf("stdout = %q", out["stdout"])
			}
			result, _ := out["result"].(map[string]any)
			if result["greeting"] != "hi Ada" || result["n"].(map[string]any)["a"] != 1.0 {
				t.Errorf("result = %#v", out["result"])
			}
		})
	}
}

func TestScriptFailures(t *testing.T) {
	var e testEnv
	out, err := runScript(context.Background(), cfg(t, map[string]any{
		"language": "bash", "code": "echo partial; exit 3",
	}), e.env())
	if err == nil || err.Error() != "exited with code 3" || out["stdout"] != "partial" {
		t.Errorf("exit 3: %v, %v", err, out)
	}

	_, err = runScript(context.Background(), cfg(t, map[string]any{
		"language": "bash", "code": `echo nope > "$CONDUCTOR_OUTPUTS"`,
	}), e.env())
	if err == nil || !strings.Contains(err.Error(), "isn't JSON") {
		t.Errorf("bad outputs: %v", err)
	}

	_, err = runScript(context.Background(), cfg(t, map[string]any{"language": "cobol", "code": "x"}), e.env())
	if err == nil || !strings.Contains(err.Error(), `unknown language "cobol"`) {
		t.Errorf("unknown language: %v", err)
	}
}

func TestScriptCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	var e testEnv
	start := time.Now()
	_, err := runScript(ctx, cfg(t, map[string]any{"language": "bash", "code": "sleep 30"}), e.env())
	if err == nil || time.Since(start) > 5*time.Second {
		t.Errorf("err = %v after %s", err, time.Since(start))
	}
}

// Docker tests need a running Docker and pull nothing: they use an image
// that must already be present.
const dockerImage = "node:24-alpine"

func needDocker(t *testing.T) {
	t.Helper()
	if os.Getenv("CONDUCTOR_TEST_DOCKER") == "" {
		t.Skip("set CONDUCTOR_TEST_DOCKER=1 to run Docker tests")
	}
	if err := exec.Command("docker", "image", "inspect", dockerImage).Run(); err != nil {
		t.Skipf("docker or the %s image isn't available: %v", dockerImage, err)
	}
}

func TestShellInDocker(t *testing.T) {
	needDocker(t)
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/in.txt", []byte("from the host"), 0o644); err != nil {
		t.Fatal(err)
	}
	var e testEnv
	out, err := runShell(context.Background(), cfg(t, map[string]any{
		"command": `cat in.txt; echo " $TOKEN"; pwd; echo '{"ok": true}' > "$CONDUCTOR_OUTPUTS"`,
		"workdir": dir, "image": dockerImage,
		"env": map[string]string{"TOKEN": "fake-token-value"},
	}), e.env())
	if err != nil {
		t.Fatalf("%v; stderr %q", err, e.stderr.String())
	}
	if out["stdout"] != "from the host fake-token-value\n/work" {
		t.Errorf("stdout = %q", out["stdout"])
	}
	if out["result"].(map[string]any)["ok"] != true {
		t.Errorf("result = %v", out["result"])
	}
}

func TestScriptInDockerIsCancelled(t *testing.T) {
	needDocker(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var e testEnv
	_, err := runScript(ctx, cfg(t, map[string]any{
		"language": "node", "code": "await new Promise((r) => setTimeout(r, 60000))", "image": dockerImage,
	}), e.env())
	if err == nil {
		t.Fatal("script wasn't stopped")
	}
	left, _ := exec.Command("docker", "ps", "-aq", "--filter", "ancestor="+dockerImage, "--filter", "name=conductor-").Output()
	if len(strings.TrimSpace(string(left))) > 0 {
		t.Errorf("containers left behind: %s", left)
	}
}
