package block

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"
)

type scriptConfig struct {
	Language string            `json:"language"`
	Code     string            `json:"code"`
	Workdir  string            `json:"workdir"`
	Env      map[string]string `json:"env"`
	Image    string            `json:"image"`
}

// language is how a script in one language is saved and run.
type language struct {
	file string
	// command runs the saved file, given its path.
	command func(file string) []string
}

var languages = map[string]language{
	"python": {"main.py", func(f string) []string { return []string{"python3", f} }},
	"node":   {"main.mjs", func(f string) []string { return []string{"node", f} }},
	"bash":   {"main.sh", func(f string) []string { return []string{"bash", f} }},
	// Without a go.mod, only the standard library can be imported.
	"go": {"main.go", func(f string) []string { return []string{"go", "run", f} }},
}

func runScript(ctx context.Context, raw json.RawMessage, env Env) (map[string]any, error) {
	var cfg scriptConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	lang, ok := languages[cfg.Language]
	if !ok {
		return nil, fmt.Errorf("unknown language %q", cfg.Language)
	}
	code := cfg.Code
	if !strings.HasSuffix(code, "\n") {
		code += "\n"
	}
	return runProcess(ctx, process{
		command: func(dir string) []string {
			return lang.command(path.Join(dir, lang.file))
		},
		hostShell: true,
		workdir:   cfg.Workdir,
		env:       cfg.Env,
		image:     cfg.Image,
		files:     map[string]string{lang.file: code},
		describe:  strings.Join(lang.command(lang.file), " "),
	}, env)
}
