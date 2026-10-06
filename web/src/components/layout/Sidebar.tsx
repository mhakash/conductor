import { History, KeyRound, Library, Workflow, type LucideIcon } from 'lucide-react'
import { motion } from 'motion/react'
import { NavLink } from 'react-router'

import { Tooltip } from '@/components/ui/Tooltip'
import { EngineStatus } from '@/features/engine/EngineStatus'
import { useWaitingRuns } from '@/features/runs/api'
import { cn } from '@/lib/cn'

type NavItem = { to: string; label: string; icon: LucideIcon }

const nav: NavItem[] = [
  { to: '/workflows', label: 'Workflows', icon: Workflow },
  { to: '/runs', label: 'Runs', icon: History },
  { to: '/library', label: 'Library', icon: Library },
  { to: '/secrets', label: 'Secrets', icon: KeyRound },
]

/**
 * Full width from md up, an icon rail below that. `rail` forces the icon rail
 * at every size, for pages that need the room (the editor).
 */
export function Sidebar({ rail = false }: { rail?: boolean }) {
  const wide = !rail
  // Runs waiting for approval need you, wherever you are in the app.
  const waiting = useWaitingRuns().data?.length ?? 0
  return (
    <aside
      className={cn(
        'flex w-14 shrink-0 flex-col border-r border-line bg-surface transition-[width] duration-200',
        wide && 'md:w-56',
      )}
    >
      <div className={cn('flex h-14 items-center justify-center px-4', wide && 'md:justify-start')}>
        <span className={cn('font-display text-2xl text-fg italic', wide && 'md:hidden')}>C</span>
        {wide && (
          <span className="hidden font-display text-2xl tracking-tight text-fg italic md:inline">
            Conductor
          </span>
        )}
      </div>

      <nav aria-label="Main" className="flex flex-1 flex-col gap-0.5 px-2 pt-2">
        {nav.map((item) => (
          <SidebarLink
            key={item.to}
            item={item}
            wide={wide}
            badge={item.to === '/runs' ? waiting : 0}
          />
        ))}
      </nav>

      <div className="border-t border-line p-2">
        {wide && (
          <div className="hidden md:block">
            <EngineStatus />
          </div>
        )}
        <div className={cn('flex justify-center', wide && 'md:hidden')}>
          <EngineStatus compact />
        </div>
      </div>
    </aside>
  )
}

function SidebarLink({ item, wide, badge }: { item: NavItem; wide: boolean; badge: number }) {
  const Icon = item.icon
  const waitingLabel = badge > 0 ? `${badge} waiting for approval` : ''
  return (
    // When labels are visible the tooltip would repeat them, so hide it.
    <Tooltip
      label={waitingLabel ? `${item.label} · ${waitingLabel}` : item.label}
      className={cn(wide && 'md:hidden')}
    >
      {/* className must be a string: the tooltip trigger merges it via Radix Slot. */}
      <NavLink
        to={item.to}
        className={cn(
          'relative flex h-9 items-center justify-center gap-2.5 rounded-md px-2.5 text-fg-muted transition-colors hover:bg-hover/60 hover:text-fg aria-[current=page]:bg-hover aria-[current=page]:text-fg',
          wide && 'md:justify-start',
        )}
      >
        {({ isActive }) => (
          <>
            {isActive && (
              <motion.span
                layoutId="nav-indicator"
                className="absolute inset-y-2 left-0 w-0.5 rounded-full bg-brass"
                transition={{ type: 'spring', stiffness: 500, damping: 40 }}
              />
            )}
            <span className="relative">
              <Icon className="size-4 shrink-0" strokeWidth={1.75} />
              {badge > 0 && (
                <span
                  aria-hidden
                  className={cn(
                    'absolute -top-0.5 -right-0.5 size-1.5 animate-pulse rounded-full bg-brass motion-reduce:animate-none',
                    wide && 'md:hidden',
                  )}
                />
              )}
            </span>
            {wide && <span className="hidden md:inline">{item.label}</span>}
            {badge > 0 && (
              <span
                aria-label={waitingLabel}
                className={cn(
                  'ml-auto hidden rounded-full bg-brass/15 px-1.5 font-mono text-[11px] text-brass tabular-nums',
                  wide && 'md:inline',
                )}
              >
                {badge}
              </span>
            )}
          </>
        )}
      </NavLink>
    </Tooltip>
  )
}
