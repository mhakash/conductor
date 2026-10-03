import { motion, useReducedMotion } from 'motion/react'
import { useEffect, useState } from 'react'

/*
 * A looping, illustrative run of the "build preparation" workflow. Each step
 * belongs to a stage; steps in the same stage run in parallel.
 */

type Step = { id: string; block: string; label: string; x: number; y: number; stage: number }

const W = 128
const H = 44

const steps: Step[] = [
  { id: 'tag', block: 'git', label: 'Create tag', x: 8, y: 98, stage: 0 },
  { id: 'push', block: 'git', label: 'Push to GitLab', x: 172, y: 98, stage: 1 },
  { id: 'pipeline', block: 'gitlab', label: 'Pipeline status', x: 336, y: 98, stage: 2 },
  { id: 'confluence', block: 'confluence', label: 'Release page', x: 500, y: 38, stage: 3 },
  { id: 'jira', block: 'jira', label: 'Update tickets', x: 500, y: 158, stage: 3 },
  { id: 'email', block: 'email', label: 'Prepare email', x: 664, y: 98, stage: 4 },
]

const edges: [string, string][] = [
  ['tag', 'push'],
  ['push', 'pipeline'],
  ['pipeline', 'confluence'],
  ['pipeline', 'jira'],
  ['confluence', 'email'],
  ['jira', 'email'],
]

const byId = Object.fromEntries(steps.map((s) => [s.id, s]))
const STAGES = 5
const BEAT_MS = 1100
// Extra beats at the end of the loop so the finished run stays on screen.
const HOLD_BEATS = 3

function edgePath(from: Step, to: Step) {
  const x1 = from.x + W
  const y1 = from.y + H / 2
  const x2 = to.x
  const y2 = to.y + H / 2
  const mid = (x1 + x2) / 2
  return `M ${x1} ${y1} C ${mid} ${y1}, ${mid} ${y2}, ${x2} ${y2}`
}

type StepState = 'idle' | 'running' | 'done'

export function FlowPreview() {
  const reduceMotion = useReducedMotion()
  const [beat, setBeat] = useState(0)

  useEffect(() => {
    if (reduceMotion) return
    const id = setInterval(() => setBeat((b) => (b + 1) % (STAGES + HOLD_BEATS)), BEAT_MS)
    return () => clearInterval(id)
  }, [reduceMotion])

  const current = reduceMotion ? STAGES : beat
  const stateOf = (s: Step): StepState =>
    s.stage < current ? 'done' : s.stage === current ? 'running' : 'idle'

  return (
    <svg
      viewBox="0 0 800 240"
      className="w-full max-w-3xl"
      role="img"
      aria-label="Example workflow: create tag, push to GitLab, check pipeline status, then update the release page and Jira tickets in parallel, then prepare an email."
    >
      {edges.map(([a, b]) => {
        const from = byId[a]
        const to = byId[b]
        const d = edgePath(from, to)
        const lit = from.stage < current
        return (
          <g key={`${a}-${b}`}>
            <path d={d} fill="none" stroke="var(--color-line-strong)" strokeWidth={1.5} />
            <motion.path
              d={d}
              fill="none"
              stroke="var(--color-brass)"
              strokeWidth={1.5}
              strokeLinecap="round"
              initial={false}
              animate={{ pathLength: lit ? 1 : 0, opacity: lit ? 0.9 : 0 }}
              transition={{
                pathLength: { duration: lit ? 0.6 : 0, ease: [0.22, 1, 0.36, 1] },
                opacity: { duration: lit ? 0.2 : 0.4 },
              }}
            />
          </g>
        )
      })}

      {steps.map((s) => (
        <StepNode key={s.id} step={s} state={stateOf(s)} />
      ))}
    </svg>
  )
}

function StepNode({ step, state }: { step: Step; state: StepState }) {
  const stroke =
    state === 'running'
      ? 'var(--color-brass)'
      : state === 'done'
        ? 'var(--color-line-strong)'
        : 'var(--color-line)'

  return (
    <g transform={`translate(${step.x} ${step.y})`}>
      <motion.rect
        width={W}
        height={H}
        rx={8}
        fill="var(--color-raised)"
        initial={false}
        animate={{ stroke }}
        transition={{ duration: 0.3 }}
        strokeWidth={1}
      />
      {state === 'running' && (
        <motion.rect
          width={W}
          height={H}
          rx={8}
          fill="none"
          stroke="var(--color-brass)"
          initial={{ opacity: 0.6, scale: 1 }}
          animate={{ opacity: 0, scale: 1.12 }}
          transition={{ duration: 1, ease: 'easeOut', repeat: Infinity }}
          style={{ transformOrigin: `${W / 2}px ${H / 2}px` }}
        />
      )}
      <text
        x={12}
        y={17}
        className="fill-fg-subtle font-mono"
        style={{ fontSize: 9.5, letterSpacing: '0.02em' }}
      >
        {step.block}
      </text>
      <text x={12} y={33} className="fill-fg" style={{ fontSize: 12, fontWeight: 500 }}>
        {step.label}
      </text>
      <StatusGlyph state={state} />
    </g>
  )
}

function StatusGlyph({ state }: { state: StepState }) {
  const cx = W - 16
  const cy = 14
  if (state === 'done') {
    return (
      <motion.path
        d={`M ${cx - 3.5} ${cy} l 2.5 2.5 l 4.5 -5`}
        fill="none"
        stroke="var(--color-sage)"
        strokeWidth={1.6}
        strokeLinecap="round"
        strokeLinejoin="round"
        initial={{ pathLength: 0 }}
        animate={{ pathLength: 1 }}
        transition={{ duration: 0.3 }}
      />
    )
  }
  return (
    <circle
      cx={cx}
      cy={cy}
      r={3}
      fill={state === 'running' ? 'var(--color-brass)' : 'var(--color-line-strong)'}
    />
  )
}
