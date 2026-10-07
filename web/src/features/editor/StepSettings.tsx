import type { ReactNode } from 'react'

import { FormField, Input } from '@/components/ui/Input'
import { Select } from '@/components/ui/Select'
import type { StepSettings as Settings } from '@/features/workflows/api'

import { CodeField } from './fields/CodeField'

// Blocks that wait for someone rather than do work.
const WAITING_BLOCKS = new Set(['approval', 'input'])

/**
 * How a step runs, whatever its block: condition, loop, retries, failure
 * handling and timeout (ADR 0006). Expressions here can't use the loop item,
 * so the caller provides a scope without it.
 */
export function StepSettings({
  stepId,
  block,
  settings,
  onChange,
}: {
  stepId: string
  block: string
  settings: Settings
  onChange: (patch: Partial<Settings>) => void
}) {
  const waits = WAITING_BLOCKS.has(block)
  const retry = settings.retry
  const loop = settings.forEach

  return (
    <>
      <FormField
        label="Run only if"
        htmlFor="step-if"
        help="Skips the step unless this is true. Steps after it are skipped too, unless they also depend on a step that ran."
      >
        <CodeField
          id="step-if"
          expression
          value={settings.if ?? ''}
          placeholder="steps.pipeline.outputs.json.status == 'success'"
          onChange={(v) => onChange({ if: v })}
        />
      </FormField>

      {!waits && (
        <Toggle
          id="step-foreach"
          label="Run once per item in a list"
          checked={!!loop}
          onChange={(on) => onChange({ forEach: on ? { items: '' } : undefined })}
        >
          {loop && (
            <div className="flex flex-col gap-3 pt-1">
              <FormField
                label="List"
                htmlFor="step-items"
                help={
                  <>
                    In the configuration, the current item is{' '}
                    <code className="font-mono text-brass">item</code> and its position{' '}
                    <code className="font-mono text-brass">index</code>.
                  </>
                }
              >
                <CodeField
                  id="step-items"
                  expression
                  value={loop.items}
                  placeholder="inputs.tickets"
                  onChange={(v) => onChange({ forEach: { ...loop, items: v } })}
                />
              </FormField>
              <FormField
                label="Items at a time"
                htmlFor="step-concurrency"
                help="1 runs them in order."
              >
                <Input
                  id="step-concurrency"
                  type="number"
                  min={1}
                  max={32}
                  value={loop.concurrency ?? 1}
                  className="w-24 font-mono text-xs"
                  onChange={(e) =>
                    onChange({ forEach: { ...loop, concurrency: e.target.valueAsNumber || 1 } })
                  }
                />
              </FormField>
            </div>
          )}
        </Toggle>
      )}

      {!waits && (
        <div className="flex flex-col gap-3">
          <FormField
            label="Retries"
            htmlFor="step-retries"
            help="How many more times to try if it fails. Timeouts count as failures."
          >
            <Input
              id="step-retries"
              type="number"
              min={0}
              max={20}
              value={retry?.max ?? 0}
              className="w-24 font-mono text-xs"
              onChange={(e) => {
                const max = e.target.valueAsNumber || 0
                onChange({ retry: max > 0 ? { ...retry, max } : undefined })
              }}
            />
          </FormField>
          {retry && (
            <div className="grid grid-cols-[6rem_1fr] gap-2">
              <FormField label="Wait" htmlFor="step-retry-delay">
                <Input
                  id="step-retry-delay"
                  value={retry.delay ?? ''}
                  placeholder="10s"
                  spellCheck={false}
                  className="font-mono text-xs"
                  onChange={(e) => onChange({ retry: { ...retry, delay: e.target.value } })}
                />
              </FormField>
              <FormField label="Between attempts" htmlFor="step-retry-backoff">
                <Select
                  id="step-retry-backoff"
                  value={retry.backoff ?? 'fixed'}
                  onValueChange={(v) =>
                    onChange({ retry: { ...retry, backoff: v as 'fixed' | 'exponential' } })
                  }
                  options={[
                    { value: 'fixed', label: 'Same wait each time' },
                    { value: 'exponential', label: 'Double it each time' },
                  ]}
                />
              </FormField>
            </div>
          )}
        </div>
      )}

      <Toggle
        id="step-continue"
        label="Keep going if this step fails"
        checked={!!settings.continueOnError}
        onChange={(on) => onChange({ continueOnError: on })}
        help={
          <>
            Steps after it still run, and the run can succeed. They can check{' '}
            <code className="font-mono text-brass">steps.{stepId}.status</code>.
          </>
        }
      />

      <FormField
        label="Timeout"
        htmlFor="step-timeout"
        help={
          waits
            ? 'How long to wait for an answer, e.g. 4h. Without one, it waits until someone answers.'
            : 'How long the step may run, e.g. 30s, 10m or 2h. Defaults to 1h.'
        }
      >
        <Input
          id="step-timeout"
          value={settings.timeout ?? ''}
          placeholder={waits ? 'No limit' : '1h'}
          className="font-mono text-xs"
          spellCheck={false}
          onChange={(e) => onChange({ timeout: e.target.value })}
        />
      </FormField>
    </>
  )
}

function Toggle({
  id,
  label,
  checked,
  onChange,
  help,
  children,
}: {
  id: string
  label: string
  checked: boolean
  onChange: (checked: boolean) => void
  help?: ReactNode
  children?: ReactNode
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <label
        htmlFor={id}
        className="relative flex cursor-pointer items-center gap-2.5 text-sm text-fg"
      >
        <input
          id={id}
          type="checkbox"
          role="switch"
          checked={checked}
          onChange={(e) => onChange(e.target.checked)}
          className="peer sr-only"
        />
        <span
          aria-hidden
          className="relative h-4 w-7 shrink-0 rounded-full bg-line-strong transition-colors peer-checked:bg-brass peer-focus-visible:ring-2 peer-focus-visible:ring-brass/60 peer-focus-visible:ring-offset-2 peer-focus-visible:ring-offset-surface after:absolute after:top-0.5 after:left-0.5 after:size-3 after:rounded-full after:bg-fg after:transition-transform peer-checked:after:translate-x-3 peer-checked:after:bg-canvas motion-reduce:transition-none motion-reduce:after:transition-none"
        />
        {label}
      </label>
      {help && <p className="pl-[38px] text-xs text-fg-subtle">{help}</p>}
      {children && <div className="pl-[38px]">{children}</div>}
    </div>
  )
}
