import { Plus, Trash2, X } from 'lucide-react'
import { useMemo, useState, type ReactNode } from 'react'
import { useShallow } from 'zustand/react/shallow'

import { Button } from '@/components/ui/Button'
import { FormField, Input, Select, Textarea } from '@/components/ui/Input'
import { BlockIcon } from '@/features/blocks/BlockIcon'
import type { Block } from '@/features/blocks/api'
import { useSecrets } from '@/features/secrets/api'
import type { InputType, WorkflowInput } from '@/features/workflows/api'

import { ConfigField } from './fields/ConfigField'
import { ExpressionScope, type ExpressionOption } from './fields/expressions'
import { ancestorsOf, type StepNode } from './graph'
import { StepSettings } from './StepSettings'
import { useEditor } from './store'

export function Inspector({ blocks }: { blocks: Map<string, Block> }) {
  const selected = useEditor(useShallow((s) => s.nodes.filter((n) => n.selected)))
  const step = selected.length === 1 ? selected[0] : null
  const block = step ? blocks.get(step.data.block) : undefined

  return (
    <aside
      aria-label="Inspector"
      className="hidden w-80 shrink-0 flex-col overflow-y-auto border-l border-line bg-surface lg:flex"
    >
      {step && block ? (
        <StepInspector key={step.id} step={step} block={block} blocks={blocks} />
      ) : (
        <WorkflowInspector selectedCount={selected.length} />
      )}
    </aside>
  )
}

function Section({ title, children }: { title?: string; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-4 border-b border-line p-4 last:border-b-0">
      {title && (
        <h3 className="text-[11px] font-medium tracking-wide text-fg-subtle uppercase">{title}</h3>
      )}
      {children}
    </section>
  )
}

function StepInspector({
  step,
  block,
  blocks,
}: {
  step: StepNode
  block: Block
  blocks: Map<string, Block>
}) {
  const scope = useStepScope(step.id, blocks)
  const configScope = useMemo(
    () => (step.data.forEach ? [...itemScope, ...scope] : scope),
    [step.data.forEach, scope],
  )
  const updateStep = useEditor((s) => s.updateStep)
  const removeStep = useEditor((s) => s.removeStep)
  const problems = useEditor(useShallow((s) => s.problems.filter((p) => p.step === step.id)))

  const setConfig = (key: string, value: unknown) =>
    updateStep(step.id, { config: { ...step.data.config, [key]: value } })

  return (
    <>
      <div className="flex items-start gap-3 border-b border-line p-4">
        <span className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-hover text-brass">
          <BlockIcon name={block.icon} className="size-4" />
        </span>
        <div className="min-w-0">
          <p className="font-medium text-fg">{block.name}</p>
          <p className="text-xs text-fg-subtle">{block.description}</p>
        </div>
      </div>

      {problems.length > 0 && (
        <ul className="flex flex-col gap-1 border-b border-line bg-coral/5 p-4 text-xs text-coral">
          {problems.map((p) => (
            <li key={p.message}>{p.message}</li>
          ))}
        </ul>
      )}

      <Section>
        <FormField label="Name" htmlFor="step-name">
          <Input
            id="step-name"
            value={step.data.name}
            placeholder={block.name}
            onChange={(e) => updateStep(step.id, { name: e.target.value })}
          />
        </FormField>
        <StepIdField id={step.id} />
      </Section>

      {block.fields.length > 0 && (
        <Section title="Configuration">
          <p className="-mt-2 text-xs text-fg-subtle">
            Type <code className="font-mono text-brass">{'{{'}</code> to use outputs of earlier
            steps, inputs or secrets.
          </p>
          <ExpressionScope.Provider value={configScope}>
            {block.fields.map((f) => (
              <ConfigField
                key={f.key}
                field={f}
                value={step.data.config[f.key]}
                onChange={(v) => setConfig(f.key, v)}
              />
            ))}
          </ExpressionScope.Provider>
        </Section>
      )}

      <Section title="Settings">
        <ExpressionScope.Provider value={scope}>
          <StepSettings
            stepId={step.id}
            block={step.data.block}
            settings={step.data}
            onChange={(patch) => updateStep(step.id, patch)}
          />
        </ExpressionScope.Provider>
      </Section>

      {block.outputs.length > 0 && (
        <Section title="Outputs for later steps">
          <dl className="flex flex-col gap-2.5">
            {outputsOf(step.id, block, !!step.data.forEach).map((o) => (
              <div key={o.path}>
                <dt className="font-mono text-xs break-all text-fg">
                  <span className="text-fg-subtle">
                    {o.path.slice(0, o.path.lastIndexOf('.') + 1)}
                  </span>
                  {o.path.slice(o.path.lastIndexOf('.') + 1)}
                </dt>
                <dd className="text-xs text-fg-subtle">{o.detail}</dd>
              </div>
            ))}
          </dl>
        </Section>
      )}

      <Section>
        <Button
          variant="ghost"
          className="w-fit text-coral hover:bg-coral/10 hover:text-coral"
          onClick={() => removeStep(step.id)}
        >
          <Trash2 className="size-3.5" /> Delete step
        </Button>
      </Section>
    </>
  )
}

/** Edits the step id locally and applies it on blur or Enter, so typing isn't blocked by validation. */
function StepIdField({ id }: { id: string }) {
  const renameStep = useEditor((s) => s.renameStep)
  const [draft, setDraft] = useState(id)
  const [error, setError] = useState<string | null>(null)

  const commit = () => {
    const err = renameStep(id, draft.trim())
    setError(err)
  }

  return (
    <FormField
      label="Step ID"
      htmlFor="step-id"
      error={error ?? undefined}
      help="Other steps use this to refer to this step's outputs."
    >
      <Input
        id="step-id"
        value={draft}
        aria-invalid={!!error}
        className="font-mono text-xs"
        spellCheck={false}
        onChange={(e) => setDraft(e.target.value)}
        onBlur={commit}
        onKeyDown={(e) => {
          if (e.key === 'Enter') e.currentTarget.blur()
          if (e.key === 'Escape') {
            setDraft(id)
            setError(null)
          }
        }}
      />
    </FormField>
  )
}

function WorkflowInspector({ selectedCount }: { selectedCount: number }) {
  const description = useEditor((s) => s.description)
  const stepCount = useEditor((s) => s.nodes.length)
  const edgeCount = useEditor((s) => s.edges.length)
  const setDescription = useEditor((s) => s.setDescription)
  // Input problems are shown next to the inputs.
  const problems = useEditor(
    useShallow((s) => s.problems.filter((p) => !p.step && !p.message.startsWith('input'))),
  )

  return (
    <>
      <Section title="Workflow">
        <FormField label="Description" htmlFor="wf-description">
          <Textarea
            id="wf-description"
            value={description}
            placeholder="What this workflow is for"
            onChange={(e) => setDescription(e.target.value)}
          />
        </FormField>
        {problems.length > 0 && (
          <ul className="flex flex-col gap-1 text-xs text-coral">
            {problems.map((p) => (
              <li key={p.message}>{p.message}</li>
            ))}
          </ul>
        )}
      </Section>
      <InputsEditor />
      <Section>
        <p className="font-mono text-xs text-fg-subtle">
          {stepCount} {stepCount === 1 ? 'step' : 'steps'} · {edgeCount}{' '}
          {edgeCount === 1 ? 'dependency' : 'dependencies'}
        </p>
        <p className="text-fg-muted">
          {selectedCount > 1
            ? `${selectedCount} steps selected. Select a single step to configure it.`
            : 'Select a step to configure it.'}
        </p>
      </Section>
    </>
  )
}

/** Everything an expression in this step can refer to, for autocomplete. */
function useStepScope(stepId: string, blocks: Map<string, Block>): ExpressionOption[] {
  const nodes = useEditor((s) => s.nodes)
  const edges = useEditor((s) => s.edges)
  const inputs = useEditor((s) => s.inputs)
  const secrets = useSecrets().data

  return useMemo(() => {
    const ancestors = ancestorsOf(stepId, edges)
    const options: ExpressionOption[] = []
    for (const n of nodes) {
      if (!ancestors.has(n.id)) continue
      const b = blocks.get(n.data.block)
      if (!b) continue
      for (const o of outputsOf(n.id, b, !!n.data.forEach)) {
        options.push({ ...o, detail: `${n.data.name || n.id} · ${o.detail}` })
      }
    }
    for (const i of inputs) {
      options.push({
        path: `inputs.${i.name}`,
        detail: i.label || `${i.type} input`,
        kind: 'input',
      })
    }
    for (const sec of secrets ?? []) {
      options.push({ path: `secrets.${sec.name}`, detail: 'Secret', kind: 'secret' })
    }
    options.push(
      { path: 'run.id', detail: 'This run’s id', kind: 'run' },
      { path: 'workflow.name', detail: 'Workflow name', kind: 'run' },
    )
    return options
  }, [stepId, nodes, edges, inputs, secrets, blocks])
}

const itemScope: ExpressionOption[] = [
  { path: 'item', detail: 'The current item of the list', kind: 'item' },
  { path: 'index', detail: 'Its position, from 0', kind: 'item' },
]

/** What later steps can read of a step: its outputs (per item, for a loop) and its status. */
function outputsOf(id: string, block: Block, loops: boolean): ExpressionOption[] {
  const outputs: ExpressionOption[] = loops
    ? [
        {
          path: `steps.${id}.outputs.results`,
          detail: `One per item, each with ${block.outputs.map((o) => o.key).join(', ')}`,
          kind: 'step',
        },
      ]
    : block.outputs.map((o) => ({
        path: `steps.${id}.outputs.${o.key}`,
        detail: o.description,
        kind: 'step',
      }))
  return [
    ...outputs,
    {
      path: `steps.${id}.status`,
      detail: 'succeeded, failed or skipped',
      kind: 'step',
    },
  ]
}

const inputTypes: { value: InputType; label: string }[] = [
  { value: 'text', label: 'Text' },
  { value: 'number', label: 'Number' },
  { value: 'boolean', label: 'Yes / no' },
]

function InputsEditor() {
  const inputs = useEditor((s) => s.inputs)
  const setInputs = useEditor((s) => s.setInputs)
  const problems = useEditor(
    useShallow((s) => s.problems.filter((p) => p.message.startsWith('input'))),
  )

  const update = (i: number, patch: Partial<WorkflowInput>) =>
    setInputs(inputs.map((input, j) => (j === i ? { ...input, ...patch } : input)))

  const add = () => {
    const taken = new Set(inputs.map((i) => i.name))
    let name = 'version'
    for (let n = 2; taken.has(name); n++) name = `input${n}`
    setInputs([...inputs, { name, type: 'text', required: false }])
  }

  return (
    <Section title="Inputs">
      <p className="-mt-2 text-xs text-fg-subtle">
        Asked for when the workflow runs. Steps read them as{' '}
        <code className="font-mono">inputs.name</code>.
      </p>
      {inputs.map((input, i) => (
        <div
          key={i}
          className="flex flex-col gap-2 rounded-lg border border-line bg-canvas/50 p-2.5"
        >
          <div className="flex gap-1.5">
            <Input
              aria-label="Input name"
              value={input.name}
              spellCheck={false}
              className="font-mono text-xs"
              onChange={(e) => update(i, { name: e.target.value })}
            />
            <Select
              aria-label="Type"
              value={input.type}
              className="w-28 shrink-0"
              onChange={(e) => update(i, { type: e.target.value as InputType, default: undefined })}
            >
              {inputTypes.map((t) => (
                <option key={t.value} value={t.value}>
                  {t.label}
                </option>
              ))}
            </Select>
            <button
              aria-label={`Remove input ${input.name}`}
              onClick={() => setInputs(inputs.filter((_, j) => j !== i))}
              className="flex size-8 shrink-0 items-center justify-center rounded-md text-fg-subtle hover:bg-hover hover:text-fg"
            >
              <X className="size-3.5" />
            </button>
          </div>
          <div className="flex items-center gap-1.5">
            {input.type === 'boolean' ? (
              <Select
                aria-label="Default"
                value={input.default === true ? 'true' : 'false'}
                onChange={(e) => update(i, { default: e.target.value === 'true' })}
              >
                <option value="false">Default: no</option>
                <option value="true">Default: yes</option>
              </Select>
            ) : (
              <Input
                aria-label="Default value"
                placeholder="Default (optional)"
                type={input.type === 'number' ? 'number' : 'text'}
                value={input.default === undefined ? '' : String(input.default)}
                onChange={(e) =>
                  update(i, {
                    default:
                      e.target.value === ''
                        ? undefined
                        : input.type === 'number'
                          ? e.target.valueAsNumber
                          : e.target.value,
                  })
                }
              />
            )}
            <label className="flex shrink-0 items-center gap-1.5 px-1 text-xs text-fg-muted">
              <input
                type="checkbox"
                checked={!!input.required}
                onChange={(e) => update(i, { required: e.target.checked })}
                className="accent-brass"
              />
              Required
            </label>
          </div>
        </div>
      ))}
      {problems.length > 0 && (
        <ul className="flex flex-col gap-1 text-xs text-coral">
          {problems.map((p) => (
            <li key={p.message}>{p.message}</li>
          ))}
        </ul>
      )}
      <Button variant="ghost" className="w-fit" onClick={add}>
        <Plus className="size-3.5" /> Add input
      </Button>
    </Section>
  )
}
