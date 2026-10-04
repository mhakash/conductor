# 0005 — Expressions, inputs and secrets

Status: accepted (2026-10-04)

## Expressions

Any string in a step's config may contain `{{ … }}` segments, evaluated with
[expr-lang](https://expr-lang.org) just before the step runs. They can read:

| Name                       | Value                                         |
| -------------------------- | --------------------------------------------- |
| `steps.<id>.outputs.<key>` | outputs of a step this step runs after        |
| `inputs.<name>`            | the run's input values                        |
| `secrets.<NAME>`           | decrypted secrets                             |
| `run.id`, `workflow.name`  | run and workflow metadata                     |

- Strings are inserted as is; numbers, booleans, objects and lists as JSON.
- A value that evaluates to nothing (usually a typo, like `outputs.stdot`)
  fails the step with the field and expression named, rather than inserting
  an empty string.
- Expressions are compiled when a workflow is **saved**. A step may only read
  outputs of its ancestors — steps guaranteed to have finished before it — and
  only declared inputs; anything else is rejected with a fix ("connect a → b").
- Each step's resolved config is recorded on the run ("Ran with"), so you can
  see exactly what a command or request received.

expr-lang was chosen over Go `text/template` for its expression syntax
(`a ? b : c`, `upper(x)`, `len(list)`), safety (no side effects, no loops)
and an AST that makes reference checking straightforward.

## Inputs

Workflows declare typed inputs (text, number, yes/no) with optional defaults.
Starting a run applies defaults and converts types; the resolved values are
stored with the run and shown on its page. "Run again" pre-fills them.

## Secrets

- Stored in SQLite encrypted with **AES-256-GCM**; the secret's name is the
  authenticated data, so a ciphertext can't be moved to another name.
- The 256-bit key lives in the **macOS Keychain** (service `conductor`),
  created on first start. `CONDUCTOR_SECRET_KEY` (base64) overrides it; where
  no keychain exists a `0600` file in the data dir is used. Losing the key
  means the stored secrets can't be decrypted — they must be re-entered.
- The API sets, lists and deletes secrets; it **never returns a value**.
- A run refuses to start if a referenced secret doesn't exist.
- Values are masked (`••••••`) in logs, errors, outputs and recorded configs,
  including their JSON-escaped form. Values shorter than 4 characters aren't
  masked, to avoid mangling output.
- Prefer passing secrets to shell steps through **environment variables**
  (`env: { TOKEN: "{{ secrets.TOKEN }}" }`, then `$TOKEN` in the command):
  a value placed directly in the command line is visible to other local
  processes for the command's lifetime.
