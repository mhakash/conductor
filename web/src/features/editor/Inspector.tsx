import { Trash2 } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { useShallow } from 'zustand/react/shallow'

import { Button } from '@/components/ui/Button'
import { FormField, Input, Textarea } from '@/components/ui/Input'
import { BlockIcon } from '@/features/blocks/BlockIcon'
import type { Block } from '@/features/blocks/api'

import { ConfigField } from './fields/ConfigField'
import type { StepNode } from './graph'
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
        <StepInspector key={step.id} step={step} block={block} />
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

function StepInspector({ step, block }: { step: StepNode; block: Block }) {
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
          {block.fields.map((f) => (
            <ConfigField
              key={f.key}
              field={f}
              value={step.data.config[f.key]}
              onChange={(v) => setConfig(f.key, v)}
            />
          ))}
        </Section>
      )}

      {block.outputs.length > 0 && (
        <Section title="Outputs">
          <dl className="flex flex-col gap-2.5">
            {block.outputs.map((o) => (
              <div key={o.key}>
                <dt className="font-mono text-xs text-fg">
                  <span className="text-fg-subtle">steps.{step.id}.outputs.</span>
                  {o.key}
                </dt>
                <dd className="text-xs text-fg-subtle">{o.description}</dd>
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
  const problems = useEditor(useShallow((s) => s.problems.filter((p) => !p.step)))

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
