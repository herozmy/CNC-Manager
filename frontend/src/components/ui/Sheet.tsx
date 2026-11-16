import { X } from 'lucide-react'
import { useEffect } from 'react'
import type { ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { Button } from './Button'

interface SheetProps {
  open: boolean
  title: string
  onClose: () => void
  children: ReactNode
  wide?: boolean
}

/** 手机上从底部滑出、宽屏居中的弹层。Esc / 点遮罩关闭，打开时锁定背景滚动。 */
export function Sheet({ open, title, onClose, children, wide = false }: SheetProps) {
  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && !e.defaultPrevented && onClose()
    window.addEventListener('keydown', onKey)
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      window.removeEventListener('keydown', onKey)
      document.body.style.overflow = prev
    }
  }, [open, onClose])

  if (!open) return null
  return createPortal(
    <div className="fixed inset-0 z-30 flex items-end justify-center bg-black/50 md:items-center" onClick={onClose}>
      <div
        role="dialog"
        aria-modal="true"
        aria-label={title}
        onClick={(e) => e.stopPropagation()}
        className={`max-h-[92dvh] w-full overflow-y-auto rounded-t-3xl bg-surface p-5 pb-[max(1.25rem,env(safe-area-inset-bottom))] shadow-2xl ${wide ? 'md:max-w-5xl' : 'md:max-w-lg'} md:rounded-3xl`}
      >
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-xl font-bold">{title}</h2>
          <button onClick={onClose} aria-label="关闭" className="flex size-12 items-center justify-center rounded-md hover:bg-inset">
            <X className="size-6" aria-hidden />
          </button>
        </div>
        {children}
      </div>
    </div>,
    document.body,
  )
}

interface ConfirmProps {
  open: boolean
  title: string
  message: string
  confirmLabel: string
  danger?: boolean
  busy?: boolean
  onConfirm: () => void
  onClose: () => void
}

/** 二次确认；破坏性操作用 danger 变红。 */
export function ConfirmSheet({ open, title, message, confirmLabel, danger, busy, onConfirm, onClose }: ConfirmProps) {
  return (
    <Sheet open={open} title={title} onClose={onClose}>
      <p className="mb-5 text-body">{message}</p>
      <div className="grid grid-cols-2 gap-3">
        <Button variant="secondary" onClick={onClose}>
          取消
        </Button>
        <Button variant={danger ? 'danger' : 'primary'} loading={busy} onClick={onConfirm}>
          {confirmLabel}
        </Button>
      </div>
    </Sheet>
  )
}
