import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { Link } from 'react-router-dom'
import { ArrowLeft, LogOut, UserRound } from 'lucide-react'
import { api } from '../api'
import { setSession, useSession } from '../auth/session'
import AccountSheet from '../components/AccountSheet'
import { Button } from '../components/ui/Button'

export default function AccountPage() {
  const [sheet, setSheet] = useState<'name' | 'password' | null>(null)
  const [saved, setSaved] = useState(false)
  const qc = useQueryClient()
  const session = useSession().data
  const logout = useMutation({
    mutationFn: api.logout,
    onSettled: () => setSession(qc, { state: 'out' }),
  })
  if (session?.state !== 'in') return null
  const { user } = session

  return <main className="page-width min-w-0 space-y-6 px-4 pt-4 pb-8 md:px-6">
    <header>
      <Link to="/me" className="inline-flex min-h-12 items-center gap-2 rounded-md px-2 text-sm text-muted hover:bg-hover focus-visible:outline-2 focus-visible:outline-accent"><ArrowLeft className="size-5" aria-hidden />返回我的</Link>
      <h1 className="mt-2 text-xl font-semibold text-ink">当前账号</h1>
      <p className="mt-1 text-sm text-muted">管理个人信息与登录设置</p>
    </header>
    <section aria-labelledby="account-heading" className="panel min-w-0 p-4 md:p-5">
      <div className="flex min-w-0 flex-col gap-6 md:flex-row md:items-center md:justify-between">
        <div className="min-w-0 flex-1">
          <h2 id="account-heading" className="text-sm font-semibold text-body">当前账号</h2>
          <div className="mt-4 flex items-start gap-4">
            <span className="flex size-14 shrink-0 items-center justify-center rounded-xl bg-accent-soft text-accent"><UserRound className="size-7" aria-hidden /></span>
            <div className="min-w-0 flex-1">
              <p className="break-words text-lg font-semibold leading-7 text-ink">{user.displayName || user.username}</p>
              <p className="mt-1 break-words text-sm leading-6 text-muted">{user.username} · {user.role === 'admin' ? '管理员' : '普通账号'}</p>
            </div>
          </div>
        </div>
        <div className="grid grid-cols-2 gap-3 md:w-72 md:shrink-0">
          <Button variant="secondary" className="px-3" onClick={() => { setSaved(false); setSheet('name') }}>修改名称</Button>
          <Button variant="secondary" className="px-3" onClick={() => { setSaved(false); setSheet('password') }}>修改密码</Button>
        </div>
      </div>
      {saved && <p role="status" className="mt-4 rounded-md bg-success-soft p-3 text-sm text-success">名称已保存</p>}
    </section>
    <footer className="border-t border-line pt-4">
      <Button variant="secondary" loading={logout.isPending} onClick={() => logout.mutate()} className="w-full sm:w-auto"><LogOut className="size-4" aria-hidden />退出登录</Button>
    </footer>
    {sheet && <AccountSheet kind={sheet} user={user} onClose={() => setSheet(null)} onSaved={() => { setSheet(null); setSaved(true) }} />}
  </main>
}
