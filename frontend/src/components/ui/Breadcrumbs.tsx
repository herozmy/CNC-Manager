import { ChevronRight } from 'lucide-react'
import { Link } from 'react-router-dom'

export interface Crumb { label: string; to?: string; mono?: boolean }

export function Breadcrumbs({ items, className = 'mb-4' }: { items: Crumb[]; className?: string }) {
  return <nav aria-label="页面路径" className={`min-w-0 text-sm text-muted ${className}`}>
    <ol className="flex min-w-0 flex-wrap items-center gap-x-1 gap-y-0.5">
      {items.map((item, index) => <li key={`${index}-${item.label}`} className="flex min-w-0 items-center gap-1">
        {index > 0 && <ChevronRight className="size-3.5 shrink-0" aria-hidden />}
        {item.to ? <Link to={item.to} className={`min-w-0 rounded px-1 py-2 break-all hover:text-accent ${item.mono ? 'font-mono' : ''}`}>{item.label}</Link> : <span aria-current="page" className={`min-w-0 px-1 py-2 font-medium break-all text-body ${item.mono ? 'font-mono' : ''}`}>{item.label}</span>}
      </li>)}
    </ol>
  </nav>
}
