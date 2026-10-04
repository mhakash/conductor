import { History, KeyRound, Workflow, type LucideIcon } from 'lucide-react'
import { motion } from 'motion/react'
import { NavLink } from 'react-router'

import { Tooltip } from '@/components/ui/Tooltip'
import { EngineStatus } from '@/features/engine/EngineStatus'
import { cn } from '@/lib/cn'

type NavItem = { to: string; label: string; icon: LucideIcon }

const nav: NavItem[] = [
  { to: '/workflows', label: 'Workflows', icon: Workflow },
  { to: '/runs', label: 'Runs', icon: History },
  { to: '/secrets', label: 'Secrets', icon: KeyRound },
]

/**
 * Full width from md up, an icon rail below that. `rail` forces the icon rail
 * at every size, for pages that need the room (the editor).
 */
export function Sidebar({ rail = false }: { rail?: boolean }) {
  const wide = !rail
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
          <SidebarLink key={item.to} item={item} wide={wide} />
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

function SidebarLink({ item, wide }: { item: NavItem; wide: boolean }) {
  const Icon = item.icon
  return (
    // When labels are visible the tooltip would repeat them, so hide it.
    <Tooltip label={item.label} className={cn(wide && 'md:hidden')}>
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
            <Icon className="size-4 shrink-0" strokeWidth={1.75} />
            {wide && <span className="hidden md:inline">{item.label}</span>}
          </>
        )}
      </NavLink>
    </Tooltip>
  )
}
