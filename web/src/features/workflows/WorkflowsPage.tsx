import { Plus } from 'lucide-react'

import { PageHeader } from '@/components/layout/PageHeader'
import { Button } from '@/components/ui/Button'
import { Tooltip } from '@/components/ui/Tooltip'

import { FlowPreview } from './FlowPreview'

export function WorkflowsPage() {
  const newWorkflow = (
    <Tooltip label="The editor arrives in phase 1" side="bottom">
      {/* Wrapper keeps the tooltip working on a disabled button. */}
      <span tabIndex={0}>
        <Button disabled>
          <Plus className="size-3.5" strokeWidth={2.25} />
          New workflow
        </Button>
      </span>
    </Tooltip>
  )

  return (
    <>
      <PageHeader title="Workflows" actions={newWorkflow} />
      <section className="flex flex-1 flex-col items-center justify-center gap-10 overflow-y-auto px-4 py-12 md:px-10">
        <FlowPreview />
        <div className="max-w-md text-center">
          <h2 className="text-base font-medium text-fg">No workflows yet</h2>
          <p className="mt-2 text-fg-muted">
            A workflow is a graph of steps. A step starts when everything it depends on has
            finished, so independent steps — like the release page and Jira tickets above — run in
            parallel.
          </p>
        </div>
      </section>
    </>
  )
}
