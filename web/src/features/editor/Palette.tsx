import { Search } from 'lucide-react'
import { useState, type DragEvent } from 'react'

import { BlockIcon } from '@/features/blocks/BlockIcon'

import { matches, type LibraryItem } from './library'

export const LIBRARY_DRAG_TYPE = 'application/x-conductor-step'

type PaletteProps = {
  items: LibraryItem[]
  onAdd: (item: LibraryItem) => void
}

/** The library: blocks, presets and other workflows, searchable. */
export function Palette({ items, onAdd }: PaletteProps) {
  const [query, setQuery] = useState('')
  const q = query.trim()
  const shown = q ? items.filter((i) => matches(i, q)) : items
  const groups = Map.groupBy(shown, (i) => i.group)

  const onDragStart = (e: DragEvent, item: LibraryItem) => {
    e.dataTransfer.setData(LIBRARY_DRAG_TYPE, item.key)
    e.dataTransfer.effectAllowed = 'move'
  }

  return (
    <aside
      aria-label="Library"
      className="hidden w-60 shrink-0 flex-col border-r border-line bg-surface md:flex"
    >
      <div className="relative border-b border-line p-3">
        <Search className="pointer-events-none absolute top-1/2 left-5.5 size-3.5 -translate-y-1/2 text-fg-subtle" />
        <input
          type="search"
          aria-label="Search the library"
          placeholder="Search"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={(e) => e.key === 'Escape' && setQuery('')}
          className="h-8 w-full rounded-md border border-line bg-canvas pr-2 pl-7.5 text-[13px] text-fg transition-colors placeholder:text-fg-subtle hover:border-line-strong focus:border-brass/70 focus:outline-none"
        />
      </div>
      <div className="flex flex-1 flex-col gap-5 overflow-y-auto p-3">
        {q ? (
          shown.length === 0 && (
            <p className="px-1 text-xs text-fg-subtle">Nothing matches “{q}”.</p>
          )
        ) : (
          <p className="px-1 text-xs text-fg-subtle">
            Drag onto the canvas, or click to add after the selected step.
          </p>
        )}
        {[...groups].map(([group, list]) => (
          <section key={group}>
            <h3 className="mb-1.5 px-1 text-[11px] font-medium tracking-wide text-fg-subtle uppercase">
              {group}
            </h3>
            <ul className="flex flex-col gap-1">
              {list.map((item) => (
                <li key={item.key}>
                  <button
                    draggable
                    onDragStart={(e) => onDragStart(e, item)}
                    onClick={() => onAdd(item)}
                    className="group flex w-full cursor-grab items-start gap-2.5 rounded-lg border border-transparent p-2 text-left transition-colors hover:border-line hover:bg-raised active:cursor-grabbing"
                  >
                    <span className="mt-0.5 flex size-6 shrink-0 items-center justify-center rounded-md bg-hover text-fg-muted transition-colors group-hover:text-brass">
                      <BlockIcon name={item.icon} className="size-3.5" />
                    </span>
                    <span className="min-w-0">
                      <span className="block truncate font-medium text-fg">{item.name}</span>
                      <span className="line-clamp-2 block text-xs leading-4 text-fg-subtle">
                        {item.description}
                      </span>
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          </section>
        ))}
      </div>
    </aside>
  )
}
