import { Tooltip } from '@/components/ui/Tooltip'
import { cn } from '@/lib/cn'

import { useHealth } from './useHealth'

export function EngineStatus({ compact }: { compact?: boolean }) {
  const { data, isError, isPending } = useHealth()
  const online = !!data && !isError

  const label = isPending ? 'Connecting…' : online ? 'Engine online' : 'Engine offline'
  const detail = online
    ? data.version === 'dev'
      ? 'development build'
      : `v${data.version}`
    : 'Start it with make dev'

  const dot = (
    <span className="relative flex size-2">
      {online && (
        <span className="absolute inline-flex size-full animate-ping rounded-full bg-brass opacity-50 motion-reduce:hidden" />
      )}
      <span
        className={cn(
          'relative inline-flex size-2 rounded-full',
          isPending ? 'bg-fg-subtle' : online ? 'bg-brass' : 'bg-coral',
        )}
      />
    </span>
  )

  if (compact) {
    return (
      <Tooltip label={`${label} · ${detail}`}>
        <span role="status" aria-label={label} className="flex size-9 items-center justify-center">
          {dot}
        </span>
      </Tooltip>
    )
  }

  return (
    <div role="status" className="flex items-center gap-2.5 px-2 py-1.5">
      {dot}
      <div className="min-w-0 leading-tight">
        <p className="text-xs text-fg">{label}</p>
        <p className="truncate font-mono text-[11px] text-fg-subtle">{detail}</p>
      </div>
    </div>
  )
}
