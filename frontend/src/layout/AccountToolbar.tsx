import { Moon, Sun, UserCircle2 } from 'lucide-react'
import { Link } from 'react-router-dom'
import { useAccess } from '../auth/permissions'
import { useTheme } from './theme'

export default function AccountToolbar({ className = '' }: { className?: string }) {
  const access = useAccess()
  const { theme, toggle } = useTheme()
  return <div role="banner" aria-label="顶部工具栏" className={`flex shrink-0 items-center justify-end gap-2 ${className}`}>
    <button type="button" onClick={toggle} aria-label={theme === 'dark' ? '切换日间模式' : '切换暗色模式'} className="flex size-12 shrink-0 items-center justify-center rounded-md text-muted hover:bg-hover md:hidden">{theme === 'dark' ? <Sun className="size-5" aria-hidden /> : <Moon className="size-5" aria-hidden />}</button>
    <Link to={access.admin ? '/admin' : '/me/account'} aria-label={access.admin ? '账号管理' : '个人账号设置'} title={access.admin ? '账号管理' : '个人账号设置'} className="flex size-12 shrink-0 items-center justify-center rounded-full text-accent hover:bg-accent-soft focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent">
      <span className="flex size-9 items-center justify-center rounded-full border border-line bg-accent-soft"><UserCircle2 className="size-6" aria-hidden /></span>
    </Link>
  </div>
}
