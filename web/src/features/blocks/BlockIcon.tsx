import { Box, Globe, Terminal, type LucideIcon, type LucideProps } from 'lucide-react'

// Blocks name their icon; only icons in use are imported to keep the bundle small.
const icons: Record<string, LucideIcon> = {
  terminal: Terminal,
  globe: Globe,
}

export function BlockIcon({ name, ...props }: { name: string } & LucideProps) {
  const Icon = icons[name] ?? Box
  return <Icon strokeWidth={1.75} {...props} />
}
