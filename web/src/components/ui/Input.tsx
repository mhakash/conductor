import type { InputHTMLAttributes, ReactNode, TextareaHTMLAttributes } from 'react'

import { cn } from '@/lib/cn'

const control =
  'w-full rounded-md border border-line bg-canvas px-2.5 text-[13px] text-fg placeholder:text-fg-subtle transition-colors hover:border-line-strong focus:border-brass/70 focus:outline-none aria-invalid:border-coral/70'

export function Input({ className, ...props }: InputHTMLAttributes<HTMLInputElement>) {
  return <input className={cn(control, 'h-8', className)} {...props} />
}

export function Textarea({ className, ...props }: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return <textarea className={cn(control, 'min-h-20 resize-y py-2', className)} {...props} />
}

type FieldProps = {
  label: string
  htmlFor?: string
  required?: boolean
  help?: ReactNode
  error?: string
  children: ReactNode
}

/** Label, control and help text stacked; the unit of every form. */
export function FormField({ label, htmlFor, required, help, error, children }: FieldProps) {
  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={htmlFor} className="text-xs font-medium text-fg-muted">
        {label}
        {required && <span className="ml-0.5 text-brass">*</span>}
      </label>
      {children}
      {error ? (
        <p className="text-xs text-coral">{error}</p>
      ) : (
        help && <p className="text-xs text-fg-subtle">{help}</p>
      )}
    </div>
  )
}
