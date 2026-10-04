package block

// Builtin returns the blocks that ship with Conductor.
func Builtin() *Registry {
	return NewRegistry(Shell, HTTP, Approval)
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
	},
	Outputs: []Output{
		{Key: "stdout", Description: "Standard output, trimmed"},
		{Key: "stderr", Description: "Standard error, trimmed"},
		{Key: "exitCode", Description: "Process exit code"},
	},
	Run: runShell,
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
