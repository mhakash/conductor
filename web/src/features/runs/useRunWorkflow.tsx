import { useState } from 'react'
import { useNavigate } from 'react-router'
import { toast } from 'sonner'

import type { WorkflowFields, WorkflowInput } from '@/features/workflows/api'
import { ApiError, type Problem } from '@/lib/api'

import { useStartRun, type Run } from './api'
import { StartRunDialog } from './StartRunDialog'

type Target = {
  /** The saved workflow to run; omitted when running `draft`. */
  workflowId?: string
  /** A workflow to run without saving it. */
  draft?: WorkflowFields
  workflowName: string
  inputs: WorkflowInput[]
  initial?: Record<string, unknown>
}

/**
 * Starts a run and opens it, or hands it to `onStarted` to show in place.
 * Workflows with inputs ask for them first. Problems tied to a step go to
 * `onStepProblems` (the editor highlights them); everything else is shown in
 * the dialog or a toast.
 */
export function useRunWorkflow(
  onStepProblems?: (problems: Problem[]) => void,
  onStarted?: (run: Run) => void,
) {
  const start = useStartRun()
  const navigate = useNavigate()
  const [target, setTarget] = useState<Target | null>(null)
  const [errors, setErrors] = useState<string[]>([])

  const submit = async (t: Target, inputs: Record<string, unknown>) => {
    try {
      const run = await start.mutateAsync({ workflowId: t.workflowId, draft: t.draft, inputs })
      setTarget(null)
      if (onStarted) onStarted(run)
      else navigate(`/runs/${run.id}`)
    } catch (err) {
      const problems = err instanceof ApiError ? err.problems : []
      const stepProblems = problems.filter((p) => p.step)
      const other = problems.filter((p) => !p.step).map((p) => p.message)
      if (stepProblems.length && onStepProblems) {
        setTarget(null)
        onStepProblems(problems)
        toast.error(
          `Can't run yet: ${stepProblems.length === 1 ? stepProblems[0].message : `${stepProblems.length} problems — see the highlighted steps`}`,
        )
      } else if (t.inputs.length > 0) {
        setTarget(t)
        setErrors(other.length ? other : [(err as Error).message])
      } else {
        toast.error(`Can't run yet: ${problems[0]?.message ?? (err as Error).message}`)
      }
    }
  }

  const run = (t: Target) => {
    setErrors([])
    if (t.inputs.length === 0) void submit(t, {})
    else setTarget(t)
  }

  const dialog = (
    <StartRunDialog
      open={target !== null}
      onOpenChange={(open) => !open && setTarget(null)}
      workflowName={target?.workflowName ?? ''}
      inputs={target?.inputs ?? []}
      initial={target?.initial}
      pending={start.isPending}
      errors={errors}
      onSubmit={(values) => target && void submit(target, values)}
    />
  )

  return { run, dialog, pending: start.isPending }
}
