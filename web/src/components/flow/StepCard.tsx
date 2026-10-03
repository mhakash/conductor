import { motion } from 'motion/react'
import type { ReactNode } from 'react'

import { BlockIcon } from '@/features/blocks/BlockIcon'
import { cn } from '@/lib/cn'

/** Border treatment; `active` adds the brass pulse used for running steps. */
export type StepTone = 'default' | 'selected' | 'active' | 'success' | 'danger' | 'muted'

const tones: Record<StepTone, string> = {
  default: 'border-line hover:border-line-strong',
  selected:
    'border-brass shadow-[0_0_0_3px_color-mix(in_oklab,var(--color-brass)_18%,transparent)]',
  active: 'border-brass',
  success: 'border-sage/45',
  danger: 'border-coral/70',
  muted: 'border-dashed border-line-strong opacity-60',
}

type StepCardProps = {
  icon: string
  block: string
  id: string
  name: string
  tone?: StepTone
  /** Shown at the top right in place of the step id, e.g. a status icon. */
  badge?: ReactNode
  footer?: ReactNode
}

/** The visual body of a step on any canvas: editor and run graph alike. */
export function StepCard({
  icon,
  block,
  id,
  name,
  tone = 'default',
  badge,
  footer,
}: StepCardProps) {
  return (
    <motion.div
      initial={{ opacity: 0, scale: 0.94 }}
      animate={{ opacity: 1, scale: 1 }}
      transition={{ type: 'spring', stiffness: 520, damping: 34 }}
      className={cn(
        'relative w-60 rounded-xl border bg-raised shadow-lg shadow-black/30 transition-[border-color,box-shadow,opacity] duration-300',
        tones[tone],
      )}
    >
      {tone === 'active' && (
        <motion.span
          aria-hidden
          className="pointer-events-none absolute -inset-px rounded-xl border border-brass"
          initial={{ opacity: 0.7, scale: 1 }}
          animate={{ opacity: 0, scale: 1.06 }}
          transition={{ duration: 1.2, ease: 'easeOut', repeat: Infinity }}
        />
      )}

      <div className="flex items-center gap-2 border-b border-line px-3 py-2">
        <span className="flex size-5 items-center justify-center rounded-md bg-hover text-fg-muted">
          <BlockIcon name={icon} className="size-3" />
        </span>
        <span className="truncate font-mono text-[11px] text-fg-subtle">{block}</span>
        <span className="ml-auto flex min-w-0 items-center truncate font-mono text-[11px] text-fg-subtle">
          {badge ?? id}
        </span>
      </div>

      <div className="px-3 py-2.5">
        <p className="truncate font-medium text-fg">{name}</p>
        {footer && <div className="mt-0.5 text-xs">{footer}</div>}
      </div>
    </motion.div>
  )
}
