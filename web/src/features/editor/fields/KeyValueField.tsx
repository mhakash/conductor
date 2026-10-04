import { Plus, X } from 'lucide-react'
import { useState } from 'react'

import { Input } from '@/components/ui/Input'

import { CodeField } from './CodeField'

type Row = { key: string; value: string }

type KeyValueFieldProps = {
  id: string
  value: Record<string, string>
  onChange: (value: Record<string, string>) => void
}

/**
 * Edits a string map as rows. Rows are kept locally so a half-typed or empty
 * key doesn't vanish; only rows with a key are written back.
 */
export function KeyValueField({ id, value, onChange }: KeyValueFieldProps) {
  const [rows, setRows] = useState<Row[]>(() =>
    Object.entries(value).map(([key, v]) => ({ key, value: v })),
  )

  const update = (next: Row[]) => {
    setRows(next)
    onChange(
      Object.fromEntries(next.filter((r) => r.key.trim()).map((r) => [r.key.trim(), r.value])),
    )
  }

  return (
    <div id={id} className="flex flex-col gap-1.5">
      {rows.map((row, i) => (
        <div key={i} className="flex items-center gap-1.5">
          <Input
            aria-label="Name"
            placeholder="NAME"
            value={row.key}
            className="w-32 shrink-0 font-mono text-xs"
            onChange={(e) =>
              update(rows.map((r, j) => (j === i ? { ...r, key: e.target.value } : r)))
            }
          />
          <div className="min-w-0 flex-1">
            <CodeField
              id={`${id}-value-${i}`}
              value={row.value}
              placeholder="value"
              singleLine
              onChange={(v) => update(rows.map((r, j) => (j === i ? { ...r, value: v } : r)))}
            />
          </div>
          <button
            aria-label="Remove"
            onClick={() => update(rows.filter((_, j) => j !== i))}
            className="flex size-8 shrink-0 items-center justify-center rounded-md text-fg-subtle hover:bg-hover hover:text-fg"
          >
            <X className="size-3.5" />
          </button>
        </div>
      ))}
      <button
        onClick={() => setRows([...rows, { key: '', value: '' }])}
        className="flex h-7 w-fit items-center gap-1 rounded-md px-1.5 text-xs text-fg-muted hover:bg-hover hover:text-fg"
      >
        <Plus className="size-3" /> Add
      </button>
    </div>
  )
}
