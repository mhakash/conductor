import type { ButtonHTMLAttributes } from 'react'

import { cn } from '@/lib/cn'

type Variant = 'primary' | 'ghost'

const variants: Record<Variant, string> = {
  primary:
    'bg-brass text-canvas hover:bg-brass-bright disabled:bg-brass-dim disabled:text-canvas/70',
  ghost: 'text-fg-muted hover:bg-hover hover:text-fg disabled:text-fg-subtle',
}

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & { variant?: Variant }

export function Button({ variant = 'primary', className, ...props }: ButtonProps) {
  return (
    <button
      className={cn(
        'inline-flex h-8 items-center gap-1.5 rounded-md px-3 text-[13px] font-medium transition-colors disabled:cursor-not-allowed',
        variants[variant],
        className,
      )}
      {...props}
    />
  )
}
