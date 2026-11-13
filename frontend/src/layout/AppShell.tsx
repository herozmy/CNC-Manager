import clsx from 'clsx'
import { Home, Plus, User } from 'lucide-react'
import { NavLink, Outlet } from 'react-router-dom'

const items = [
  { to: '/', label: '总览', icon: Home, end: true },
  { to: '/drawings/new', label: '新建图纸', icon: Plus, primary: true },
  { to: '/me', label: '我的', icon: User },
] as const

/**
 * 手机：底部标签栏，「新建图纸」居中突出；
 * 平板/桌面（md 以上）：左侧导航。
 */
export default function AppShell() {
  return (
    <div className="min-h-dvh bg-slate-50 text-slate-900 md:flex">
      <aside className="hidden w-56 shrink-0 flex-col gap-2 bg-slate-900 p-4 md:flex">
        <p className="mb-4 px-3 text-lg font-bold text-amber-400">CNC-Manager</p>
        {items.map(({ to, label, icon: Icon, ...rest }) => (
          <NavLink
            key={to}
            to={to}
            end={'end' in rest}
            className={({ isActive }) =>
              clsx(
                'flex min-h-12 items-center gap-3 rounded-xl px-3 font-medium',
                isActive ? 'bg-amber-500 text-slate-900' : 'text-slate-300 hover:bg-slate-800',
              )
            }
          >
            <Icon className="size-5" aria-hidden />
            {label}
          </NavLink>
        ))}
      </aside>

      <div className="min-w-0 flex-1 pb-24 md:pb-0">
        <Outlet />
      </div>

      <nav
        aria-label="主导航"
        className="fixed inset-x-0 bottom-0 z-10 flex items-end justify-around border-t border-slate-200 bg-white px-2 pt-1 pb-[max(0.25rem,env(safe-area-inset-bottom))] md:hidden"
      >
        {items.map(({ to, label, icon: Icon, ...rest }) => (
          <NavLink key={to} to={to} end={'end' in rest} aria-label={label} className="flex flex-1 flex-col items-center gap-0.5 py-1 text-xs">
            {({ isActive }) =>
              'primary' in rest ? (
                <>
                  <span className="-mt-6 flex size-14 items-center justify-center rounded-full bg-amber-500 text-slate-900 shadow-lg ring-4 ring-slate-50">
                    <Icon className="size-7" aria-hidden />
                  </span>
                  <span className="font-semibold text-slate-900">{label}</span>
                </>
              ) : (
                <>
                  <Icon className={clsx('size-6', isActive ? 'text-amber-600' : 'text-slate-500')} aria-hidden />
                  <span className={isActive ? 'font-semibold text-slate-900' : 'text-slate-500'}>{label}</span>
                </>
              )
            }
          </NavLink>
        ))}
      </nav>
    </div>
  )
}
