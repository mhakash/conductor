import type { ButtonHTMLAttributes } from 'react'

import { cn } from '@/lib/cn'

type Variant = 'primary' | 'secondary' | 'ghost' | 'danger'

const variants: Record<Variant, string> = {
  primary:
    'bg-brass text-canvas hover:bg-brass-bright disabled:bg-brass-dim disabled:text-canvas/70',
  secondary: 'border border-line-strong bg-raised text-fg hover:bg-hover disabled:text-fg-subtle',
  ghost: 'text-fg-muted hover:bg-hover hover:text-fg disabled:text-fg-subtle',
  danger: 'bg-coral text-canvas hover:bg-coral/90 disabled:opacity-60',
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
