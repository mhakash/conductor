import { useState, type KeyboardEvent, type PointerEvent } from 'react'

type Options = {
  /** localStorage key the height is remembered under. */
  storageKey: string
  initial: number
  min: number
  /** Space kept for the content above the panel. */
  reserve: number
}

/**
 * The height of a panel docked to the bottom of a flex column, resized by
 * dragging the handle on its top edge. The height is remembered per key.
 */
export function useBottomPanelHeight({ storageKey, initial, min, reserve }: Options) {
  const [height, setHeight] = useState(() => {
    try {
      const saved = Number(localStorage.getItem(storageKey))
      return saved >= min ? saved : initial
    } catch {
      return initial
    }
  })

  const commit = (h: number, parent: HTMLElement | null) => {
    const max = Math.max(min, (parent?.clientHeight ?? Infinity) - reserve)
    const next = Math.round(Math.min(max, Math.max(min, h)))
    setHeight(next)
    try {
      localStorage.setItem(storageKey, String(next))
    } catch {
      // Not remembered; the panel still resizes.
    }
  }

  const onPointerDown = (e: PointerEvent<HTMLDivElement>) => {
    if (e.button !== 0) return
    e.preventDefault()
    const handle = e.currentTarget
    const parent = handle.parentElement?.parentElement ?? null
    const startY = e.clientY
    const startH = handle.parentElement?.offsetHeight ?? height
    handle.setPointerCapture(e.pointerId)
    const move = (ev: globalThis.PointerEvent) => commit(startH + startY - ev.clientY, parent)
    const up = () => {
      handle.removeEventListener('pointermove', move)
      handle.removeEventListener('pointerup', up)
      handle.removeEventListener('pointercancel', up)
    }
    handle.addEventListener('pointermove', move)
    handle.addEventListener('pointerup', up)
    handle.addEventListener('pointercancel', up)
  }

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    const step = e.shiftKey ? 64 : 16
    const delta = e.key === 'ArrowUp' ? step : e.key === 'ArrowDown' ? -step : 0
    if (!delta) return
    e.preventDefault()
    commit(height + delta, e.currentTarget.parentElement?.parentElement ?? null)
  }

  return {
    height,
    handleProps: {
      onPointerDown,
      onKeyDown,
      onDoubleClick: () => commit(initial, null),
      'aria-valuenow': height,
      'aria-valuemin': min,
    },
  }
}
