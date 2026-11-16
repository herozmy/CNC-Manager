import clsx from 'clsx'
import { FileCode2, FolderKanban, Moon, PanelLeftClose, PanelLeftOpen, Plus, Sun, UserCircle2 } from 'lucide-react'
import { useState } from 'react'
import { Link, Outlet, useLocation } from 'react-router-dom'
import { useAccess } from '../auth/permissions'
import { useTheme } from './theme'

const items = [
  { to: '/', label: '总览', icon: FolderKanban },
  { to: '/drawings/new', label: '新建图纸', icon: Plus },
  { to: '/me', label: '我的', icon: UserCircle2 },
] as const

export default function AppShell() {
  const access = useAccess()
  const { pathname } = useLocation()
  const { theme, toggle } = useTheme()
  const [collapsed, setCollapsed] = useState(false)
  const isMine = pathname.startsWith('/me') || pathname.startsWith('/machines') || pathname.startsWith('/admin')
  const isActive = (to: string) => to === '/me' ? isMine : to === '/drawings/new' ? pathname === to : pathname === '/' || (pathname.startsWith('/drawings/') && pathname !== '/drawings/new')
  const visibleItems = items.filter(item => item.to !== '/drawings/new' || access.can('create'))

  return <div className="min-h-dvh bg-canvas text-ink [overflow-wrap:anywhere] md:flex">
    <a href="#page-content" className="sr-only z-50 bg-surface p-3 focus:not-sr-only focus:fixed focus:top-2 focus:left-2">跳到页面内容</a>
    <aside className={clsx('sticky top-0 hidden h-dvh shrink-0 flex-col border-r border-slate-800 bg-[#0B1120] text-slate-200 md:flex', collapsed ? 'w-16' : 'w-56')}>
      <div className="flex h-16 shrink-0 items-center gap-2 border-b border-slate-800 px-3">
        <Link to="/" aria-label="CNC-Manager 总览" className="flex min-w-0 flex-1 items-center gap-2">
          <span className="flex size-9 shrink-0 items-center justify-center rounded-md bg-blue-600 text-white"><FileCode2 className="size-5" aria-hidden /></span>
          {!collapsed && <span className="min-w-0"><span className="block truncate text-sm font-semibold text-white">CNC-Manager</span><span className="block text-[10px] tracking-wider text-slate-400">NC LEDGER</span></span>}
        </Link>
        {!collapsed && <button type="button" onClick={() => setCollapsed(true)} aria-label="收起侧边栏" className="flex size-8 shrink-0 items-center justify-center rounded-md hover:bg-slate-800"><PanelLeftClose className="size-4" aria-hidden /></button>}
      </div>
      <nav aria-label="桌面导航" className="flex-1 space-y-1 px-2 py-4">
        {!collapsed && <p className="mb-2 px-3 text-xs text-slate-500">核心台账</p>}
        {visibleItems.map(({ to, label, icon: Icon }) => <Link key={to} to={to} aria-label={label} title={collapsed ? label : undefined} aria-current={isActive(to) ? 'page' : undefined} className={clsx('flex min-h-12 items-center gap-3 rounded-md px-3 text-sm font-medium transition-colors focus-visible:outline-2 focus-visible:outline-blue-400', collapsed && 'justify-center px-0', isActive(to) ? 'bg-blue-600/20 text-blue-300' : 'text-slate-300 hover:bg-slate-800')}>
          <Icon className="size-5 shrink-0" aria-hidden />{!collapsed && <span>{label}</span>}
        </Link>)}
      </nav>
      <div className="space-y-1 border-t border-slate-800 p-2">
        <button type="button" onClick={toggle} aria-label={theme === 'dark' ? '切换日间模式' : '切换暗色模式'} title={theme === 'dark' ? '日间模式' : '暗色模式'} className={clsx('flex min-h-12 w-full items-center gap-3 rounded-md px-3 text-sm text-slate-300 hover:bg-slate-800', collapsed && 'justify-center px-0')}>
          {theme === 'dark' ? <Sun className="size-5 shrink-0 text-amber-400" aria-hidden /> : <Moon className="size-5 shrink-0 text-sky-400" aria-hidden />}{!collapsed && <span>{theme === 'dark' ? '车间暗色模式' : '日间明亮模式'}</span>}
        </button>
        {collapsed && <button type="button" onClick={() => setCollapsed(false)} aria-label="展开侧边栏" className="flex min-h-12 w-full items-center justify-center rounded-md hover:bg-slate-800"><PanelLeftOpen className="size-5" aria-hidden /></button>}
      </div>
    </aside>

    <div className="min-w-0 flex-1 pb-[calc(4.5rem+env(safe-area-inset-bottom))] md:pb-0">
      <header aria-label="顶部工具栏" className="flex min-h-14 items-center justify-end gap-2 border-b border-line bg-surface px-4 md:px-6">
        <button type="button" onClick={toggle} aria-label={theme === 'dark' ? '切换日间模式' : '切换暗色模式'} className="flex size-12 shrink-0 items-center justify-center rounded-md text-muted hover:bg-hover md:hidden">{theme === 'dark' ? <Sun className="size-5" aria-hidden /> : <Moon className="size-5" aria-hidden />}</button>
        <Link to={access.admin ? '/admin' : '/me/account'} aria-label={access.admin ? '账号管理' : '个人账号设置'} title={access.admin ? '账号管理' : '个人账号设置'} className="flex size-12 shrink-0 items-center justify-center rounded-full text-accent hover:bg-accent-soft focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent">
          <span className="flex size-9 items-center justify-center rounded-full border border-line bg-accent-soft"><UserCircle2 className="size-6" aria-hidden /></span>
        </Link>
      </header>
      <div id="page-content" tabIndex={-1} className="min-w-0 outline-none"><Outlet /></div>
    </div>

    <nav aria-label="主导航" className="fixed inset-x-0 bottom-0 z-20 flex items-center justify-around border-t border-line bg-surface/95 px-2 pt-1 pb-[max(0.25rem,env(safe-area-inset-bottom))] backdrop-blur md:hidden">
      {visibleItems.map(({ to, label, icon: Icon }) => <Link key={to} to={to} aria-label={label} aria-current={isActive(to) ? 'page' : undefined} className={clsx('relative flex min-h-14 flex-1 flex-col items-center justify-center gap-1 rounded-md text-xs', isActive(to) ? 'font-semibold text-accent' : 'text-muted')}>
        {isActive(to) && <span className="absolute top-0 h-0.5 w-8 rounded-full bg-accent" />}<Icon className="size-5" aria-hidden /><span>{label}</span>
      </Link>)}
    </nav>
  </div>
}
