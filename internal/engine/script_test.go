package engine

import (
	"context"
	"strings"
	"testing"

	"conductor/internal/run"
	"conductor/internal/workflow"
)

// Script code is literal: braces in it are code, not expressions, while the
// step's environment variables still evaluate theirs.
func TestScriptCodeIsLiteral(t *testing.T) {
	f := newFixture(t, 8)
	wf := build([]stepSpec{
		{id: "tag", block: "emit", config: `{"value":"v1.4.0"}`},
		{id: "script", block: "script", config: `{
			"language": "bash",
			"code": "echo \"{{ not an expression }} $VERSION $TOKEN\"\nprintf '{\"v\": \"%s\"}' \"$VERSION\" > \"$CONDUCTOR_OUTPUTS\"",
			"env": {"VERSION": "{{ steps.tag.outputs.value }}", "TOKEN": "{{ secrets.TOKEN }}"}
		}`},
		{id: "after", block: "emit", config: `{"value":"{{ steps.script.outputs.result.v }}"}`},
	}, "tag>script", "script>after")
	if err := f.engine.Check(context.Background(), wf); err != nil {
		t.Fatal(err)
	}

	r, _ := f.runToEnd(t, wf)
	if r.Status != run.StatusSucceeded {
		t.Fatalf("run %s: %q", r.Status, step(r, "script").Error)
	}
	st := step(r, "script")
	if !strings.Contains(string(st.Outputs), `"stdout":"{{ not an expression }} v1.4.0 ••••••"`) {
		t.Errorf("script outputs = %s", st.Outputs)
	}
	if !strings.Contains(string(st.Config), `{{ not an expression }}`) || strings.Contains(string(st.Config), "s3cr3t") {
		t.Errorf("recorded config = %s", st.Config)
	}
	if out := string(step(r, "after").Outputs); out != `{"value":"v1.4.0"}` {
		t.Errorf("after outputs = %s", out)
	}
}

func TestLiteralFieldsSkipValidation(t *testing.T) {
	f := newFixture(t, 8)
	wf := build([]stepSpec{{id: "s", block: "script", config: `{"language":"go","code":"x := []T{{1, 2}}"}`}})
	if err := workflow.Validate(wf, f.engine.blocks); err != nil {
		t.Errorf("literal code was checked for expressions: %v", err)
	}
	wf = build([]stepSpec{{id: "s", block: "script", config: `{"language":"go","code":"x","env":{"A":"{{ steps.nope.outputs.x }}"}}`}})
	if err := workflow.Validate(wf, f.engine.blocks); err == nil {
		t.Error("env expressions weren't checked")
	}
}
