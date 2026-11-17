import { Link } from 'react-router-dom'
import { ArrowLeft } from 'lucide-react'
import { useAccess } from '../auth/permissions'
import CurrentAccount from '../components/CurrentAccount'
import AdminPage from './AdminPage'

export default function AccountPage() {
  const access = useAccess()
  if (access.admin) return <AdminPage />
  return <main className="page-width min-w-0 space-y-4 px-4 pt-3 pb-8 md:px-6">
    <header className="flex min-w-0 items-center gap-3">
      <Link to="/me" aria-label="返回高级管理" title="返回高级管理" className="flex size-12 shrink-0 items-center justify-center rounded-lg text-muted hover:bg-hover focus-visible:outline-2 focus-visible:outline-accent"><ArrowLeft className="size-5" aria-hidden /></Link>
      <div><h1 className="text-xl font-semibold text-ink">账号管理</h1><p className="mt-1 text-sm text-muted">管理个人信息与登录设置</p></div>
    </header>
    <CurrentAccount />
  </main>
}
