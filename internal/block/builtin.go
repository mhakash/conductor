package block

import "conductor/internal/workflow"

// Builtin returns the blocks that ship with Conductor.
func Builtin() *Registry {
	return NewRegistry(Shell, Script, HTTP, Approval, Workflow)
}

// Workflow runs another workflow as one step. Like Approval it has no
// RunFunc: the engine starts the workflow as a child run and waits for it.
// Its outputs are the called workflow's outputs, so it declares none here.
var Workflow = Block{
	Type:        workflow.CallBlock,
	Name:        "Run workflow",
	Description: "Run another workflow as one step and use its outputs.",
	Category:    "Control",
	Icon:        "workflow",
	Fields: []Field{
		{Key: "workflow", Label: "Workflow", Type: FieldWorkflow, Required: true},
		{
			Key: "inputs", Label: "Inputs", Type: FieldInputs,
			Help: "Leave an input empty to use its default.",
		},
	},
	Outputs: []Output{},
}

// ApprovalType is the block the engine runs itself: it doesn't do work, it
// pauses the run until someone decides on the run page.
const ApprovalType = "approval"

var Approval = Block{
	Type:        ApprovalType,
	Name:        "Approval",
	Description: "Pause the run until you approve or reject it on the run page. Rejecting fails the step.",
	Category:    "Control",
	Icon:        "hand",
	Fields: []Field{
		{
			Key: "message", Label: "Message", Type: FieldTextarea,
			Placeholder: "Push {{ inputs.version }} to GitLab?",
			Help:        "Shown on the run page while it waits. Without a timeout it waits indefinitely, even if Conductor restarts.",
		},
	},
	Outputs: []Output{
		{Key: "approved", Description: "true if approved"},
		{Key: "comment", Description: "Comment left with the decision"},
	},
}

var Shell = Block{
	Type:        "shell",
	Name:        "Shell command",
	Description: "Run a command with your shell, in a working directory of your choice.",
	Category:    "System",
	Icon:        "terminal",
	Fields: []Field{
		{
			Key: "command", Label: "Command", Type: FieldCode, Language: "shell", Required: true,
			Placeholder: "git tag -a v1.4.0 -m \"Release 1.4.0\"",
		},
		{
			Key: "workdir", Label: "Working directory", Type: FieldText,
			Placeholder: "~/code/my-service",
			Help:        "Defaults to your home directory.",
		},
		{
			Key: "shell", Label: "Shell", Type: FieldSelect,
			Options: []Option{{"", "Login shell ($SHELL)"}, {"bash", "bash"}, {"zsh", "zsh"}, {"sh", "sh"}},
			Help:    "Runs as a login shell, so PATH matches your terminal.",
		},
		{Key: "env", Label: "Environment variables", Type: FieldKeyValue},
		imageField,
	},
	Outputs: processOutputs,
	Run:     runShell,
}

// imageField runs a shell or script step in a container instead of on the
// host (ADR 0008).
var imageField = Field{
	Key: "image", Label: "Docker image", Type: FieldText,
	Placeholder: "python:3.13-slim",
	Help:        "Runs the step in a fresh container of this image instead of on your machine. Only the working directory, if you set one, is mounted (at /work).",
}

var processOutputs = []Output{
	{Key: "stdout", Description: "Standard output, trimmed"},
	{Key: "stderr", Description: "Standard error, trimmed"},
	{Key: "exitCode", Description: "Process exit code"},
	{Key: "result", Description: "JSON the step wrote to the file named by $" + OutputsEnv},
}

var Script = Block{
	Type:        "script",
	Name:        "Script",
	Description: "Run Python, Node, Bash or Go code written here. Pass data in through environment variables.",
	Category:    "System",
	Icon:        "code",
	Fields: []Field{
		{
			Key: "language", Label: "Language", Type: FieldSelect, Default: "python",
			Options: []Option{{"python", "Python"}, {"node", "Node.js"}, {"bash", "Bash"}, {"go", "Go"}},
			Help:    "Runs python3, node, bash or go run from your PATH, or from the Docker image.",
		},
		{
			Key: "code", Label: "Code", Type: FieldCode, Required: true, Literal: true,
			LanguageFrom: "language", Placeholders: scriptPlaceholders,
			Help: "Taken as written: use environment variables, not {{ }}, to bring in values. Write JSON to the file named by $" + OutputsEnv + " to return it as result.",
		},
		{
			Key: "env", Label: "Environment variables", Type: FieldKeyValue,
			Help: "Values may use {{ }}, e.g. VERSION = {{ inputs.version }}.",
		},
		{
			Key: "workdir", Label: "Working directory", Type: FieldText,
			Placeholder: "~/code/my-service",
			Help:        "Defaults to your home directory.",
		},
		imageField,
	},
	Outputs: processOutputs,
	Run:     runScript,
}

var scriptPlaceholders = map[string]string{
	"python": `import json, os

version = os.environ["VERSION"]
print(f"releasing {version}")

with open(os.environ["CONDUCTOR_OUTPUTS"], "w") as f:
    json.dump({"tag": f"v{version}"}, f)`,
	"node": `import { writeFileSync } from 'node:fs'

const version = process.env.VERSION
console.log(` + "`releasing ${version}`" + `)

writeFileSync(process.env.CONDUCTOR_OUTPUTS, JSON.stringify({ tag: ` + "`v${version}`" + ` }))`,
	"bash": `set -euo pipefail

echo "releasing $VERSION"
printf '{"tag": "v%s"}' "$VERSION" > "$CONDUCTOR_OUTPUTS"`,
	"go": `package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	version := os.Getenv("VERSION")
	fmt.Println("releasing", version)

	out, _ := json.Marshal(map[string]string{"tag": "v" + version})
	os.WriteFile(os.Getenv("CONDUCTOR_OUTPUTS"), out, 0o644)
}`,
}

var HTTP = Block{
	Type:        "http",
	Name:        "HTTP request",
	Description: "Call an HTTP endpoint. Non-2xx responses fail the step.",
	Category:    "Network",
	Icon:        "globe",
	Fields: []Field{
		{
			Key: "method", Label: "Method", Type: FieldSelect, Default: "GET",
			Options: []Option{
				{"GET", "GET"}, {"POST", "POST"}, {"PUT", "PUT"},
				{"PATCH", "PATCH"}, {"DELETE", "DELETE"},
			},
		},
		{
			Key: "url", Label: "URL", Type: FieldText, Required: true,
			Placeholder: "https://gitlab.example.com/api/v4/projects",
		},
		{Key: "headers", Label: "Headers", Type: FieldKeyValue},
		{Key: "body", Label: "Body", Type: FieldCode, Language: "json"},
	},
	Outputs: []Output{
		{Key: "status", Description: "HTTP status code"},
		{Key: "headers", Description: "Response headers"},
		{Key: "body", Description: "Response body as text"},
		{Key: "json", Description: "Response body parsed as JSON, when it is JSON"},
	},
	Run: runHTTP,
}
