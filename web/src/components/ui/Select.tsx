import * as RadixSelect from '@radix-ui/react-select'
import { Check, ChevronDown } from 'lucide-react'
import type { ReactNode } from 'react'

import { cn } from '@/lib/cn'

export type SelectOption = { value: string; label: ReactNode; disabled?: boolean }

type SelectProps = {
  value: string
  onValueChange: (value: string) => void
  options: SelectOption[]
  /** Shown when value matches no option, e.g. "Choose…". */
  placeholder?: ReactNode
  id?: string
  className?: string
  disabled?: boolean
  'aria-label'?: string
  'aria-invalid'?: boolean
}

// Radix reserves "" for "no value", so an option meaning "none" travels as this.
const EMPTY = '__empty__'
const toRadix = (v: string) => (v === '' ? EMPTY : v)
const fromRadix = (v: string) => (v === EMPTY ? '' : v)

export function Select({
  value,
  onValueChange,
  options,
  placeholder,
  id,
  className,
  disabled,
  ...aria
}: SelectProps) {
  const known = options.some((o) => o.value === value)

  return (
    <RadixSelect.Root
      value={known ? toRadix(value) : ''}
      onValueChange={(v) => onValueChange(fromRadix(v))}
      disabled={disabled}
    >
      <RadixSelect.Trigger
        id={id}
        {...aria}
        className={cn(
          'group flex h-8 w-full min-w-0 items-center justify-between gap-2 rounded-md border border-line bg-canvas pr-2.5 pl-2.5 text-left text-[13px] text-fg transition-colors outline-none hover:border-line-strong focus-visible:border-brass/70 disabled:cursor-not-allowed disabled:opacity-50 aria-invalid:border-coral/70 data-placeholder:text-fg-subtle data-[state=open]:border-brass/70',
          className,
        )}
      >
        <span className="truncate">
          <RadixSelect.Value placeholder={placeholder} />
        </span>
        <RadixSelect.Icon asChild>
          <ChevronDown className="size-3.5 shrink-0 text-fg-subtle transition-transform duration-200 group-hover:text-fg-muted group-data-[state=open]:rotate-180 group-data-[state=open]:text-fg-muted motion-reduce:transition-none" />
        </RadixSelect.Icon>
      </RadixSelect.Trigger>
      <RadixSelect.Portal>
        <RadixSelect.Content
          position="popper"
          sideOffset={4}
          className="z-50 max-h-(--radix-select-content-available-height) min-w-(--radix-select-trigger-width) overflow-hidden rounded-lg border border-line-strong bg-raised shadow-xl shadow-black/40 data-[state=open]:animate-pop-in"
        >
          <RadixSelect.Viewport className="p-1">
            {options.map((o) => (
              <RadixSelect.Item
                key={o.value}
                value={toRadix(o.value)}
                disabled={o.disabled}
                className="relative flex h-8 cursor-default items-center rounded-md pr-8 pl-2 text-[13px] text-fg outline-none select-none data-disabled:text-fg-subtle data-highlighted:bg-hover"
              >
                <RadixSelect.ItemText>{o.label}</RadixSelect.ItemText>
                <RadixSelect.ItemIndicator className="absolute right-2 flex items-center">
                  <Check className="size-3.5 text-brass" />
                </RadixSelect.ItemIndicator>
              </RadixSelect.Item>
            ))}
          </RadixSelect.Viewport>
        </RadixSelect.Content>
      </RadixSelect.Portal>
    </RadixSelect.Root>
  )
}
