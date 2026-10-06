import { FileUp, Plus } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { useNavigate } from 'react-router'
import { toast } from 'sonner'

import { PageHeader } from '@/components/layout/PageHeader'
import { Button } from '@/components/ui/Button'

import { useCreateWorkflow, useWorkflows } from './api'
import { FlowPreview } from './FlowPreview'
import { ImportDialog } from './ImportDialog'
import { WorkflowList } from './WorkflowList'

export function WorkflowsPage() {
  const workflows = useWorkflows()
  const create = useCreateWorkflow()
  const navigate = useNavigate()
  const [importing, setImporting] = useState(false)

  const createWorkflow = () =>
    create.mutate(
      { name: 'Untitled workflow', description: '' },
      {
        onSuccess: (wf) => navigate(`/workflows/${wf.id}`),
        onError: (err) => toast.error(`Couldn't create the workflow: ${err.message}`),
      },
    )

  const actions = (
    <>
      <Button variant="secondary" onClick={() => setImporting(true)}>
        <FileUp className="size-3.5" />
        Import
      </Button>
      <Button onClick={createWorkflow} disabled={create.isPending}>
        <Plus className="size-3.5" strokeWidth={2.25} />
        New workflow
      </Button>
    </>
  )

  return (
    <>
      <PageHeader title="Workflows" actions={actions} />
      <ImportDialog open={importing} onOpenChange={setImporting} />
      <div className="flex flex-1 flex-col overflow-y-auto">
        {workflows.isPending ? null : workflows.isError ? (
          <Message title="Couldn't load workflows">
            {workflows.error.message}. Check that the engine is running, then{' '}
            <button className="text-brass hover:underline" onClick={() => workflows.refetch()}>
              try again
            </button>
            .
          </Message>
        ) : workflows.data.length === 0 ? (
          <EmptyState />
        ) : (
          <WorkflowList workflows={workflows.data} />
        )}
      </div>
    </>
  )
}

function EmptyState() {
  return (
    <section className="flex flex-1 flex-col items-center justify-center gap-10 px-4 py-12 md:px-10">
      <FlowPreview />
      <div className="max-w-md text-center">
        <h2 className="text-base font-medium text-fg">No workflows yet</h2>
        <p className="mt-2 text-fg-muted">
          A workflow is a graph of steps. A step starts when everything it depends on has finished,
          so independent steps — like the release page and Jira tickets above — run in parallel.
        </p>
      </div>
    </section>
  )
}

function Message({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="flex flex-1 items-center justify-center px-4">
      <div className="max-w-sm text-center">
        <h2 className="text-base font-medium text-fg">{title}</h2>
        <p className="mt-2 text-fg-muted">{children}</p>
      </div>
    </section>
  )
}
