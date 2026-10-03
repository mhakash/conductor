import { Handle, Position, type NodeProps } from '@xyflow/react'
import { AlertTriangle } from 'lucide-react'
import { motion } from 'motion/react'
import { memo } from 'react'

import { BlockIcon } from '@/features/blocks/BlockIcon'
import { missingFields, useBlockMap } from '@/features/blocks/api'
import { cn } from '@/lib/cn'

import type { StepNode } from './graph'
import { useEditor } from './store'

const handleClass =
  '!size-2.5 !rounded-full !border-[1.5px] !border-line-strong !bg-canvas transition-colors hover:!border-brass hover:!bg-brass'

function StepNodeViewImpl({ id, data, selected }: NodeProps<StepNode>) {
  const block = useBlockMap().get(data.block)
  const hasProblem = useEditor((s) => s.problems.some((p) => p.step === id))
  const missing = block ? missingFields(block, data.config) : []

  return (
    <motion.div
      initial={{ opacity: 0, scale: 0.94 }}
      animate={{ opacity: 1, scale: 1 }}
      transition={{ type: 'spring', stiffness: 520, damping: 34 }}
      className={cn(
        'w-60 rounded-xl border bg-raised shadow-lg shadow-black/30 transition-[border-color,box-shadow]',
        selected
          ? 'border-brass shadow-[0_0_0_3px_color-mix(in_oklab,var(--color-brass)_18%,transparent)]'
          : hasProblem
            ? 'border-coral/70'
            : 'border-line hover:border-line-strong',
      )}
    >
      <Handle type="target" position={Position.Left} className={handleClass} />

      <div className="flex items-center gap-2 border-b border-line px-3 py-2">
        <span className="flex size-5 items-center justify-center rounded-md bg-hover text-fg-muted">
          <BlockIcon name={block?.icon ?? ''} className="size-3" />
        </span>
        <span className="truncate font-mono text-[11px] text-fg-subtle">{data.block}</span>
        <span className="ml-auto truncate font-mono text-[11px] text-fg-subtle">{id}</span>
      </div>

      <div className="px-3 py-2.5">
        <p className="truncate font-medium text-fg">{data.name || block?.name || data.block}</p>
        {hasProblem ? (
          <p className="mt-0.5 flex items-center gap-1 text-xs text-coral">
            <AlertTriangle className="size-3" /> Has problems
          </p>
        ) : missing.length > 0 ? (
          <p className="mt-0.5 text-xs text-fg-subtle">
            Needs {missing.map((f) => sentenceCase(f.label)).join(', ')}
          </p>
        ) : null}
      </div>

      <Handle type="source" position={Position.Right} className={handleClass} />
    </motion.div>
  )
}

export const StepNodeView = memo(StepNodeViewImpl)

/** "Command" → "command", but acronyms like "URL" stay as they are. */
function sentenceCase(label: string) {
  return label === label.toUpperCase() ? label : label.toLowerCase()
}
