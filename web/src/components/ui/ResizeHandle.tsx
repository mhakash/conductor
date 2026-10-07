import { cn } from '@/lib/cn'
import type { useBottomPanelHeight } from '@/lib/useBottomPanelHeight'

/**
 * The drag handle on a bottom panel's top edge. Put it first inside the
 * panel, which must be `relative`.
 */
export function ResizeHandle({
  className,
  ...props
}: ReturnType<typeof useBottomPanelHeight>['handleProps'] & { className?: string }) {
  return (
    <div
      role="separator"
      aria-orientation="horizontal"
      aria-label="Resize panel"
      title="Drag to resize · double-click to reset"
      tabIndex={0}
      className={cn(
        'group absolute inset-x-0 -top-1 z-10 flex h-2 cursor-row-resize touch-none items-center focus-visible:outline-none',
        className,
      )}
      {...props}
    >
      <div className="h-px w-full bg-transparent transition-colors group-hover:h-0.5 group-hover:bg-brass/60 group-focus-visible:h-0.5 group-focus-visible:bg-brass group-active:h-0.5 group-active:bg-brass" />
    </div>
  )
}
