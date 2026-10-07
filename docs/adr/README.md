# Decision records

| ADR                                             | Decides                                                                                 |
| ----------------------------------------------- | --------------------------------------------------------------------------------------- |
| [0001](0001-foundations.md)                     | Local single user, no auth, binds 127.0.0.1; single binary; visual direction            |
| [0002](0002-workflow-definition-and-blocks.md)  | Workflow graph is one versioned JSON doc; blocks describe their own config fields       |
| [0003](0003-execution-model.md)                 | DAG scheduling, parallelism cap, runs are snapshots, login shell, process groups, logs  |
| [0004](0004-live-run-events.md)                 | Runs stream over SSE: snapshot first, then deltas                                       |
| [0005](0005-expressions-inputs-secrets.md)      | `{{ expr }}` (expr-lang), workflow inputs, AES-GCM secrets + output masking             |
| [0006](0006-control-flow.md)                    | `if`, retries, continue-on-error, `forEach` loops, approvals (survive restarts)         |
| [0007](0007-reuse.md)                           | Sub-workflows as child runs, workflow outputs, presets are copies, library page         |
| [0008](0008-code-execution.md)                  | Script block (Python/Node/Bash/Go), `$CONDUCTOR_OUTPUTS` results, opt-in Docker         |
| [0009](0009-yaml.md)                            | YAML is the human format; stored definition stays JSON; `internal/wfyaml` converts      |
| [0010](0010-scratch-and-test-runs.md)           | Run unsaved drafts via `POST /api/runs`; scratch workflow; test runs in the editor      |

New ADR: `NNNN-short-name.md` with `Status: accepted (YYYY-MM-DD)`, then add a row here.
