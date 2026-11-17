import { useMutation, useQueryClient } from '@tanstack/react-query'
import { ChevronDown, LogOut, Moon, Settings, Sun, UserCircle2 } from 'lucide-react'
import { useEffect, useId, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api'
import { setSession, useSession } from '../auth/session'
import { useTheme } from './theme'

export default function AccountToolbar({ className = '' }: { className?: string }) {
  const session = useSession().data
  const user = session?.state === 'in' ? session.user : undefined
  const { theme, toggle } = useTheme()
  const qc = useQueryClient()
  const [open, setOpen] = useState(false)
  const container = useRef<HTMLDivElement>(null)
  const trigger = useRef<HTMLButtonElement>(null)
  const menu = useRef<HTMLDivElement>(null)
  const initialFocus = useRef(0)
  const pending = useRef(false)
  const id = useId()
  const logout = useMutation({ mutationFn: api.logout, onSettled: () => setSession(qc, { state: 'out' }) })

  useEffect(() => {
    if (!open) return
    menu.current?.querySelectorAll<HTMLElement>('[role="menuitem"]')[initialFocus.current]?.focus()
    const outside = (e: PointerEvent) => {
      if (e.target instanceof Node && !container.current?.contains(e.target)) setOpen(false)
    }
    document.addEventListener('pointerdown', outside)
    return () => document.removeEventListener('pointerdown', outside)
  }, [open])

  return <div role="banner" aria-label="顶部工具栏" className={`flex shrink-0 items-center justify-end gap-2 ${className}`}>
    <button type="button" onClick={toggle} aria-label={theme === 'dark' ? '切换日间模式' : '切换暗色模式'} className="flex size-12 shrink-0 items-center justify-center rounded-md text-muted hover:bg-hover md:hidden">{theme === 'dark' ? <Sun className="size-5" aria-hidden /> : <Moon className="size-5" aria-hidden />}</button>
    <div ref={container} className="relative" onBlur={e => { if (!e.currentTarget.contains(e.relatedTarget)) setOpen(false) }}>
      <button ref={trigger} type="button" aria-label="账号菜单" title="账号菜单" aria-haspopup="menu" aria-expanded={open} aria-controls={open ? id : undefined}
        onClick={() => { initialFocus.current = 0; setOpen(value => !value) }}
        onKeyDown={e => { if (e.key === 'ArrowDown' || e.key === 'ArrowUp') { e.preventDefault(); initialFocus.current = e.key === 'ArrowUp' ? 1 : 0; setOpen(true) } }}
        className="flex min-h-12 items-center justify-center gap-1 rounded-full px-1 text-accent hover:bg-accent-soft focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent">
        <span className="flex size-9 items-center justify-center rounded-full border border-line bg-accent-soft"><UserCircle2 className="size-6" aria-hidden /></span><ChevronDown className={`size-4 transition-transform ${open ? 'rotate-180' : ''}`} aria-hidden />
      </button>
      {open && <div className="absolute top-full right-0 z-30 mt-2 w-60 max-w-[calc(100vw-2rem)] rounded-xl border border-line-strong bg-surface p-2 shadow-xl">
        <div className="min-w-0 border-b border-line px-3 py-3"><p className="break-words text-sm font-semibold text-ink">{user?.displayName || user?.username}</p><p className="mt-1 break-all text-xs text-muted">{user?.username} · {user?.role === 'admin' ? '管理员' : '普通账号'}</p></div>
        <div ref={menu} id={id} role="menu" aria-label="账号菜单" className="pt-1" onKeyDown={e => {
          const items = Array.from(menu.current?.querySelectorAll<HTMLElement>('[role="menuitem"]') ?? [])
          const current = items.indexOf(document.activeElement as HTMLElement)
          if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); setOpen(false); trigger.current?.focus() }
          else if (['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(e.key)) {
            e.preventDefault()
            const next = e.key === 'Home' ? 0 : e.key === 'End' ? items.length - 1 : (current + (e.key === 'ArrowDown' ? 1 : -1) + items.length) % items.length
            items[next]?.focus()
          }
        }}>
          <Link role="menuitem" tabIndex={-1} to="/me/account" onClick={() => { setOpen(false); trigger.current?.focus() }} className="flex min-h-12 items-center gap-3 rounded-lg px-3 text-sm text-body hover:bg-hover focus:bg-hover focus:outline-none"><Settings className="size-4" aria-hidden />账号管理</Link>
          <button role="menuitem" tabIndex={-1} type="button" aria-disabled={logout.isPending} onClick={() => { if (pending.current) return; pending.current = true; logout.mutate() }} className="flex min-h-12 w-full items-center gap-3 rounded-lg px-3 text-sm text-danger hover:bg-danger-soft focus:bg-danger-soft focus:outline-none aria-disabled:opacity-50"><LogOut className="size-4" aria-hidden />{logout.isPending ? '正在退出…' : '退出登录'}</button>
        </div>
      </div>}
    </div>
  </div>
}
