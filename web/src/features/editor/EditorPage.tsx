import { ReactFlowProvider, useReactFlow } from '@xyflow/react'
import { ChevronLeft, Network, Play, Plus } from 'lucide-react'
import { useEffect, useEffectEvent } from 'react'
import { Link, useBlocker, useNavigate, useParams } from 'react-router'
import { toast } from 'sonner'

import { Button } from '@/components/ui/Button'
import { ConfirmDialog } from '@/components/ui/ConfirmDialog'
import { Menu, MenuContent, MenuItem, MenuTrigger } from '@/components/ui/Menu'
import { Tooltip } from '@/components/ui/Tooltip'
import { BlockIcon } from '@/features/blocks/BlockIcon'
import { useBlockMap, useBlocks, type Block } from '@/features/blocks/api'
import { useRuns, useStartRun } from '@/features/runs/api'
import { StatusIcon } from '@/features/runs/StatusIcon'
import { useUpdateWorkflow, useWorkflow, type Workflow } from '@/features/workflows/api'
import { ApiError } from '@/lib/api'
import { cn } from '@/lib/cn'
import { timeAgo } from '@/lib/time'

import { Canvas } from './Canvas'
import { findFreeSpot, NODE_SIZE, toDefinition } from './graph'
import { Inspector } from './Inspector'
import { animatePositions, layoutGraph } from './layout'
import { Palette } from './Palette'
import { useEditor } from './store'

export function EditorPage() {
  const { id = '' } = useParams()
  const workflow = useWorkflow(id)
  const blocks = useBlocks()

  if (workflow.isError || blocks.isError) {
    const err = workflow.error ?? blocks.error
    const notFound = err instanceof ApiError && err.status === 404
    return (
      <div className="flex flex-1 items-center justify-center px-4">
        <div className="max-w-sm text-center">
          <h2 className="text-base font-medium text-fg">
            {notFound ? 'This workflow no longer exists' : "Couldn't open the workflow"}
          </h2>
          <p className="mt-2 text-fg-muted">
            {notFound ? 'It may have been deleted.' : err?.message}{' '}
            <Link to="/workflows" className="text-brass hover:underline">
              Back to workflows
            </Link>
          </p>
        </div>
      </div>
    )
  }
  if (!workflow.data || !blocks.data) return null

  return (
    <ReactFlowProvider>
      <Editor key={workflow.data.id} workflow={workflow.data} blocks={blocks.data} />
    </ReactFlowProvider>
  )
}

function Editor({ workflow, blocks }: { workflow: Workflow; blocks: Block[] }) {
  const blockMap = useBlockMap()
  const load = useEditor((s) => s.load)
  const loaded = useEditor((s) => s.workflowId === workflow.id)
  const dirty = useEditor((s) => s.dirty)
  const addStep = useEditor((s) => s.addStep)
  const update = useUpdateWorkflow(workflow.id)
  const startRun = useStartRun()
  const navigate = useNavigate()
  const { screenToFlowPosition, flowToScreenPosition, setCenter, getZoom, fitView } = useReactFlow()

  // Load once per workflow; later refetches must not clobber the draft.
  useEffect(() => {
    load(workflow)
  }, [workflow.id]) // eslint-disable-line react-hooks/exhaustive-deps

  const showProblems = (prefix: string, err: Error) => {
    const problems = err instanceof ApiError ? err.problems : []
    useEditor.getState().setProblems(problems)
    toast.error(
      problems.length
        ? `${prefix}: ${problems.length === 1 ? problems[0].message : `${problems.length} problems — see the highlighted steps`}`
        : `${prefix}: ${err.message}`,
    )
  }

  /** Saves the draft; resolves to false (and shows why) when it fails. */
  const persist = async () => {
    const s = useEditor.getState()
    try {
      await update.mutateAsync({
        name: s.name,
        description: s.description,
        definition: toDefinition(s.nodes, s.edges),
      })
      useEditor.getState().markSaved()
      return true
    } catch (err) {
      showProblems('Not saved', err as Error)
      return false
    }
  }

  const save = async () => {
    if (await persist()) toast.success('Saved')
  }

  // Runs execute the saved workflow, so unsaved edits are saved first.
  const runWorkflow = async () => {
    if (useEditor.getState().dirty && !(await persist())) return
    try {
      const run = await startRun.mutateAsync(workflow.id)
      navigate(`/runs/${run.id}`)
    } catch (err) {
      showProblems("Can't run yet", err as Error)
    }
  }

  const onKey = useEffectEvent((e: KeyboardEvent) => {
    if ((e.metaKey || e.ctrlKey) && e.key === 's') {
      e.preventDefault()
      void save()
    }
  })
  useEffect(() => {
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  // Clicking a block appends it after the selected step, so a sequence can be
  // built by clicking alone. With nothing selected it lands mid-canvas.
  const addFromPalette = (block: Block) => {
    const { nodes } = useEditor.getState()
    const selected = nodes.filter((n) => n.selected)
    const anchor = selected.length === 1 ? selected[0] : null
    const rect = canvasRect()

    let desired
    if (anchor) {
      const width = anchor.measured?.width ?? NODE_SIZE.width
      desired = { x: anchor.position.x + width + 80, y: anchor.position.y }
    } else {
      const c = rect
        ? screenToFlowPosition({ x: rect.left + rect.width / 2, y: rect.top + rect.height / 2 })
        : { x: 0, y: 0 }
      desired = { x: c.x - NODE_SIZE.width / 2, y: c.y - NODE_SIZE.height / 2 }
    }
    const position = findFreeSpot(nodes, desired)
    addStep(block, position, anchor?.id)

    // Keep the new step in view without changing the zoom.
    if (rect) {
      const tl = flowToScreenPosition(position)
      const br = flowToScreenPosition({
        x: position.x + NODE_SIZE.width,
        y: position.y + NODE_SIZE.height,
      })
      const visible =
        tl.x >= rect.left && tl.y >= rect.top && br.x <= rect.right && br.y <= rect.bottom
      if (!visible) {
        void setCenter(position.x + NODE_SIZE.width / 2, position.y + NODE_SIZE.height / 2, {
          zoom: getZoom(),
          duration: 300,
        })
      }
    }
  }

  const autoLayout = async () => {
    const { nodes, edges, setPositions } = useEditor.getState()
    if (nodes.length === 0) return
    const target = await layoutGraph(nodes, edges)
    await animatePositions(nodes, target, setPositions)
    void fitView({ padding: 0.3, maxZoom: 1, duration: 400 })
  }

  if (!loaded) return null

  return (
    <div className="flex h-full flex-col">
      <header className="flex h-14 shrink-0 items-center gap-2 border-b border-line px-3 md:px-4">
        <Tooltip label="All workflows" side="bottom">
          <Link
            to="/workflows"
            aria-label="All workflows"
            className="flex size-8 items-center justify-center rounded-md text-fg-muted hover:bg-hover hover:text-fg"
          >
            <ChevronLeft className="size-4" />
          </Link>
        </Tooltip>
        <NameInput />
        <SaveState dirty={dirty} saving={update.isPending} />
        <LastRun workflowId={workflow.id} />

        <div className="ml-auto flex items-center gap-1.5">
          <Menu>
            <MenuTrigger asChild>
              <Button variant="secondary" className="md:hidden" aria-label="Add step">
                <Plus className="size-3.5" />
              </Button>
            </MenuTrigger>
            <MenuContent>
              {blocks.map((b) => (
                <MenuItem
                  key={b.type}
                  icon={<BlockIcon name={b.icon} className="size-3.5" />}
                  onSelect={() => addFromPalette(b)}
                >
                  {b.name}
                </MenuItem>
              ))}
            </MenuContent>
          </Menu>
          <Button variant="ghost" onClick={autoLayout}>
            <Network className="size-3.5" />
            <span className="hidden sm:inline">Tidy up</span>
          </Button>
          <Button variant="secondary" onClick={save} disabled={update.isPending}>
            Save
            <kbd className="hidden font-sans text-[11px] text-fg-subtle sm:inline">⌘S</kbd>
          </Button>
          <Button onClick={runWorkflow} disabled={update.isPending || startRun.isPending}>
            <Play className="size-3" fill="currentColor" />
            Run
          </Button>
        </div>
      </header>

      <div className="flex min-h-0 flex-1">
        <Palette blocks={blocks} onAdd={addFromPalette} />
        <Canvas blocks={blockMap} />
        <Inspector blocks={blockMap} />
      </div>

      <UnsavedChangesGuard dirty={dirty} />
    </div>
  )
}

function canvasRect() {
  return document.querySelector('.react-flow')?.getBoundingClientRect()
}

function NameInput() {
  const name = useEditor((s) => s.name)
  const setName = useEditor((s) => s.setName)
  return (
    <input
      aria-label="Workflow name"
      value={name}
      onChange={(e) => setName(e.target.value)}
      onKeyDown={(e) => e.key === 'Enter' && e.currentTarget.blur()}
      placeholder="Untitled workflow"
      size={Math.max(name.length, 12)}
      className="h-8 max-w-[40vw] min-w-0 rounded-md border border-transparent bg-transparent px-2 text-sm font-medium text-fg transition-colors placeholder:text-fg-subtle hover:border-line focus:border-brass/70 focus:outline-none"
    />
  )
}

function SaveState({ dirty, saving }: { dirty: boolean; saving: boolean }) {
  const label = saving ? 'Saving…' : dirty ? 'Unsaved changes' : 'Saved'
  return (
    <span className="hidden items-center gap-1.5 text-xs text-fg-subtle sm:flex" aria-live="polite">
      <span
        className={cn(
          'size-1.5 rounded-full transition-colors',
          dirty || saving ? 'bg-brass' : 'bg-line-strong',
        )}
      />
      {label}
    </span>
  )
}

/** Link to the workflow's most recent run, so its result is one click away. */
function LastRun({ workflowId }: { workflowId: string }) {
  const last = useRuns(workflowId).data?.[0]
  if (!last) return null
  return (
    <Link
      to={`/runs/${last.id}`}
      className="hidden items-center gap-1.5 rounded-md px-2 py-1 text-xs text-fg-subtle hover:bg-hover hover:text-fg md:flex"
    >
      <StatusIcon status={last.status} className="size-3.5" />
      Last run {timeAgo(last.createdAt)}
    </Link>
  )
}

/** Asks before leaving with unsaved changes, both in-app and on tab close. */
function UnsavedChangesGuard({ dirty }: { dirty: boolean }) {
  // Read the store at navigation time: "Run" saves and navigates in one go,
  // before React has re-rendered with dirty = false.
  const blocker = useBlocker(() => useEditor.getState().dirty)

  useEffect(() => {
    if (!dirty) return
    const onBeforeUnload = (e: BeforeUnloadEvent) => e.preventDefault()
    window.addEventListener('beforeunload', onBeforeUnload)
    return () => window.removeEventListener('beforeunload', onBeforeUnload)
  }, [dirty])

  return (
    <ConfirmDialog
      open={blocker.state === 'blocked'}
      onOpenChange={(open) => !open && blocker.state === 'blocked' && blocker.reset()}
      title="Discard unsaved changes?"
      description="You have changes to this workflow that haven't been saved."
      confirmLabel="Discard changes"
      destructive
      onConfirm={() => blocker.state === 'blocked' && blocker.proceed()}
    />
  )
}
