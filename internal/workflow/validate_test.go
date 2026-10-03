package workflow

import (
	"errors"
	"strings"
	"testing"
)

func known(string) bool { return true }

func wf(steps []string, edges ...Edge) Workflow {
	def := EmptyDefinition()
	for _, id := range steps {
		def.Steps = append(def.Steps, Step{ID: id, Block: "shell"})
	}
	def.Edges = append(def.Edges, edges...)
	return Workflow{Name: "Build prep", Definition: def}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		w       Workflow
		exists  func(string) bool
		wantErr string
	}{
		{
			name: "empty workflow is valid",
			w:    wf(nil),
		},
		{
			name: "diamond is valid",
			w: wf([]string{"tag", "confluence", "jira", "email"},
				Edge{"tag", "confluence"}, Edge{"tag", "jira"},
				Edge{"confluence", "email"}, Edge{"jira", "email"}),
		},
		{
			name:    "missing name",
			w:       Workflow{Definition: EmptyDefinition()},
			wantErr: "name is required",
		},
		{
			name:    "bad step id",
			w:       wf([]string{"Create-Tag"}),
			wantErr: `step id "Create-Tag" must start with a letter`,
		},
		{
			name:    "duplicate step id",
			w:       wf([]string{"tag", "tag"}),
			wantErr: `step id "tag" is used more than once`,
		},
		{
			name:    "unknown block",
			w:       wf([]string{"tag"}),
			exists:  func(string) bool { return false },
			wantErr: `unknown block "shell"`,
		},
		{
			name:    "dangling edge",
			w:       wf([]string{"tag"}, Edge{"tag", "push"}),
			wantErr: "references a missing step",
		},
		{
			name:    "self loop",
			w:       wf([]string{"tag"}, Edge{"tag", "tag"}),
			wantErr: "cannot depend on itself",
		},
		{
			name:    "duplicate edge",
			w:       wf([]string{"a", "b"}, Edge{"a", "b"}, Edge{"a", "b"}),
			wantErr: "is duplicated",
		},
		{
			name: "bad timeout",
			w: func() Workflow {
				w := wf([]string{"tag"})
				w.Definition.Steps[0].Timeout = "soon"
				return w
			}(),
			wantErr: `invalid timeout "soon"`,
		},
		{
			name:    "cycle",
			w:       wf([]string{"a", "b", "c"}, Edge{"a", "b"}, Edge{"b", "c"}, Edge{"c", "a"}),
			wantErr: "steps form a cycle: a → b → c → a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists := tt.exists
			if exists == nil {
				exists = known
			}
			err := Validate(tt.w, exists)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			var verr *ValidationError
			if !errors.As(err, &verr) {
				t.Fatalf("got %v, want *ValidationError", err)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err, tt.wantErr)
			}
		})
	}
}
