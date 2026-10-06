import {
  Background,
  BackgroundVariant,
  Controls,
  ReactFlow,
  useReactFlow,
  type Connection,
  type Edge,
  type IsValidConnection,
} from '@xyflow/react'
import type { DragEvent } from 'react'

import { edgeId, wouldCreateCycle } from './graph'
import type { LibraryItem } from './library'
import { LIBRARY_DRAG_TYPE } from './Palette'
import { StepNodeView } from './StepNodeView'
import { useEditor } from './store'

const nodeTypes = { step: StepNodeView }

export function Canvas({ items }: { items: Map<string, LibraryItem> }) {
  const nodes = useEditor((s) => s.nodes)
  const edges = useEditor((s) => s.edges)
  const onNodesChange = useEditor((s) => s.onNodesChange)
  const onEdgesChange = useEditor((s) => s.onEdgesChange)
  const connect = useEditor((s) => s.connect)
  const addStep = useEditor((s) => s.addStep)
  const { screenToFlowPosition } = useReactFlow()

  const isValidConnection: IsValidConnection = (c: Connection | Edge) => {
    const all = useEditor.getState().edges
    if (all.some((e) => e.id === edgeId({ from: c.source, to: c.target }))) return false
    return !wouldCreateCycle(all, c.source, c.target)
  }

  const onDragOver = (e: DragEvent) => {
    if (!e.dataTransfer.types.includes(LIBRARY_DRAG_TYPE)) return
    e.preventDefault()
    e.dataTransfer.dropEffect = 'move'
  }

  const onDrop = (e: DragEvent) => {
    const item = items.get(e.dataTransfer.getData(LIBRARY_DRAG_TYPE))
    if (!item) return
    e.preventDefault()
    const p = screenToFlowPosition({ x: e.clientX, y: e.clientY })
    // Drop so the cursor sits near the node's header, not its corner.
    addStep(item.template, { x: p.x - 40, y: p.y - 20 })
  }

  return (
    <div className="relative flex-1" onDragOver={onDragOver} onDrop={onDrop}>
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        onConnect={connect}
        isValidConnection={isValidConnection}
        colorMode="dark"
        // An empty canvas starts at 100%; fitting nothing would zoom out.
        fitView={nodes.length > 0}
        fitViewOptions={{ padding: 0.3, maxZoom: 1 }}
        defaultViewport={{ x: 80, y: 80, zoom: 1 }}
        minZoom={0.25}
        maxZoom={1.75}
        snapToGrid
        snapGrid={[8, 8]}
        deleteKeyCode={['Backspace', 'Delete']}
        defaultEdgeOptions={{ interactionWidth: 16 }}
      >
        <Background variant={BackgroundVariant.Dots} gap={20} size={1.2} />
        <Controls showInteractive={false} position="bottom-left" />
      </ReactFlow>

      {nodes.length === 0 && (
        <div className="pointer-events-none absolute inset-0 flex items-center justify-center">
          <div className="max-w-xs rounded-xl border border-dashed border-line-strong px-5 py-4 text-center">
            <p className="font-medium text-fg">Add your first step</p>
            <p className="mt-1 text-fg-muted">
              Drag a block, preset or workflow from the left. Connect steps to make one wait for
              another.
            </p>
          </div>
        </div>
      )}
    </div>
  )
}
