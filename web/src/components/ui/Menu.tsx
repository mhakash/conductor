import * as DropdownMenu from '@radix-ui/react-dropdown-menu'
import type { ReactNode } from 'react'

import { cn } from '@/lib/cn'

export const Menu = DropdownMenu.Root
export const MenuTrigger = DropdownMenu.Trigger

export function MenuContent({ children }: { children: ReactNode }) {
  return (
    <DropdownMenu.Portal>
      <DropdownMenu.Content
        align="end"
        sideOffset={4}
        className="z-50 min-w-40 rounded-lg border border-line-strong bg-raised p-1 shadow-xl shadow-black/40"
      >
        {children}
      </DropdownMenu.Content>
    </DropdownMenu.Portal>
  )
}

type MenuItemProps = {
  icon?: ReactNode
  children: ReactNode
  onSelect: () => void
  destructive?: boolean
}

export function MenuItem({ icon, children, onSelect, destructive }: MenuItemProps) {
  return (
    <DropdownMenu.Item
      onSelect={onSelect}
      className={cn(
        'flex h-8 cursor-default items-center gap-2 rounded-md px-2 text-[13px] outline-none select-none data-highlighted:bg-hover',
        destructive ? 'text-coral' : 'text-fg',
      )}
    >
      {icon}
      {children}
    </DropdownMenu.Item>
  )
}
