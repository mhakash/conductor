package workflow

import (
	"encoding/json"
	"fmt"
)

// InputBlock is the block of a step that pauses the run until someone fills
// in its fields on the run page. The engine runs it itself, like approval.
const InputBlock = "input"

// Form is the config of an InputBlock step. Fields are defined like the
// workflow's inputs; the values given become the step's outputs.
type Form struct {
	Message string  `json:"message"`
	Fields  []Input `json:"fields"`
}

// ParseForm reads the config of an input step.
func ParseForm(config json.RawMessage) (Form, error) {
	var f Form
	if len(config) > 0 {
		if err := json.Unmarshal(config, &f); err != nil {
			return Form{}, fmt.Errorf("config: %w", err)
		}
	}
	return f, nil
}

func validateForm(s Step) []Problem {
	f, err := ParseForm(s.Config)
	if err != nil {
		return []Problem{{Step: s.ID, Message: fmt.Sprintf("step %q has an invalid %v", s.ID, err)}}
	}
	problems := validateInputs(f.Fields, "field", true)
	for i := range problems {
		problems[i].Step = s.ID
		problems[i].Message = fmt.Sprintf("step %q: %s", s.ID, problems[i].Message)
	}
	return problems
}
