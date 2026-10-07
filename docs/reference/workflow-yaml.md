# Writing workflows in YAML

This is the complete reference for writing a Conductor workflow as YAML by
hand (or by an agent) and importing it: on the Workflows page (**Import**), in
the editor (**YAML** → edit → **Apply to workflow**), or with
`POST /api/workflows/import` (body: the YAML). Why the format looks like this
is in [ADR 0009](../adr/0009-yaml.md).

Every `yaml` example in this file is imported and validated by
`internal/wfyaml/doc_test.go`, and that test fails if a block or config field
is missing from this page. Keep it that way: see [Keeping this page
current](#keeping-this-page-current).

## A complete example

```yaml
conductor: 1
name: Release
description: Tag a version, check CI, then announce it
inputs:
  - name: version
    type: text
    required: true
  - name: dryRun
    label: Dry run
    type: boolean
    default: true
outputs:
  - name: tag
    value: steps.tag.outputs.result.tag
    description: The tag that was pushed
steps:
  - id: tag
    name: Create tag
    block: script
    config:
      language: python
      code: |
        import json, os
        tag = "v" + os.environ["VERSION"]
        print("tagging", tag)
        json.dump({"tag": tag}, open(os.environ["CONDUCTOR_OUTPUTS"], "w"))
      env:
        VERSION: "{{ inputs.version }}"
  - id: pipeline
    name: Check CI
    block: http
    needs: [tag]
    retry: { max: 5, delay: 30s, backoff: exponential }
    config:
      url: "https://gitlab.example.com/api/v4/projects/42/pipelines?ref={{ steps.tag.outputs.result.tag }}"
      headers:
        PRIVATE-TOKEN: "{{ secrets.GITLAB_TOKEN }}"
  - id: approve
    name: Approve announcement
    block: approval
    needs: [pipeline]
    if: "!inputs.dryRun"
    config:
      message: "Announce {{ steps.tag.outputs.result.tag }}?"
  - id: announce
    name: Announce
    block: shell
    needs: [approve]
    config:
      command: echo "Released $TAG"
      env:
        TAG: "{{ steps.tag.outputs.result.tag }}"
```

## Top level

| Key           | Required | Meaning                                                            |
| ------------- | -------- | ------------------------------------------------------------------ |
| `conductor`   | yes      | Format version. Always `1`; anything else is refused.              |
| `name`        | yes      | Workflow name, up to 200 characters.                               |
| `description` | no       | Free text.                                                         |
| `inputs`      | no       | Values asked for when a run starts. See [Inputs](#inputs).         |
| `outputs`     | no       | Values handed back to a calling workflow. See [Outputs](#outputs). |
| `steps`       | yes      | The steps, in any order. May be empty (`steps: []`).               |

Unknown keys anywhere are an error (`line 3: unknown key "stpes"`), so check
spelling and case: keys are camelCase (`forEach`, `continueOnError`).

## Inputs

```yaml
conductor: 1
name: Inputs example
inputs:
  - name: version # letters, digits, _; starts with a letter
    label: Version to release # shown when starting a run
    type: text # text | number | boolean
    default: "1.0.0"
    required: true
    description: Semantic version, without the v
  - name: retries
    type: number
    default: 3
  - name: notify
    type: boolean
    default: false
steps: []
```

- Read in expressions as `inputs.<name>`. Only declared inputs may be used.
- `default` must match the type. A `required` input with no default must be
  given when the run starts.
- Names are unique and match `^[a-zA-Z][a-zA-Z0-9_]{0,63}$`.

## Outputs

```yaml
conductor: 1
name: Outputs example
outputs:
  - name: url
    value: steps.deploy.outputs.stdout
    description: Where it was deployed
  - name: summary
    value: "'deployed ' + inputs.service"
inputs:
  - name: service
    type: text
steps:
  - id: deploy
    block: shell
    config:
      command: echo https://example.com
```

- `value` is a **bare expression** (no `{{ }}`), evaluated after every step
  has finished. It may read any step (no `needs` required) and the inputs, and
  keeps its type: a list stays a list.
- If it can't be evaluated (e.g. a typo gives no value), the run fails.
- A workflow that runs this one as a step reads them as
  `steps.<id>.outputs.<name>`.
- Names follow the same rule as inputs.

## Steps

```yaml
conductor: 1
name: Step settings example
inputs:
  - name: services
    type: text
    default: "api,web"
steps:
  - id: build # required, unique: ^[a-z][a-z0-9_]{0,63}$
    name: Build # display name; optional
    block: shell # required, see Blocks below
    timeout: 10m # Go duration; default 1h
    config:
      command: make build
  - id: deploy
    block: shell
    needs: [build] # steps that must finish first
    if: steps.build.status == 'succeeded' # bare expression; skip unless true
    forEach:
      items: split(inputs.services, ",") # bare expression giving a list
      concurrency: 2 # items at a time, 1–32; default 1 (in order)
    retry:
      max: 3 # retries after the first try, 1–20
      delay: 10s # before the first retry; default 10s, at most 1h
      backoff: exponential # fixed (default) or exponential
    continueOnError: true # later steps still run; the run can succeed
    config:
      command: ./deploy.sh "$SERVICE"
      env:
        SERVICE: "{{ item }}"
```

| Key               | Meaning                                                                              |
| ----------------- | ------------------------------------------------------------------------------------ |
| `id`              | Unique; lowercase letters, digits and `_`, starting with a letter.                   |
| `name`            | Display name.                                                                        |
| `block`           | What the step does. One of the [blocks](#blocks).                                    |
| `needs`           | Ids of steps this one runs after. No cycles. Without `needs`, steps run in parallel. |
| `if`              | Bare expression that must be `true` or `false`. False skips the step.                |
| `forEach`         | Run once per list item: `items` (bare expression), `concurrency`.                    |
| `retry`           | `max`, `delay`, `backoff`.                                                           |
| `continueOnError` | `true` lets dependents run and the run succeed if this step fails.                   |
| `timeout`         | Per attempt, e.g. `30s`, `5m`, `2h`. Default `1h`. For approvals: how long to wait.  |
| `config`          | The block's fields, below. Omit fields you don't need.                               |

When a step runs:

- A dependency failed (without `continueOnError`) → the step is skipped, and
  so is everything below it.
- **All** its dependencies were skipped → it is skipped. One branch running
  is enough, so a step that joins an `if` branch and its opposite still runs.
- Otherwise its `if` is evaluated, then it runs.

`approval` steps can't use `retry` or `forEach`.

## Expressions

Two forms, both [expr-lang](https://expr-lang.org) syntax:

- **In config strings:** `{{ … }}` segments, e.g. `"v{{ inputs.version }}"`.
  Strings are inserted as is; numbers, booleans, lists and objects as JSON.
- **Bare:** `if`, `forEach.items` and output `value` are whole expressions
  without braces.

What they can read:

| Name                                     | Value                                                                         |
| ---------------------------------------- | ----------------------------------------------------------------------------- |
| `steps.<id>.outputs.<key>`               | Outputs of a step this one `needs`, directly or further up                    |
| `steps.<id>.status`                      | `succeeded`, `failed` or `skipped` (same rule)                                |
| `inputs.<name>`                          | Declared inputs                                                               |
| `secrets.<NAME>`                         | Secrets stored in Conductor (Secrets page); must exist when the run starts    |
| `item`, `index`                          | Current list element and its position, in the config of a `forEach` step only |
| `run.id`, `workflow.id`, `workflow.name` | Run metadata                                                                  |

Rules checked when importing or saving:

- A step may only read steps it depends on (through `needs`, at any depth).
  Add `needs` rather than relying on timing.
- A step that loops has outputs `results`: a list with one outputs object per
  item, e.g. `steps.deploy.outputs.results[0].stdout`.
- Useful functions: `upper`, `lower`, `trim`, `split`, `join`, `len`,
  `map(list, #.field)`, `filter(list, # > 1)`, `a ? b : c`, `??`.
- An expression that gives no value (usually a misspelt key) fails the step
  instead of inserting an empty string.

## YAML pitfalls

- **Quote any value that starts with `{{` or `{`** — unquoted, YAML reads it
  as a mapping: `VERSION: "{{ inputs.version }}"`.
- **Quote bare expressions that start with `!`, `[`, `'` or `"`**:
  `if: "!inputs.dryRun"`, `items: '["api", "web"]'`.
- **Environment variables, headers and HTTP bodies are strings.** Quote
  numbers and booleans (`PORT: "8080"`); write a JSON body as a string, best
  as a block (`body: |`).
- Use `|` for multi-line commands and code; it keeps newlines exactly.

## Blocks

Each block's config fields, and the outputs later steps can read.

### `shell`

Runs a command with your login shell (so PATH matches your terminal), or in
a Docker container.

| Field     | Required | Value                                                                                                                   |
| --------- | -------- | ----------------------------------------------------------------------------------------------------------------------- |
| `command` | yes      | Shell command; `{{ }}` allowed (prefer `env` for values).                                                               |
| `workdir` | no       | Directory to run in; `~` allowed. Default: home directory.                                                              |
| `shell`   | no       | `bash`, `zsh` or `sh`. Default: your login shell (`$SHELL`).                                                            |
| `env`     | no       | Mapping of environment variables; values may use `{{ }}`.                                                               |
| `image`   | no       | Docker image, e.g. `alpine:3`. Runs in a fresh container with `sh -c`; only `workdir`, if set, is mounted (at `/work`). |

Outputs: `stdout`, `stderr` (trimmed), `exitCode`, and `result` — JSON the
command wrote to the file named by `$CONDUCTOR_OUTPUTS`, if any. A non-zero
exit fails the step.

```yaml
conductor: 1
name: Shell example
steps:
  - id: version
    block: shell
    config:
      command: |
        set -e
        v=$(git describe --tags --abbrev=0)
        echo "{\"version\": \"$v\"}" > "$CONDUCTOR_OUTPUTS"
      workdir: ~/code/my-service
  - id: in_container
    block: shell
    needs: [version]
    config:
      command: echo "building $VERSION"
      image: alpine:3
      env:
        VERSION: "{{ steps.version.outputs.result.version }}"
```

### `script`

Runs code in Python, Node.js, Bash or Go, written in the config.

| Field      | Required | Value                                                                                        |
| ---------- | -------- | -------------------------------------------------------------------------------------------- |
| `language` | yes      | `python` (python3), `node` (an ES module), `bash` or `go` (`go run`, standard library only). |
| `code`     | yes      | The program. **Literal: `{{ }}` is not evaluated here.** Pass values through `env`.          |
| `env`      | no       | Mapping of environment variables; values may use `{{ }}`.                                    |
| `workdir`  | no       | Directory to run in. Default: home directory.                                                |
| `image`    | no       | Docker image with the interpreter, e.g. `python:3.13-slim`; as for `shell`.                  |

Outputs: as for `shell` — `stdout`, `stderr`, `exitCode`, `result`. To return
structured data, write JSON to the file named by `$CONDUCTOR_OUTPUTS`; writing
anything that isn't JSON fails the step.

```yaml
conductor: 1
name: Script example
inputs:
  - name: version
    type: text
    default: "1.4.0"
steps:
  - id: bump
    block: script
    config:
      language: node
      code: |
        import { writeFileSync } from 'node:fs'
        const [major, minor] = process.env.VERSION.split('.').map(Number)
        writeFileSync(process.env.CONDUCTOR_OUTPUTS, JSON.stringify({ next: `${major}.${minor + 1}.0` }))
      env:
        VERSION: "{{ inputs.version }}"
  - id: show
    block: script
    needs: [bump]
    config:
      language: go
      code: |
        package main

        import (
        	"fmt"
        	"os"
        )

        func main() { fmt.Println("next:", os.Getenv("NEXT")) }
      env:
        NEXT: "{{ steps.bump.outputs.result.next }}"
```

### `http`

Calls an HTTP endpoint. A response outside 2xx fails the step.

| Field     | Required | Value                                                                                  |
| --------- | -------- | -------------------------------------------------------------------------------------- |
| `method`  | no       | `GET` (default), `POST`, `PUT`, `PATCH` or `DELETE`.                                   |
| `url`     | yes      | The URL; `{{ }}` allowed.                                                              |
| `headers` | no       | Mapping of header names to string values; `{{ }}` allowed.                             |
| `body`    | no       | Request body as a string. JSON bodies get `Content-Type: application/json` unless set. |

Outputs: `status`, `headers`, `body` (text), and `json` (the parsed body,
when the response is JSON).

```yaml
conductor: 1
name: HTTP example
steps:
  - id: create_issue
    block: http
    config:
      method: POST
      url: https://jira.example.com/rest/api/2/issue
      headers:
        Authorization: "Bearer {{ secrets.JIRA_TOKEN }}"
      body: |
        {"fields": {"project": {"key": "REL"}, "summary": "Release {{ run.id }}"}}
  - id: report
    block: shell
    needs: [create_issue]
    config:
      command: echo "created $KEY"
      env:
        KEY: "{{ steps.create_issue.outputs.json.key }}"
```

### `approval`

Pauses the run until someone approves or rejects it on the run page.
Rejecting fails the step. Waiting survives restarts; `timeout` limits it.

| Field     | Required | Value                                  |
| --------- | -------- | -------------------------------------- |
| `message` | no       | Shown while it waits; `{{ }}` allowed. |

Outputs: `approved` (`true` or `false`), `comment`.

### `workflow`

Runs another saved workflow as one step (a child run) and uses its outputs.

| Field      | Required | Value                                                                                                            |
| ---------- | -------- | ---------------------------------------------------------------------------------------------------------------- |
| `workflow` | yes      | The workflow to run, by **name** (must be unique) or id. It must exist, and must not lead back to this workflow. |
| `inputs`   | no       | Mapping of the called workflow's input names to values; `{{ }}` allowed. Missing inputs use their defaults.      |

Outputs: the called workflow's [outputs](#outputs). With `forEach`, one child
run per item and `results` as usual.

```yaml
conductor: 1
name: Deploy all
inputs:
  - name: version
    type: text
steps:
  - id: deploy
    block: workflow
    forEach:
      items: '["api", "web"]'
      concurrency: 2
    config:
      workflow: Deploy service
      inputs:
        service: "{{ item }}"
        version: "{{ inputs.version }}"
  - id: links
    block: shell
    needs: [deploy]
    config:
      command: echo "$URLS"
      env:
        URLS: "{{ join(map(steps.deploy.outputs.results, #.url), ' ') }}"
```

## Checklist for writing one from scratch

1. Start with `conductor: 1`, `name`, then `inputs` for anything that varies
   per run.
2. One step per action; give each a short `id`. Pick the block: `shell` for
   commands, `script` for logic, `http` for APIs, `approval` to pause,
   `workflow` to reuse a workflow.
3. Add `needs` for order **and** for every step whose outputs a step reads.
4. Pass values with expressions, quoted; put secrets in `env` or headers, not
   in command text.
5. Add `outputs` if other workflows will run this one.
6. Import it. Problems come back with the step and the reason; fix and
   import again.

## Keeping this page current

Update this page in the same change whenever any of these change:

- a block is added, or a block's fields, defaults or outputs change
  (`internal/block/builtin.go`);
- a step setting, input, output or top-level key changes
  (`internal/workflow`, `internal/wfyaml`);
- expression scope or rules change (`internal/tmpl`, `workflow/validate.go`);
- validation limits change (retries, concurrency, items, nesting).

`go test ./internal/wfyaml` checks that every example here imports and
validates, and that every block and config field is documented.
