import {
  Circle,
  CircleCheck,
  CircleDashed,
  CircleSlash,
  CircleX,
  LoaderCircle,
  type LucideIcon,
} from 'lucide-react'

import { cn } from '@/lib/cn'

import { statusLabel, type RunStatus } from './api'

const styles: Record<RunStatus, { icon: LucideIcon; className: string }> = {
  queued: { icon: Circle, className: 'text-steel' },
  running: {
    icon: LoaderCircle,
    className: 'text-brass animate-spin motion-reduce:animate-none',
  },
  succeeded: { icon: CircleCheck, className: 'text-sage' },
  failed: { icon: CircleX, className: 'text-coral' },
  cancelled: { icon: CircleSlash, className: 'text-fg-subtle' },
  skipped: { icon: CircleDashed, className: 'text-fg-subtle' },
}

export function StatusIcon({ status, className }: { status: RunStatus; className?: string }) {
  const { icon: Icon, className: tone } = styles[status]
  return (
    <Icon
      aria-label={statusLabel(status)}
      strokeWidth={2}
      className={cn('size-4 shrink-0', tone, className)}
    />
  )
}

export function StatusBadge({ status }: { status: RunStatus }) {
  return (
    <span className="inline-flex items-center gap-1.5 rounded-full border border-line bg-raised py-0.5 pr-2.5 pl-1.5 text-xs text-fg">
      <StatusIcon status={status} className="size-3.5" />
      {statusLabel(status)}
    </span>
  )
}
