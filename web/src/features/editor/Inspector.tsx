import { BookmarkPlus, Plus, Trash2, X } from 'lucide-react'
import { useMemo, useState, type ReactNode } from 'react'
import { useShallow } from 'zustand/react/shallow'

import { Button } from '@/components/ui/Button'
import { FormField, Input, Textarea } from '@/components/ui/Input'
import { BlockIcon } from '@/features/blocks/BlockIcon'
import {
  CALL_BLOCK,
  INPUT_BLOCK,
  type Block,
  type CallConfig,
  type FormConfig,
} from '@/features/blocks/api'
import { PresetDialog } from '@/features/presets/PresetDialog'
import { useSecrets } from '@/features/secrets/api'
import { useWorkflowMap, type WorkflowOutput, type WorkflowSummary } from '@/features/workflows/api'

import { CodeField } from './fields/CodeField'
import { ConfigField } from './fields/ConfigField'
import { InputDefsEditor } from './fields/InputDefsEditor'
import { ExpressionScope, type ExpressionOption } from './fields/expressions'
import { ancestorsOf, cleanSettings, type StepData, type StepNode } from './graph'
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
        <WorkflowInspector selectedCount={selected.length} blocks={blocks} />
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
  const workflows = useWorkflowMap()
  const scope = useScope(step.id, blocks)
  const configScope = useMemo(
    () => (step.data.forEach ? [...itemScope, ...scope] : scope),
    [step.data.forEach, scope],
  )
  const updateStep = useEditor((s) => s.updateStep)
  const removeStep = useEditor((s) => s.removeStep)
  const problems = useEditor(useShallow((s) => s.problems.filter((p) => p.step === step.id)))
  const [savingPreset, setSavingPreset] = useState(false)

  const setConfig = (key: string, value: unknown) =>
    updateStep(step.id, { config: { ...step.data.config, [key]: value } })

  // Choosing another workflow keeps the inputs it also has, and takes its
  // name if the step was still named after the old one.
  const chooseWorkflow = (id: string) => {
    const config = step.data.config as CallConfig
    const before = workflows.get(config.workflow ?? '')
    const after = workflows.get(id)
    const names = new Set(after?.inputs.map((i) => i.name))
    const inputs = Object.fromEntries(
      Object.entries(config.inputs ?? {}).filter(([name]) => names.has(name)),
    )
    const renamed = !step.data.name || step.data.name === (before?.name ?? block.name)
    updateStep(step.id, {
      config: { ...step.data.config, workflow: id, inputs },
      ...(renamed && after && { name: after.name }),
    })
  }

  const target =
    step.data.block === CALL_BLOCK
      ? workflows.get((step.data.config as CallConfig).workflow ?? '')
      : undefined
  const outputs = outputsOf(
    step.id,
    declaredOutputs(step.data, block, workflows),
    !!step.data.forEach,
  )
  const { timeout, retry, continueOnError } = cleanSettings(step.data)

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
                config={step.data.config}
                onChange={(v) =>
                  f.type === 'workflow' ? chooseWorkflow(v as string) : setConfig(f.key, v)
                }
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

      <Section title="Outputs for later steps">
        {target && target.outputs.length === 0 && (
          <p className="-mt-2 text-xs text-fg-subtle">
            {target.name} has no outputs. Add them in its workflow settings to use its results here.
          </p>
        )}
        <dl className="flex flex-col gap-2.5">
          {outputs.map((o) => (
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

      <Section>
        <div className="flex flex-wrap gap-1">
          <Button variant="ghost" className="w-fit" onClick={() => setSavingPreset(true)}>
            <BookmarkPlus className="size-3.5" /> Save as preset
          </Button>
          <Button
            variant="ghost"
            className="w-fit text-coral hover:bg-coral/10 hover:text-coral"
            onClick={() => removeStep(step.id)}
          >
            <Trash2 className="size-3.5" /> Delete step
          </Button>
        </div>
      </Section>

      <PresetDialog
        open={savingPreset}
        onOpenChange={setSavingPreset}
        draft={{
          name: step.data.name || block.name,
          description: '',
          block: step.data.block,
          step: { config: step.data.config, timeout, retry, continueOnError },
        }}
      />
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

function WorkflowInspector({
  selectedCount,
  blocks,
}: {
  selectedCount: number
  blocks: Map<string, Block>
}) {
  const description = useEditor((s) => s.description)
  const stepCount = useEditor((s) => s.nodes.length)
  const edgeCount = useEditor((s) => s.edges.length)
  const setDescription = useEditor((s) => s.setDescription)
  // Input and output problems are shown next to them.
  const problems = useEditor(
    useShallow((s) =>
      s.problems.filter(
        (p) => !p.step && !p.message.startsWith('input') && !p.message.startsWith('output'),
      ),
    ),
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
      <OutputsEditor blocks={blocks} />
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

/**
 * Everything an expression in step `stepId` can refer to, for autocomplete:
 * its ancestors' outputs, inputs, secrets and run details. With `stepId`
 * null, what the workflow's outputs can use: every step, and no secrets.
 */
function useScope(stepId: string | null, blocks: Map<string, Block>): ExpressionOption[] {
  const nodes = useEditor((s) => s.nodes)
  const edges = useEditor((s) => s.edges)
  const inputs = useEditor((s) => s.inputs)
  const secrets = useSecrets().data
  const workflows = useWorkflowMap()

  return useMemo(() => {
    const ancestors = stepId ? ancestorsOf(stepId, edges) : null
    const options: ExpressionOption[] = []
    for (const n of nodes) {
      if (ancestors && !ancestors.has(n.id)) continue
      const b = blocks.get(n.data.block)
      if (!b) continue
      for (const o of outputsOf(n.id, declaredOutputs(n.data, b, workflows), !!n.data.forEach)) {
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
    if (stepId) {
      for (const sec of secrets ?? []) {
        options.push({ path: `secrets.${sec.name}`, detail: 'Secret', kind: 'secret' })
      }
    }
    options.push(
      { path: 'run.id', detail: 'This run’s id', kind: 'run' },
      { path: 'workflow.name', detail: 'Workflow name', kind: 'run' },
    )
    return options
  }, [stepId, nodes, edges, inputs, secrets, blocks, workflows])
}

const itemScope: ExpressionOption[] = [
  { path: 'item', detail: 'The current item of the list', kind: 'item' },
  { path: 'index', detail: 'Its position, from 0', kind: 'item' },
]

type DeclaredOutput = { key: string; description: string }

/**
 * A step's outputs: its block's, for a step that runs a workflow that
 * workflow's, and for an input step its fields.
 */
function declaredOutputs(
  data: StepData,
  block: Block,
  workflows: Map<string, WorkflowSummary>,
): DeclaredOutput[] {
  if (data.block === INPUT_BLOCK) {
    return ((data.config as FormConfig).fields ?? []).map((f) => ({
      key: f.name,
      description: f.label || `${f.type} field`,
    }))
  }
  if (data.block !== CALL_BLOCK) return block.outputs
  const target = workflows.get((data.config as CallConfig).workflow ?? '')
  if (!target) return []
  return target.outputs.map((o) => ({
    key: o.name,
    description: o.description || `From ${target.name}`,
  }))
}

/** What later steps can read of a step: its outputs (per item, for a loop) and its status. */
function outputsOf(id: string, declared: DeclaredOutput[], loops: boolean): ExpressionOption[] {
  const outputs: ExpressionOption[] = loops
    ? [
        {
          path: `steps.${id}.outputs.results`,
          detail: declared.length
            ? `One per item, each with ${declared.map((o) => o.key).join(', ')}`
            : 'One per item',
          kind: 'step',
        },
      ]
    : declared.map((o) => ({
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

function InputsEditor() {
  const inputs = useEditor((s) => s.inputs)
  const setInputs = useEditor((s) => s.setInputs)
  const problems = useEditor(
    useShallow((s) => s.problems.filter((p) => p.message.startsWith('input'))),
  )

  return (
    <Section title="Inputs">
      <p className="-mt-2 text-xs text-fg-subtle">
        Asked for when the workflow runs. Steps read them as{' '}
        <code className="font-mono">inputs.name</code>.
      </p>
      <InputDefsEditor value={inputs} onChange={setInputs} noun="input" firstName="version" />
      {problems.length > 0 && (
        <ul className="flex flex-col gap-1 text-xs text-coral">
          {problems.map((p) => (
            <li key={p.message}>{p.message}</li>
          ))}
        </ul>
      )}
    </Section>
  )
}

/** The workflow's outputs, handed to a workflow that runs it as a step. */
function OutputsEditor({ blocks }: { blocks: Map<string, Block> }) {
  const outputs = useEditor((s) => s.outputs)
  const setOutputs = useEditor((s) => s.setOutputs)
  const scope = useScope(null, blocks)
  const problems = useEditor(
    useShallow((s) => s.problems.filter((p) => p.message.startsWith('output'))),
  )

  const update = (i: number, patch: Partial<WorkflowOutput>) =>
    setOutputs(outputs.map((o, j) => (j === i ? { ...o, ...patch } : o)))

  const add = () => {
    const taken = new Set(outputs.map((o) => o.name))
    let name = 'result'
    for (let n = 2; taken.has(name); n++) name = `result${n}`
    setOutputs([...outputs, { name, value: '' }])
  }

  return (
    <Section title="Outputs">
      <p className="-mt-2 text-xs text-fg-subtle">
        What this workflow hands back when another workflow runs it as a step. Each value is an
        expression, evaluated once every step has finished.
      </p>
      <ExpressionScope.Provider value={scope}>
        {outputs.map((output, i) => (
          <div
            key={i}
            className="flex flex-col gap-2 rounded-lg border border-line bg-canvas/50 p-2.5"
          >
            <div className="flex gap-1.5">
              <Input
                aria-label="Output name"
                value={output.name}
                spellCheck={false}
                className="font-mono text-xs"
                onChange={(e) => update(i, { name: e.target.value })}
              />
              <button
                aria-label={`Remove output ${output.name}`}
                onClick={() => setOutputs(outputs.filter((_, j) => j !== i))}
                className="flex size-8 shrink-0 items-center justify-center rounded-md text-fg-subtle hover:bg-hover hover:text-fg"
              >
                <X className="size-3.5" />
              </button>
            </div>
            <CodeField
              id={`output-${i}`}
              expression
              value={output.value}
              placeholder="steps.build.outputs.stdout"
              onChange={(v) => update(i, { value: v })}
            />
            <Input
              aria-label="Description"
              placeholder="Description (optional)"
              value={output.description ?? ''}
              onChange={(e) => update(i, { description: e.target.value || undefined })}
            />
          </div>
        ))}
      </ExpressionScope.Provider>
      {problems.length > 0 && (
        <ul className="flex flex-col gap-1 text-xs text-coral">
          {problems.map((p) => (
            <li key={p.message}>{p.message}</li>
          ))}
        </ul>
      )}
      <Button variant="ghost" className="w-fit" onClick={add}>
        <Plus className="size-3.5" /> Add output
      </Button>
    </Section>
  )
}
