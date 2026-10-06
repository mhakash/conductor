package wfyaml

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"conductor/internal/block"
	"conductor/internal/workflow"
)

// docs/workflow-yaml.md is how people and agents learn to write workflows,
// so it must stay true: its examples import, and every block and field is
// described.
const docPath = "../../docs/workflow-yaml.md"

var yamlBlock = regexp.MustCompile("(?s)```yaml\n(.*?)```")

func readDoc(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestDocExamplesImport(t *testing.T) {
	blocks := block.Builtin()
	examples := yamlBlock.FindAllStringSubmatch(readDoc(t), -1)
	if len(examples) == 0 {
		t.Fatal("no yaml examples found")
	}
	for _, m := range examples {
		src := m[1]
		name := strings.SplitN(src, "\n", 3)[1]
		wf, err := Decode([]byte(src))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		// Calls to other workflows are left unchecked: they name workflows
		// that only exist where the example is imported.
		if err := workflow.Validate(wf, blocks); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		for _, s := range wf.Definition.Steps {
			b, _ := blocks.Get(s.Block)
			if missing, _ := b.Missing(s.Config); len(missing) > 0 {
				t.Errorf("%s: step %q lacks %v", name, s.ID, missing)
			}
		}
	}
}

func TestDocDescribesEveryBlock(t *testing.T) {
	doc := readDoc(t)
	for _, b := range block.Builtin().All() {
		heading := "### `" + b.Type + "`"
		start := strings.Index(doc, heading)
		if start < 0 {
			t.Errorf("block %q has no section %q", b.Type, heading)
			continue
		}
		section := doc[start+len(heading):]
		if end := strings.Index(section, "\n## "); end >= 0 {
			section = section[:end]
		}
		if end := strings.Index(section, "\n### "); end >= 0 {
			section = section[:end]
		}
		for _, f := range b.Fields {
			if !strings.Contains(section, "| `"+f.Key+"`") {
				t.Errorf("block %q: field %q isn't in its table", b.Type, f.Key)
			}
		}
		for _, o := range b.Outputs {
			if !strings.Contains(section, "`"+o.Key+"`") {
				t.Errorf("block %q: output %q isn't described", b.Type, o.Key)
			}
		}
	}
}
