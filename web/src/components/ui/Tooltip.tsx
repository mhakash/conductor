import * as RadixTooltip from '@radix-ui/react-tooltip'
import type { ReactNode } from 'react'

import { cn } from '@/lib/cn'

type TooltipProps = {
  label: ReactNode
  side?: RadixTooltip.TooltipContentProps['side']
  /** Extra classes for the tooltip bubble, e.g. to hide it at some breakpoints. */
  className?: string
  children: ReactNode
}

export function Tooltip({ label, side = 'right', className, children }: TooltipProps) {
  return (
    <RadixTooltip.Root>
      <RadixTooltip.Trigger asChild>{children}</RadixTooltip.Trigger>
      <RadixTooltip.Portal>
        <RadixTooltip.Content
          side={side}
          sideOffset={8}
          className={cn(
            'z-50 rounded-md border border-line-strong bg-raised px-2 py-1 text-xs text-fg shadow-lg shadow-black/40',
            className,
          )}
        >
          {label}
        </RadixTooltip.Content>
      </RadixTooltip.Portal>
    </RadixTooltip.Root>
  )
}

export const TooltipProvider = RadixTooltip.Provider
