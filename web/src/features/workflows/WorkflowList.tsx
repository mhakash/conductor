import { Download, MoreHorizontal, Trash2 } from 'lucide-react'
import { motion } from 'motion/react'
import { useState } from 'react'
import { Link } from 'react-router'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/ui/ConfirmDialog'
import { Menu, MenuContent, MenuItem, MenuTrigger } from '@/components/ui/Menu'
import { timeAgo } from '@/lib/time'

import { exportURL, useDeleteWorkflow, type WorkflowSummary } from './api'

export function WorkflowList({ workflows }: { workflows: WorkflowSummary[] }) {
  const [toDelete, setToDelete] = useState<WorkflowSummary | null>(null)
  const del = useDeleteWorkflow()

  const confirmDelete = () => {
    if (!toDelete) return
    del.mutate(toDelete.id, {
      onSuccess: () => {
        toast(`Deleted “${toDelete.name}”`)
        setToDelete(null)
      },
      onError: (err) => toast.error(`Couldn't delete the workflow: ${err.message}`),
    })
  }

  return (
    <div className="mx-auto w-full max-w-4xl px-4 py-6 md:px-6">
      <ul className="divide-y divide-line overflow-hidden rounded-xl border border-line bg-surface">
        {workflows.map((wf, i) => (
          <motion.li
            key={wf.id}
            initial={{ opacity: 0, y: 4 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: Math.min(i, 10) * 0.025, duration: 0.25 }}
            className="group relative flex items-center gap-4 px-4 py-3 transition-colors hover:bg-raised"
          >
            <div className="min-w-0 flex-1">
              <Link
                to={`/workflows/${wf.id}`}
                className="font-medium text-fg after:absolute after:inset-0 focus-visible:outline-none focus-visible:after:rounded-[inherit] focus-visible:after:outline-2 focus-visible:after:-outline-offset-2 focus-visible:after:outline-brass"
              >
                {wf.name}
              </Link>
              {wf.description && <p className="truncate text-fg-muted">{wf.description}</p>}
            </div>
            <span className="hidden font-mono text-xs text-fg-subtle sm:inline">
              {wf.stepCount} {wf.stepCount === 1 ? 'step' : 'steps'}
            </span>
            <span
              className="shrink-0 text-xs whitespace-nowrap text-fg-subtle"
              title={wf.updatedAt}
            >
              Edited {timeAgo(wf.updatedAt)}
            </span>
            <Menu>
              <MenuTrigger
                aria-label={`Actions for ${wf.name}`}
                className="relative z-10 flex size-7 items-center justify-center rounded-md text-fg-subtle hover:bg-hover hover:text-fg data-[state=open]:bg-hover"
              >
                <MoreHorizontal className="size-4" />
              </MenuTrigger>
              <MenuContent>
                <MenuItem
                  icon={<Download className="size-3.5" />}
                  onSelect={() => {
                    // The server names the file (Content-Disposition).
                    const a = document.createElement('a')
                    a.href = exportURL(wf.id)
                    a.download = ''
                    a.click()
                  }}
                >
                  Export as YAML
                </MenuItem>
                <MenuItem
                  destructive
                  icon={<Trash2 className="size-3.5" />}
                  onSelect={() => setToDelete(wf)}
                >
                  Delete
                </MenuItem>
              </MenuContent>
            </Menu>
          </motion.li>
        ))}
      </ul>

      <ConfirmDialog
        open={toDelete !== null}
        onOpenChange={(open) => !open && setToDelete(null)}
        title="Delete workflow?"
        description={<>“{toDelete?.name}” and its steps will be removed. This can't be undone.</>}
        confirmLabel="Delete"
        destructive
        pending={del.isPending}
        onConfirm={confirmDelete}
      />
    </div>
  )
}
