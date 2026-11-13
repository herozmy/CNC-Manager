import { Loader2 } from 'lucide-react'
import type { ReactNode } from 'react'

export function Spinner({ label = '加载中…' }: { label?: string }) {
  return (
    <div role="status" className="flex items-center justify-center gap-2 py-16 text-slate-500">
      <Loader2 className="size-5 animate-spin" aria-hidden />
      <span>{label}</span>
    </div>
  )
}

export function ErrorState({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return (
    <div role="alert" className="mx-auto max-w-sm space-y-3 py-16 text-center">
      <p className="text-slate-700">{message}</p>
      {onRetry && (
        <button onClick={onRetry} className="min-h-12 rounded-xl border border-slate-300 bg-white px-5 font-semibold">
          重试
        </button>
      )}
    </div>
  )
}

export function EmptyState({ title, children }: { title: string; children?: ReactNode }) {
  return (
    <div className="mx-auto max-w-sm space-y-4 py-16 text-center">
      <p className="text-lg font-semibold text-slate-800">{title}</p>
      {children}
    </div>
  )
}
