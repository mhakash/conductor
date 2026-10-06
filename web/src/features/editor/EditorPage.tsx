import { ReactFlowProvider, useReactFlow } from '@xyflow/react'
import {
  ChevronLeft,
  FileCode,
  FlaskConical,
  Network,
  Play,
  Plus,
  Save,
  Trash2,
} from 'lucide-react'
import * as Dialog from '@radix-ui/react-dialog'
import { useQueryClient } from '@tanstack/react-query'
import { useEffect, useEffectEvent, useMemo, useState } from 'react'
import { Link, useBlocker, useNavigate, useParams } from 'react-router'
import { toast } from 'sonner'

import { Button } from '@/components/ui/Button'
import { ConfirmDialog } from '@/components/ui/ConfirmDialog'
import { Menu, MenuContent, MenuItem, MenuTrigger } from '@/components/ui/Menu'
import { Tooltip } from '@/components/ui/Tooltip'
import { BlockIcon } from '@/features/blocks/BlockIcon'
import { useBlockMap, useBlocks } from '@/features/blocks/api'
import { isActive, useRun, type Run } from '@/features/runs/api'
import { useRunWorkflow } from '@/features/runs/useRunWorkflow'
import { StatusIcon } from '@/features/runs/StatusIcon'
import {
  fetchWorkflow,
  useCreateWorkflow,
  useUpdateWorkflow,
  useWorkflow,
  type Workflow,
} from '@/features/workflows/api'
import { ApiError } from '@/lib/api'
import { cn } from '@/lib/cn'
import { timeAgo } from '@/lib/time'

import { Canvas } from './Canvas'
import { findFreeSpot, NODE_SIZE, toDefinition } from './graph'
import { Inspector } from './Inspector'
import { useLibrary, type LibraryItem } from './library'
import { animatePositions, layoutGraph } from './layout'
import { Palette } from './Palette'
import { RunOverlayContext, useLatestRun, useRunStatuses } from './runOverlay'
import { RunPanel } from './RunPanel'
import { clearScratch, saveScratch, saveScratchRun } from './scratch'
import { useEditor } from './store'
import { YamlDialog } from './YamlDialog'

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
      <Editor key={workflow.data.id} workflow={workflow.data} />
    </ReactFlowProvider>
  )
}

type EditorProps = {
  workflow: Workflow
  /**
   * Set for the scratch workflow, which is never saved: it is kept in the
   * browser and run in place. runId is its last test run.
   */
  scratch?: { runId?: string; onCleared: () => void }
}

export function Editor({ workflow, scratch }: EditorProps) {
  const blockMap = useBlockMap()
  const library = useLibrary(workflow.id)
  const libraryMap = useMemo(() => new Map(library.map((i) => [i.key, i])), [library])
  const load = useEditor((s) => s.load)
  const loaded = useEditor((s) => s.workflowId === workflow.id)
  const dirty = useEditor((s) => s.dirty)
  const addStep = useEditor((s) => s.addStep)
  const update = useUpdateWorkflow(workflow.id)
  const queryClient = useQueryClient()
  const runner = useRunWorkflow((problems) => useEditor.getState().setProblems(problems))
  const latest = useLatestRun(workflow.id)

  // A test run runs the draft as it is, unsaved, and shows it here.
  const [testRunId, setTestRunId] = useState(scratch?.runId)
  const showTestRun = (id: string | undefined) => {
    setTestRunId(id)
    if (scratch) saveScratchRun(id)
  }
  const tester = useRunWorkflow(
    (problems) => useEditor.getState().setProblems(problems),
    (run) => showTestRun(run.id),
  )
  const testRunStatuses = useRunStatuses(useRun(testRunId).data)
  const testRun = () => {
    const s = useEditor.getState()
    s.setProblems([])
    tester.run({
      workflowId: scratch ? undefined : workflow.id,
      draft: {
        name: s.name,
        description: s.description,
        definition: toDefinition(s.nodes, s.edges, s.inputs, s.outputs),
      },
      workflowName: s.name,
      inputs: s.inputs,
    })
  }
  const { screenToFlowPosition, flowToScreenPosition, setCenter, getZoom, fitView } = useReactFlow()

  // Load once per workflow; later refetches must not clobber the draft.
  useEffect(() => {
    load(workflow)
  }, [workflow.id]) // eslint-disable-line react-hooks/exhaustive-deps

  // The scratch workflow is kept in the browser as it is edited.
  const scratchMode = !!scratch
  useEffect(() => {
    if (!scratchMode) return
    let timer: ReturnType<typeof setTimeout> | undefined
    const unsubscribe = useEditor.subscribe((s) => {
      clearTimeout(timer)
      timer = setTimeout(() => {
        saveScratch({
          name: s.name,
          description: s.description,
          definition: toDefinition(s.nodes, s.edges, s.inputs, s.outputs),
        })
      }, 300)
    })
    return () => {
      clearTimeout(timer)
      unsubscribe()
    }
  }, [scratchMode])

  const showProblems = (prefix: string, err: Error) => {
    const problems = err instanceof ApiError ? err.problems : []
    useEditor.getState().setProblems(problems)
    toast.error(
      problems.length
        ? `${prefix}: ${problems.length === 1 ? problems[0].message : `${problems.length} problems — see the highlighted steps`}`
        : `${prefix}: ${err.message}`,
    )
  }

  const [conflict, setConflict] = useState(false)
  const [yamlOpen, setYamlOpen] = useState(false)

  /**
   * Saves the draft; resolves to false (and shows why) when it fails. Unless
   * overwriting, the save is refused if the workflow changed elsewhere.
   */
  const persist = async (overwrite = false) => {
    const s = useEditor.getState()
    try {
      const saved = await update.mutateAsync({
        name: s.name,
        description: s.description,
        definition: toDefinition(s.nodes, s.edges, s.inputs, s.outputs),
        updatedAt: overwrite ? undefined : (s.version ?? undefined),
      })
      useEditor.getState().markSaved(saved.updatedAt)
      return true
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) setConflict(true)
      else showProblems('Not saved', err as Error)
      return false
    }
  }

  const loadLatest = async () => {
    const latest = await fetchWorkflow(workflow.id)
    queryClient.setQueryData(['workflows', workflow.id], latest)
    load(latest)
    setConflict(false)
    toast('Loaded the latest version')
  }

  const save = async () => {
    if (await persist()) toast.success('Saved')
  }

  // Turns the scratch workflow into a saved one and opens it.
  const create = useCreateWorkflow()
  const navigate = useNavigate()
  const saveScratchAsWorkflow = async () => {
    const s = useEditor.getState()
    try {
      const wf = await create.mutateAsync({
        name: s.name,
        description: s.description,
        definition: toDefinition(s.nodes, s.edges, s.inputs, s.outputs),
      })
      clearScratch()
      toast.success(`Saved “${wf.name}”`)
      navigate(`/workflows/${wf.id}`)
    } catch (err) {
      showProblems('Not saved', err as Error)
    }
  }
  const [clearing, setClearing] = useState(false)

  // Runs execute the saved workflow, so unsaved edits are saved first.
  const runWorkflow = async () => {
    if (useEditor.getState().dirty && !(await persist())) return
    const s = useEditor.getState()
    runner.run({ workflowId: workflow.id, workflowName: s.name, inputs: s.inputs })
  }

  const onKey = useEffectEvent((e: KeyboardEvent) => {
    if ((e.metaKey || e.ctrlKey) && e.key === 's' && !scratch) {
      e.preventDefault()
      void save()
    }
  })
  useEffect(() => {
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  // Clicking an item appends it after the selected step, so a sequence can be
  // built by clicking alone. With nothing selected it lands mid-canvas.
  const addFromPalette = (item: LibraryItem) => {
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
    addStep(item.template, position, anchor?.id)

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
        {scratch ? (
          <span
            className="hidden items-center gap-1.5 rounded-full bg-hover px-2 py-0.5 text-xs text-fg-muted sm:flex"
            title="Kept in this browser, not saved to Conductor. Runs don't save it either."
          >
            <FlaskConical className="size-3" /> Scratch · not saved
          </span>
        ) : (
          <>
            <SaveState dirty={dirty} saving={update.isPending} />
            <LastRun run={latest.run} />
          </>
        )}

        <div className="ml-auto flex items-center gap-1.5">
          <Menu>
            <MenuTrigger asChild>
              <Button variant="secondary" className="md:hidden" aria-label="Add step">
                <Plus className="size-3.5" />
              </Button>
            </MenuTrigger>
            <MenuContent>
              {library.map((item) => (
                <MenuItem
                  key={item.key}
                  icon={<BlockIcon name={item.icon} className="size-3.5" />}
                  onSelect={() => addFromPalette(item)}
                >
                  {item.name}
                </MenuItem>
              ))}
            </MenuContent>
          </Menu>
          <Button variant="ghost" onClick={() => setYamlOpen(true)}>
            <FileCode className="size-3.5" />
            <span className="hidden sm:inline">YAML</span>
          </Button>
          <Button variant="ghost" onClick={autoLayout}>
            <Network className="size-3.5" />
            <span className="hidden sm:inline">Tidy up</span>
          </Button>
          {scratch ? (
            <>
              <Button variant="ghost" onClick={() => setClearing(true)}>
                <Trash2 className="size-3.5" />
                <span className="hidden sm:inline">Clear</span>
              </Button>
              <Button
                variant="secondary"
                onClick={saveScratchAsWorkflow}
                disabled={create.isPending}
              >
                <Save className="size-3.5" />
                <span className="hidden sm:inline">Save as workflow</span>
              </Button>
              <Button onClick={testRun} disabled={tester.pending}>
                <Play className="size-3" fill="currentColor" />
                Run
              </Button>
            </>
          ) : (
            <>
              <Tooltip label="Run the workflow as it is now, without saving it" side="bottom">
                <Button variant="ghost" onClick={testRun} disabled={tester.pending}>
                  <FlaskConical className="size-3.5" />
                  <span className="hidden sm:inline">Test run</span>
                </Button>
              </Tooltip>
              <Button variant="secondary" onClick={save} disabled={update.isPending}>
                Save
                <kbd className="hidden font-sans text-[11px] text-fg-subtle sm:inline">⌘S</kbd>
              </Button>
              <Button onClick={runWorkflow} disabled={update.isPending || runner.pending}>
                <Play className="size-3" fill="currentColor" />
                Run
              </Button>
            </>
          )}
        </div>
      </header>

      <div className="flex min-h-0 flex-1">
        <Palette items={library} onAdd={addFromPalette} />
        <div className="flex min-w-0 flex-1 flex-col">
          <RunOverlayContext.Provider value={testRunId ? testRunStatuses : latest.statuses}>
            <Canvas items={libraryMap} />
          </RunOverlayContext.Provider>
          {testRunId && <RunPanel runId={testRunId} onClose={() => showTestRun(undefined)} />}
        </div>
        <Inspector blocks={blockMap} />
      </div>

      <YamlDialog open={yamlOpen} onOpenChange={setYamlOpen} />
      {scratch ? (
        <ConfirmDialog
          open={clearing}
          onOpenChange={setClearing}
          title="Clear the scratch workflow?"
          description="Its steps, inputs and outputs are removed. Runs already made stay in Runs."
          confirmLabel="Clear"
          destructive
          onConfirm={() => {
            clearScratch()
            setClearing(false)
            scratch.onCleared()
          }}
        />
      ) : (
        <UnsavedChangesGuard dirty={dirty} />
      )}
      <ConflictDialog
        open={conflict}
        onOpenChange={setConflict}
        onLoadLatest={() => void loadLatest()}
        onOverwrite={async () => {
          setConflict(false)
          if (await persist(true)) toast.success('Saved over the other version')
        }}
      />
      {runner.dialog}
      {tester.dialog}
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
function LastRun({ run }: { run: Run | undefined }) {
  if (!run) return null
  return (
    <Link
      to={`/runs/${run.id}`}
      className="hidden items-center gap-1.5 rounded-md px-2 py-1 text-xs text-fg-subtle hover:bg-hover hover:text-fg md:flex"
    >
      <StatusIcon status={run.status} className="size-3.5" />
      {isActive(run.status) ? 'Running now' : `Last run ${timeAgo(run.createdAt)}`}
    </Link>
  )
}

/** Shown when saving would silently replace someone else's newer save. */
function ConflictDialog({
  open,
  onOpenChange,
  onLoadLatest,
  onOverwrite,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  onLoadLatest: () => void
  onOverwrite: () => void
}) {
  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-black/50 backdrop-blur-[2px] data-[state=open]:animate-fade-in" />
        <Dialog.Content className="fixed top-1/2 left-1/2 z-50 w-[calc(100vw-32px)] max-w-md -translate-x-1/2 -translate-y-1/2 rounded-xl border border-line-strong bg-raised p-5 shadow-2xl shadow-black/50 data-[state=open]:animate-pop-in">
          <Dialog.Title className="text-sm font-medium text-fg">
            This workflow changed somewhere else
          </Dialog.Title>
          <Dialog.Description className="mt-2 text-fg-muted">
            It was saved from another tab or window after you opened it. Load that version to
            continue from it (your unsaved edits here are discarded), or overwrite it with yours.
          </Dialog.Description>
          <div className="mt-5 flex flex-wrap justify-end gap-2">
            <Dialog.Close asChild>
              <Button variant="ghost">Keep editing</Button>
            </Dialog.Close>
            <Button variant="secondary" onClick={onOverwrite}>
              Overwrite with mine
            </Button>
            <Button onClick={onLoadLatest}>Load latest</Button>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
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
