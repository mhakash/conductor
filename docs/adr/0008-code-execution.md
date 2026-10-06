# 0008 — Code execution and Docker isolation

Status: accepted (2026-10-06)

## Script steps

The `script` block runs code written in the editor in Python, Node.js, Bash
or Go. The code is saved to a fresh directory for each attempt (`main.py`,
`main.mjs`, `main.sh`, `main.go`) and run with `python3`, `node`, `bash` or
`go run`, through the user's login shell so PATH matches their terminal, as
for shell steps (ADR 0003). Go scripts have no `go.mod`, so only the standard
library can be imported; anything bigger belongs in a repository and a shell
step.

**Code is literal.** Fields marked `literal` are not searched for `{{ }}`,
on save or at run time: braces are everyday syntax in Go, JavaScript and
Python f-strings, and templating values into source code invites quoting
bugs and injection. Values come in through the step's environment variables,
which do take expressions (`VERSION = {{ inputs.version }}`), and secrets
passed that way stay out of the source and the process list.

## Structured results

Every shell and script step gets `$CONDUCTOR_OUTPUTS`, the path of an empty
file. JSON written to it becomes the step's `result` output
(`steps.bump.outputs.result.next`); anything that isn't JSON fails the step.
A file was chosen over parsing stdout, which scripts use for logging, and
over GitHub Actions' `key=value` lines, which can't carry nested values.

## Docker isolation

Shell and script steps take an optional **Docker image**. When set, the step
runs with `docker run --rm --init` in a fresh container instead of on the
host:

- The step's files are mounted at `/conductor`. The working directory is
  mounted at `/work` only if the step sets one; the home directory is never
  mounted by default, which would defeat the isolation.
- Environment variables are given to the docker client in its environment
  and passed through by name (`-e NAME`), so values don't appear on its
  command line.
- Shell steps run with `sh -c`, which every image has.
- Stopping the step (cancel, timeout, shutdown) force-removes the container;
  killing the docker client alone would leave it running.
- Images are pulled when missing, with the pull shown in the step's log.
  Exit code 125 (Docker couldn't start the container) is reported as such.

It is a setting of these two blocks, not of every step: HTTP, approval and
sub-workflow steps have nothing to isolate.

Docker tests need a running Docker and the `node:24-alpine` image, and only
run with `CONDUCTOR_TEST_DOCKER=1`.
