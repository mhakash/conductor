import { useReactFlow, useStoreApi } from '@xyflow/react'
import { useReducedMotion } from 'motion/react'
import { useEffect, useRef } from 'react'

const MARGIN = 24

/**
 * Pans the view to a node when it becomes selected and isn't fully in view,
 * keeping the zoom. Render it inside a `<ReactFlow>`.
 */
export function RevealNode({ id }: { id: string | undefined }) {
  const { getViewport, setCenter } = useReactFlow()
  const store = useStoreApi()
  const reduceMotion = useReducedMotion()
  // The selection the page opens with is left to the initial fit view.
  const initial = useRef(id)

  useEffect(() => {
    if (!id || id === initial.current) return
    initial.current = undefined

    // Returns false while the node isn't measured; nodes rebuilt by their
    // owner lose their size until React Flow measures them again.
    const reveal = () => {
      const { nodeLookup, width, height } = store.getState()
      const node = nodeLookup.get(id)
      const w = node?.measured.width
      const h = node?.measured.height
      if (!node || !w || !h) return false

      const { x, y, zoom } = getViewport()
      const p = node.internals.positionAbsolute
      const left = p.x * zoom + x
      const top = p.y * zoom + y
      const inView =
        left >= MARGIN &&
        top >= MARGIN &&
        left + w * zoom <= width - MARGIN &&
        top + h * zoom <= height - MARGIN
      if (!inView) {
        void setCenter(p.x + w / 2, p.y + h / 2, { zoom, duration: reduceMotion ? 0 : 300 })
      }
      return true
    }

    if (reveal()) return
    const unsubscribe = store.subscribe(() => {
      if (reveal()) unsubscribe()
    })
    return unsubscribe
  }, [id, getViewport, setCenter, store, reduceMotion])

  return null
}
