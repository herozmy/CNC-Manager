import { X } from 'lucide-react'
import { useLayoutEffect, useRef } from 'react'
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
  const dialogRef = useRef<HTMLDialogElement>(null)
  useLayoutEffect(() => {
    const dialog = dialogRef.current
    if (!open || !dialog) return
    const trigger = document.activeElement
    dialog.showModal()
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      dialog.close()
      document.body.style.overflow = prev
      if (trigger instanceof HTMLElement && trigger.isConnected) trigger.focus({ preventScroll: true })
    }
  }, [open])

  if (!open) return null
  return createPortal(
    <dialog ref={dialogRef} aria-label={title} aria-modal="true"
      className="fixed inset-0 m-0 h-full max-h-none w-full max-w-none bg-transparent p-0 text-inherit backdrop:bg-black/50 open:flex open:items-end open:justify-center md:open:items-center"
      onCancel={(event) => { event.preventDefault(); onClose() }}
      onKeyDown={(event) => {
        if (event.defaultPrevented) return
        if (event.key === 'Escape') {
          event.preventDefault()
          onClose()
        }
        if (event.key !== 'Tab') return
        const controls = Array.from(event.currentTarget.querySelectorAll<HTMLElement>(
          'button, input, select, textarea, a[href], [tabindex], [contenteditable="true"]',
        )).filter(el => el.tabIndex >= 0 && !el.matches(':disabled') && el.getClientRects().length > 0)
        const first = controls[0]
        const last = controls.at(-1)
        const target = event.shiftKey ? last : first
        if (target && document.activeElement === (event.shiftKey ? first : last)) {
          event.preventDefault()
          target.focus()
        }
      }}
      onClick={(event) => { if (event.target === event.currentTarget) onClose() }}>
      <div
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
    </dialog>,
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
