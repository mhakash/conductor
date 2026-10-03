import {
  Background,
  BackgroundVariant,
  BaseEdge,
  Controls,
  getBezierPath,
  Handle,
  Position,
  ReactFlow,
  type Edge,
  type EdgeProps,
  type Node,
  type NodeProps,
} from '@xyflow/react'
import { motion, useReducedMotion } from 'motion/react'
import { memo, useMemo } from 'react'

import { StepCard, type StepTone } from '@/components/flow/StepCard'
import { useBlockMap } from '@/features/blocks/api'
import { elapsed, formatDuration } from '@/lib/time'
import { useNow } from '@/lib/useNow'

import { statusLabel, type Run, type RunStatus, type StepRun } from './api'
import { StatusIcon } from './StatusIcon'

type RunNodeData = {
  block: string
  name: string
  step: StepRun
  selected: boolean
}
type RunNode = Node<RunNodeData, 'runStep'>

/**
 * How an edge looks depends on what happened at its ends:
 * lit — the dependency succeeded; flowing — and the dependent is running now;
 * dead — the dependency failed, was skipped or cancelled.
 */
type EdgeState = 'idle' | 'lit' | 'flowing' | 'dead'
type RunEdge = Edge<{ state: EdgeState }, 'runEdge'>

const toneFor: Record<RunStatus, StepTone> = {
  queued: 'default',
  running: 'active',
  succeeded: 'success',
  failed: 'danger',
  cancelled: 'muted',
  skipped: 'muted',
}

function edgeState(source: RunStatus, target: RunStatus): EdgeState {
  if (source === 'succeeded') return target === 'running' ? 'flowing' : 'lit'
  if (source === 'failed' || source === 'skipped' || source === 'cancelled') return 'dead'
  return 'idle'
}

const RunStepNode = memo(function RunStepNode({ id, data }: NodeProps<RunNode>) {
  const block = useBlockMap().get(data.block)
  const { step } = data
  const now = useNow(step.status === 'running')
  const took = elapsed(step.startedAt, step.finishedAt, now)

  return (
    <div className="cursor-pointer">
      {/* Edges need handles to attach to; these are anchors only, not interactive. */}
      <Handle type="target" position={Position.Left} isConnectable={false} className={anchor} />
      <StepCard
        icon={block?.icon ?? ''}
        block={data.block}
        id={id}
        name={data.name || block?.name || id}
        tone={data.selected ? 'selected' : toneFor[step.status]}
        badge={<StatusIcon status={step.status} className="size-3.5" />}
        footer={
          <span className="flex items-center justify-between gap-2 text-fg-subtle">
            <span className="truncate">{step.error || statusLabel(step.status)}</span>
            {took !== null && (
              <span className="shrink-0 font-mono tabular-nums">{formatDuration(took)}</span>
            )}
          </span>
        }
      />
      <Handle type="source" position={Position.Right} isConnectable={false} className={anchor} />
    </div>
  )
})

const anchor = '!pointer-events-none !size-1 !min-h-0 !min-w-0 !border-0 !bg-transparent'

const RunEdgeView = memo(function RunEdgeView(props: EdgeProps<RunEdge>) {
  const reduceMotion = useReducedMotion()
  const [path] = getBezierPath(props)
  const state = props.data?.state ?? 'idle'
  const lit = state === 'lit' || state === 'flowing'

  return (
    <>
      <BaseEdge
        path={path}
        style={{
          stroke: 'var(--color-line-strong)',
          strokeWidth: 1.5,
          strokeDasharray: state === 'dead' ? '4 4' : undefined,
          opacity: state === 'dead' ? 0.5 : 1,
        }}
      />
      {/* The brass trail draws itself the moment the dependency succeeds. */}
      <motion.path
        d={path}
        fill="none"
        stroke="var(--color-brass)"
        strokeWidth={1.5}
        strokeLinecap="round"
        initial={false}
        animate={{ pathLength: lit ? 1 : 0, opacity: lit ? (state === 'flowing' ? 1 : 0.55) : 0 }}
        transition={{ duration: reduceMotion ? 0 : 0.6, ease: [0.22, 1, 0.36, 1] }}
      />
      {state === 'flowing' && !reduceMotion && (
        <path
          d={path}
          fill="none"
          stroke="var(--color-brass-bright)"
          strokeWidth={2}
          strokeLinecap="round"
          className="animate-edge-flow"
          strokeDasharray="6 14"
        />
      )}
    </>
  )
})

const nodeTypes = { runStep: RunStepNode }
const edgeTypes = { runEdge: RunEdgeView }

type RunGraphProps = {
  run: Run
  selectedId: string | undefined
  onSelect: (stepId: string) => void
}

export function RunGraph({ run, selectedId, onSelect }: RunGraphProps) {
  const statusById = useMemo(() => new Map(run.steps.map((s) => [s.stepId, s])), [run.steps])

  const nodes = useMemo<RunNode[]>(
    () =>
      run.definition.steps.flatMap((s) => {
        const step = statusById.get(s.id)
        if (!step) return []
        return [
          {
            id: s.id,
            type: 'runStep' as const,
            position: s.position,
            data: { block: s.block, name: s.name, step, selected: s.id === selectedId },
            draggable: false,
            connectable: false,
          },
        ]
      }),
    [run.definition.steps, statusById, selectedId],
  )

  const edges = useMemo<RunEdge[]>(
    () =>
      run.definition.edges.map((e) => ({
        id: `${e.from}->${e.to}`,
        source: e.from,
        target: e.to,
        type: 'runEdge' as const,
        data: {
          state: edgeState(
            statusById.get(e.from)?.status ?? 'queued',
            statusById.get(e.to)?.status ?? 'queued',
          ),
        },
        selectable: false,
      })),
    [run.definition.edges, statusById],
  )

  return (
    <ReactFlow
      nodes={nodes}
      edges={edges}
      nodeTypes={nodeTypes}
      edgeTypes={edgeTypes}
      onNodeClick={(_, node) => onSelect(node.id)}
      colorMode="dark"
      fitView
      fitViewOptions={{ padding: 0.25, maxZoom: 1 }}
      minZoom={0.2}
      maxZoom={1.5}
      nodesDraggable={false}
      nodesConnectable={false}
      elementsSelectable={false}
      deleteKeyCode={null}
    >
      <Background variant={BackgroundVariant.Dots} gap={20} size={1.2} />
      <Controls showInteractive={false} position="bottom-left" />
    </ReactFlow>
  )
}
