import { PageHeader } from '@/components/layout/PageHeader'

export function RunsPage() {
  return (
    <>
      <PageHeader title="Runs" />
      <section className="flex flex-1 items-center justify-center px-4">
        <div className="max-w-sm text-center">
          <h2 className="text-base font-medium text-fg">No runs yet</h2>
          <p className="mt-2 text-fg-muted">
            Every time a workflow runs, it shows up here with its logs, outputs and step timings.
          </p>
        </div>
      </section>
    </>
  )
}
