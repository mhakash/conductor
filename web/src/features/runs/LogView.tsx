import { useEffect, useRef } from 'react'

import { cn } from '@/lib/cn'

import type { LogLine } from './api'

const timeFormat = new Intl.DateTimeFormat(undefined, {
  hour: '2-digit',
  minute: '2-digit',
  second: '2-digit',
  hour12: false,
})

type LogViewProps = {
  lines: LogLine[]
  /** Keep scrolled to the newest line while output is still arriving. */
  follow: boolean
  empty: string
}

export function LogView({ lines, follow, empty }: LogViewProps) {
  const ref = useRef<HTMLDivElement>(null)
  const pinned = useRef(true)

  useEffect(() => {
    const el = ref.current
    if (el && follow && pinned.current) el.scrollTop = el.scrollHeight
  }, [lines.length, follow])

  if (lines.length === 0) {
    return <p className="px-4 py-3 font-mono text-xs text-fg-subtle">{empty}</p>
  }

  return (
    <div
      ref={ref}
      role="log"
      onScroll={(e) => {
        const el = e.currentTarget
        // Stop following when the user scrolls up; resume at the bottom.
        pinned.current = el.scrollHeight - el.scrollTop - el.clientHeight < 24
      }}
      className="max-h-[60vh] overflow-auto py-2 font-mono text-xs leading-5"
    >
      {lines.map((l) => (
        <div key={l.seq} className="group flex gap-3 px-4 hover:bg-hover/50">
          <time
            className="hidden shrink-0 text-fg-subtle/70 select-none sm:block"
            dateTime={l.time}
          >
            {timeFormat.format(new Date(l.time))}
          </time>
          <span className="w-6 shrink-0 text-fg-subtle select-none">
            {l.stream === 'stderr' ? 'err' : l.stream === 'system' ? '›' : ''}
          </span>
          <span
            className={cn(
              'min-w-0 break-words whitespace-pre-wrap',
              l.stream === 'stdout' && 'text-fg',
              l.stream === 'stderr' && 'text-fg-muted',
              l.stream === 'system' && 'text-brass-dim',
            )}
          >
            {l.item !== undefined && (
              <span className="mr-2 text-fg-subtle select-none" title={`Item ${l.item + 1}`}>
                #{l.item + 1}
              </span>
            )}
            {l.text || ' '}
          </span>
        </div>
      ))}
    </div>
  )
}
