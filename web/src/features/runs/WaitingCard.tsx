import { Hand, TextCursorInput } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { toast } from 'sonner'

import { Button } from '@/components/ui/Button'
import { Textarea } from '@/components/ui/Input'
import { CALL_BLOCK, INPUT_BLOCK, type FormConfig } from '@/features/blocks/api'
import { ApiError } from '@/lib/api'
import { timeAgo } from '@/lib/time'

import { useDecide, useRespond, type StepRun } from './api'
import { InputFields } from './InputFields'
import { filledValues, initialValues, type InputValues } from './inputValues'

/**
 * Where you answer a step that waits for you: approve or reject an approval
 * step, fill in an input step. Nothing for other steps, or once answered.
 */
export function WaitingCard({
  runId,
  step,
  block,
}: {
  runId: string
  step: StepRun
  block: string
}) {
  if (step.status !== 'waiting' || block === CALL_BLOCK) return null
  // The config after expressions were evaluated, as recorded.
  const config = (step.config ?? {}) as Record<string, unknown>
  if (block === INPUT_BLOCK) {
    return (
      <InputCard
        key={step.stepId}
        runId={runId}
        stepId={step.stepId}
        form={config as FormConfig}
        since={step.startedAt}
      />
    )
  }
  return (
    <ApprovalCard
      key={step.stepId}
      runId={runId}
      stepId={step.stepId}
      message={(config.message as string | undefined) ?? ''}
      since={step.startedAt}
    />
  )
}

function ApprovalCard({
  runId,
  stepId,
  message,
  since,
}: {
  runId: string
  stepId: string
  message: string
  since: string | null
}) {
  const [comment, setComment] = useState('')
  const decide = useDecide(runId)

  const submit = (approve: boolean) =>
    decide.mutate(
      { stepId, approve, comment: comment.trim() },
      {
        onSuccess: () => toast(approve ? 'Approved' : 'Rejected'),
        onError: (err) => toast.error(`Couldn't send your decision: ${err.message}`),
      },
    )

  return (
    <section
      aria-label="Approval"
      className="flex animate-pop-in flex-col gap-3 rounded-xl border border-brass/40 bg-brass/5 p-4"
    >
      <p className="flex items-center gap-2 text-[11px] font-medium tracking-wide text-brass uppercase">
        <Hand className="size-3.5" strokeWidth={2} />
        Waiting for your approval
        {since && (
          <span className="font-normal tracking-normal text-fg-subtle normal-case">
            · since {timeAgo(since)}
          </span>
        )}
      </p>
      <p className="text-[15px] whitespace-pre-wrap text-fg">
        {message || 'Approve to let the run continue.'}
      </p>
      <Textarea
        aria-label="Comment"
        placeholder="Comment (optional) — later steps can read it"
        value={comment}
        rows={2}
        onChange={(e) => setComment(e.target.value)}
      />
      <div className="flex flex-wrap items-center gap-2">
        <Button onClick={() => submit(true)} disabled={decide.isPending}>
          Approve
        </Button>
        <Button variant="danger" onClick={() => submit(false)} disabled={decide.isPending}>
          Reject
        </Button>
        <span className="text-xs text-fg-subtle">Rejecting fails this step.</span>
      </div>
    </section>
  )
}

function InputCard({
  runId,
  stepId,
  form,
  since,
}: {
  runId: string
  stepId: string
  form: FormConfig
  since: string | null
}) {
  const fields = form.fields ?? []
  const [values, setValues] = useState<InputValues>(() => initialValues(fields))
  const [errors, setErrors] = useState<string[]>([])
  const respond = useRespond(runId)

  const submit = (e: FormEvent) => {
    e.preventDefault()
    respond.mutate(
      { stepId, values: filledValues(values) },
      {
        onSuccess: () => toast('Sent'),
        onError: (err) => {
          const problems = err instanceof ApiError ? err.problems.map((p) => p.message) : []
          setErrors(problems)
          if (problems.length === 0) toast.error(`Couldn't send your answers: ${err.message}`)
        },
      },
    )
  }

  return (
    <form
      aria-label="Input"
      onSubmit={submit}
      className="flex animate-pop-in flex-col gap-4 rounded-xl border border-brass/40 bg-brass/5 p-4"
    >
      <p className="flex items-center gap-2 text-[11px] font-medium tracking-wide text-brass uppercase">
        <TextCursorInput className="size-3.5" strokeWidth={2} />
        Waiting for your input
        {since && (
          <span className="font-normal tracking-normal text-fg-subtle normal-case">
            · since {timeAgo(since)}
          </span>
        )}
      </p>
      {form.message && <p className="text-[15px] whitespace-pre-wrap text-fg">{form.message}</p>}
      <div className="flex max-w-md flex-col gap-4">
        <InputFields
          inputs={fields}
          values={values}
          onChange={(name, v) => setValues((prev) => ({ ...prev, [name]: v }))}
          idPrefix={`step-input-${stepId}`}
        />
      </div>
      {errors.length > 0 && (
        <ul className="rounded-lg border border-coral/30 bg-coral/5 px-3 py-2 text-xs text-coral">
          {errors.map((e) => (
            <li key={e}>{e}</li>
          ))}
        </ul>
      )}
      <div>
        <Button type="submit" disabled={respond.isPending}>
          Submit
        </Button>
      </div>
    </form>
  )
}
