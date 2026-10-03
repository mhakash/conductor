import type { DragEvent } from 'react'

import { BlockIcon } from '@/features/blocks/BlockIcon'
import type { Block } from '@/features/blocks/api'

export const BLOCK_DRAG_TYPE = 'application/x-conductor-block'

type PaletteProps = {
  blocks: Block[]
  onAdd: (block: Block) => void
}

export function Palette({ blocks, onAdd }: PaletteProps) {
  const categories = Map.groupBy(blocks, (b) => b.category)

  const onDragStart = (e: DragEvent, block: Block) => {
    e.dataTransfer.setData(BLOCK_DRAG_TYPE, block.type)
    e.dataTransfer.effectAllowed = 'move'
  }

  return (
    <aside
      aria-label="Blocks"
      className="hidden w-60 shrink-0 flex-col gap-5 overflow-y-auto border-r border-line bg-surface p-3 md:flex"
    >
      <p className="px-1 text-xs text-fg-subtle">
        Drag onto the canvas, or click to add after the selected step.
      </p>
      {[...categories].map(([category, items]) => (
        <section key={category}>
          <h3 className="mb-1.5 px-1 text-[11px] font-medium tracking-wide text-fg-subtle uppercase">
            {category}
          </h3>
          <ul className="flex flex-col gap-1">
            {items.map((block) => (
              <li key={block.type}>
                <button
                  draggable
                  onDragStart={(e) => onDragStart(e, block)}
                  onClick={() => onAdd(block)}
                  className="group flex w-full cursor-grab items-start gap-2.5 rounded-lg border border-transparent p-2 text-left transition-colors hover:border-line hover:bg-raised active:cursor-grabbing"
                >
                  <span className="mt-0.5 flex size-6 shrink-0 items-center justify-center rounded-md bg-hover text-fg-muted transition-colors group-hover:text-brass">
                    <BlockIcon name={block.icon} className="size-3.5" />
                  </span>
                  <span className="min-w-0">
                    <span className="block font-medium text-fg">{block.name}</span>
                    <span className="block text-xs leading-4 text-fg-subtle">
                      {block.description}
                    </span>
                  </span>
                </button>
              </li>
            ))}
          </ul>
        </section>
      ))}
    </aside>
  )
}
