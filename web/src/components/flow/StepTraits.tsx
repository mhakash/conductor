import { GitBranch, Repeat, RotateCw, ShieldCheck, type LucideIcon } from 'lucide-react'

import { Tooltip } from '@/components/ui/Tooltip'
import type { StepSettings } from '@/features/workflows/api'

type Trait = { icon: LucideIcon; label: string; detail: string }

/** The control-flow settings a step has, in reading order. */
function traitsOf(s: StepSettings): Trait[] {
  const traits: Trait[] = []
  if (s.if?.trim()) traits.push({ icon: GitBranch, label: 'if', detail: `Runs only if ${s.if}` })
  if (s.forEach) {
    const n = s.forEach.concurrency ?? 1
    traits.push({
      icon: Repeat,
      label: 'each',
      detail: `Runs once per item in ${s.forEach.items || '…'}${n > 1 ? `, ${n} at a time` : ''}`,
    })
  }
  if (s.retry && s.retry.max > 0) {
    traits.push({
      icon: RotateCw,
      label: `×${s.retry.max + 1}`,
      detail: `Retries up to ${s.retry.max} ${s.retry.max === 1 ? 'time' : 'times'}`,
    })
  }
  if (s.continueOnError) {
    traits.push({
      icon: ShieldCheck,
      label: 'may fail',
      detail: 'Later steps run even if it fails',
    })
  }
  return traits
}

/** Small chips on a step card for its condition, loop, retries and failure handling. */
export function StepTraits({ settings }: { settings: StepSettings }) {
  const traits = traitsOf(settings)
  if (traits.length === 0) return null
  return (
    <span className="flex flex-wrap gap-1">
      {traits.map(({ icon: Icon, label, detail }) => (
        <Tooltip key={label} label={detail} side="top">
          <span className="inline-flex items-center gap-1 rounded bg-hover px-1.5 py-px font-mono text-[10px] text-fg-muted">
            <Icon className="size-2.5" strokeWidth={2} />
            {label}
          </span>
        </Tooltip>
      ))}
    </span>
  )
}
