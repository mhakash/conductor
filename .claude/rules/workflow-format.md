---
paths:
  - "internal/block/**"
  - "internal/workflow/**"
  - "internal/wfyaml/**"
  - "internal/tmpl/**"
  - "**/*.yaml"
  - "**/*.yml"
---

# Workflow format

The YAML format and every block field are documented in
`docs/reference/workflow-yaml.md`. Read it before writing a workflow file.
Changing blocks, fields, outputs, step settings, inputs/outputs, expression
rules or validation limits: update that doc in the same change —
`go test ./internal/wfyaml` imports every example and fails on undocumented
fields.
