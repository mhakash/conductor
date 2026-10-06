import { ReactFlowProvider } from '@xyflow/react'
import { useState } from 'react'

import { useBlocks } from '@/features/blocks/api'

import { Editor } from './EditorPage'
import { loadScratch } from './scratch'

/** The editor on a workflow that isn't saved, for trying things out. */
export function ScratchPage() {
  const blocks = useBlocks()
  // Bumped by "Clear" to start again from an empty scratch.
  const [generation, setGeneration] = useState(0)
  if (!blocks.data) return null
  const { workflow, runId } = loadScratch()
  return (
    <ReactFlowProvider>
      <Editor
        key={generation}
        workflow={workflow}
        scratch={{ runId, onCleared: () => setGeneration((g) => g + 1) }}
      />
    </ReactFlowProvider>
  )
}
