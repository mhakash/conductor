import { Handle, Position, type NodeProps } from '@xyflow/react'
import { AlertTriangle } from 'lucide-react'
import { memo } from 'react'

import { StepCard } from '@/components/flow/StepCard'
import { missingFields, useBlockMap } from '@/features/blocks/api'
import { statusLabel } from '@/features/runs/api'
import { StatusIcon } from '@/features/runs/StatusIcon'

import type { StepNode } from './graph'
import { useStepRunStatus } from './runOverlay'
import { useEditor } from './store'

const handleClass =
  '!size-2.5 !rounded-full !border-[1.5px] !border-line-strong !bg-canvas transition-colors hover:!border-brass hover:!bg-brass'

function StepNodeViewImpl({ id, data, selected }: NodeProps<StepNode>) {
  const block = useBlockMap().get(data.block)
  const hasProblem = useEditor((s) => s.problems.some((p) => p.step === id))
  const missing = block ? missingFields(block, data.config) : []
  const lastStatus = useStepRunStatus(id)

  return (
    <>
      <Handle type="target" position={Position.Left} className={handleClass} />
      <StepCard
        icon={block?.icon ?? ''}
        block={data.block}
        id={id}
        name={data.name || block?.name || data.block}
        tone={
          selected
            ? 'selected'
            : hasProblem
              ? 'danger'
              : lastStatus === 'running'
                ? 'active'
                : 'default'
        }
        badge={
          lastStatus && (
            <span
              className="flex items-center gap-1.5"
              title={`Last run: ${statusLabel(lastStatus)}`}
            >
              {id}
              <StatusIcon status={lastStatus} className="size-3" />
            </span>
          )
        }
        footer={
          hasProblem ? (
            <span className="flex items-center gap-1 text-coral">
              <AlertTriangle className="size-3" /> Has problems
            </span>
          ) : missing.length > 0 ? (
            <span className="text-fg-subtle">
              Needs {missing.map((f) => sentenceCase(f.label)).join(', ')}
            </span>
          ) : null
        }
      />
      <Handle type="source" position={Position.Right} className={handleClass} />
    </>
  )
}

export const StepNodeView = memo(StepNodeViewImpl)

/** "Command" → "command", but acronyms like "URL" stay as they are. */
function sentenceCase(label: string) {
  return label === label.toUpperCase() ? label : label.toLowerCase()
}
