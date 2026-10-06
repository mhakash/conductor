package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func caller(id, name string, calls ...string) Workflow {
	w := Workflow{ID: id, Name: name, Definition: EmptyDefinition()}
	for i, c := range calls {
		w.Definition.Steps = append(w.Definition.Steps, Step{
			ID: "call" + string(rune('a'+i)), Block: CallBlock, Config: json.RawMessage(c),
		})
	}
	return w
}

func TestValidateCalls(t *testing.T) {
	stored := map[string]Workflow{
		"greet":   {ID: "greet", Name: "Greet", Definition: Definition{Inputs: []Input{{Name: "name", Type: InputText}}}},
		"release": caller("release", "Release", `{"workflow":"notify"}`),
		"notify":  caller("notify", "Notify", `{"workflow":"build"}`),
	}
	get := func(_ context.Context, id string) (Workflow, error) {
		if w, ok := stored[id]; ok {
			return w, nil
		}
		return Workflow{}, ErrNotFound
	}

	tests := []struct {
		name    string
		w       Workflow
		wantErr string
	}{
		{name: "known workflow and input", w: caller("build", "Build", `{"workflow":"greet","inputs":{"name":"Ada"}}`)},
		{name: "no workflow chosen yet", w: caller("build", "Build", `{}`)},
		{name: "itself", w: caller("build", "Build", `{"workflow":"build"}`), wantErr: "can't run the workflow it belongs to"},
		{name: "unknown workflow", w: caller("build", "Build", `{"workflow":"nope"}`), wantErr: "runs a workflow that doesn't exist"},
		{name: "unknown input", w: caller("build", "Build", `{"workflow":"greet","inputs":{"nme":"Ada"}}`), wantErr: `gives input "nme", which "Greet" doesn't have`},
		{name: "cycle through others", w: caller("build", "Build", `{"workflow":"release"}`), wantErr: "run each other forever: Build → Release → Notify → Build"},
		{name: "new workflow can't loop", w: caller("", "New", `{"workflow":"release"}`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCalls(context.Background(), tt.w, get)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			var verr *ValidationError
			if !errors.As(err, &verr) || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
			if verr.Problems[0].Step != "calla" {
				t.Errorf("problem not attached to the step: %+v", verr.Problems[0])
			}
		})
	}
}

func TestValidateOutputs(t *testing.T) {
	tests := []struct {
		outputs []Output
		wantErr string
	}{
		{outputs: []Output{{Name: "version", Value: "steps.tag.outputs.stdout"}, {Name: "who", Value: "inputs.owner"}}},
		{outputs: []Output{{Name: "1st", Value: "steps.tag.outputs.stdout"}}, wantErr: `output name "1st" must start with a letter`},
		{outputs: []Output{{Name: "v", Value: "1"}, {Name: "v", Value: "2"}}, wantErr: `output "v" is defined more than once`},
		{outputs: []Output{{Name: "v", Value: " "}}, wantErr: `output "v" needs a value`},
		{outputs: []Output{{Name: "v", Value: "steps.tagg.outputs.stdout"}}, wantErr: `output "v" uses step "tagg", which doesn't exist`},
		{outputs: []Output{{Name: "v", Value: "inputs.nope"}}, wantErr: `output "v" uses input "nope"`},
		{outputs: []Output{{Name: "v", Value: "item"}}, wantErr: `output "v" uses item`},
		{outputs: []Output{{Name: "v", Value: "steps.tag.outputs.("}}, wantErr: `output "v" value:`},
	}
	for _, tt := range tests {
		w := wf([]string{"tag"})
		w.Definition.Inputs = []Input{{Name: "owner", Type: InputText}}
		w.Definition.Outputs = tt.outputs
		err := Validate(w, blockSet(known))
		switch {
		case tt.wantErr == "" && err != nil:
			t.Errorf("%v: unexpected error %v", tt.outputs, err)
		case tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)):
			t.Errorf("%v: error = %v, want %q", tt.outputs, err, tt.wantErr)
		}
	}
}
