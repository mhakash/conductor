import { History, Workflow, type LucideIcon } from 'lucide-react'
import { motion } from 'motion/react'
import { NavLink } from 'react-router'

import { Tooltip } from '@/components/ui/Tooltip'
import { EngineStatus } from '@/features/engine/EngineStatus'

type NavItem = { to: string; label: string; icon: LucideIcon }

const nav: NavItem[] = [
  { to: '/workflows', label: 'Workflows', icon: Workflow },
  { to: '/runs', label: 'Runs', icon: History },
]

export function Sidebar() {
  return (
    <aside className="flex w-14 shrink-0 flex-col border-r border-line bg-surface md:w-56">
      <div className="flex h-14 items-center justify-center px-4 md:justify-start">
        <span className="font-display text-2xl text-fg italic md:hidden">C</span>
        <span className="hidden font-display text-2xl tracking-tight text-fg italic md:inline">
          Conductor
        </span>
      </div>

      <nav aria-label="Main" className="flex flex-1 flex-col gap-0.5 px-2 pt-2">
        {nav.map((item) => (
          <SidebarLink key={item.to} item={item} />
        ))}
      </nav>

      <div className="border-t border-line p-2">
        <div className="hidden md:block">
          <EngineStatus />
        </div>
        <div className="flex justify-center md:hidden">
          <EngineStatus compact />
        </div>
      </div>
    </aside>
  )
}

function SidebarLink({ item }: { item: NavItem }) {
  const Icon = item.icon
  return (
    // Labels are visible from md up, so the tooltip only matters on the icon rail.
    <Tooltip label={item.label} className="md:hidden">
      {/* className must be a string: the tooltip trigger merges it via Radix Slot. */}
      <NavLink
        to={item.to}
        className="relative flex h-9 items-center justify-center gap-2.5 rounded-md px-2.5 text-fg-muted transition-colors hover:bg-hover/60 hover:text-fg aria-[current=page]:bg-hover aria-[current=page]:text-fg md:justify-start"
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
            <span className="hidden md:inline">{item.label}</span>
          </>
        )}
      </NavLink>
    </Tooltip>
  )
}
