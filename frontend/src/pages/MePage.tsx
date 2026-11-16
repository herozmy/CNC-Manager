import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { useState } from 'react'
import { Archive, ChevronRight, Factory, LogOut, UserRound, Users } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { api } from '../api'
import { useAccess } from '../auth/permissions'
import { setSession, useSession } from '../auth/session'
import { Button } from '../components/ui/Button'
import AccountSheet from '../components/AccountSheet'
function ManagementCard({ to, label, title, description, icon: Icon }: {
  to: string
  label: string
  title: string
  description: string
  icon: LucideIcon
}) {
  return (
    <Link to={to} aria-label={label} className="panel group flex min-w-0 items-start gap-4 p-4 transition-colors hover:border-accent active:bg-inset focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent sm:min-h-36 md:p-5">
      <span className="flex size-12 shrink-0 items-center justify-center rounded-lg bg-accent-soft text-accent"><Icon className="size-6" aria-hidden /></span>
      <span className="min-w-0 flex-1">
        <span className="block text-base font-semibold text-ink">{title}</span>
        <span className="mt-2 block text-sm leading-6 text-muted">{description}</span>
      </span>
      <ChevronRight className="size-5 shrink-0 self-center text-subtle group-hover:text-accent" aria-hidden />
    </Link>
  )
}

export default function MePage() {
  const access = useAccess()
  const [sheet, setSheet] = useState<'name' | 'password' | null>(null)
  const [saved, setSaved] = useState(false)
  const qc = useQueryClient()
  const session = useSession().data
  const meta = useQuery({ queryKey: ['meta'], queryFn: api.meta, staleTime: Infinity })
  const logout = useMutation({
    mutationFn: api.logout,
    onSettled: () => setSession(qc, { state: 'out' }),
  })
  if (session?.state !== 'in') return null
  const { user } = session

  return (
    <main className="page-width min-w-0 space-y-6 px-4 pt-4 pb-8 md:px-6">
      <header className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold text-ink">我的</h1>
          <p className="mt-1 text-sm text-muted">账号信息与管理入口</p>
        </div>
        {meta.data && <p className="rounded-md border border-line bg-surface px-3 py-2 text-xs text-muted">版本 {meta.data.version}</p>}
      </header>
      <section aria-labelledby="account-heading" className="panel min-w-0 p-4 md:p-5">
        <div className="flex min-w-0 flex-col gap-6 md:flex-row md:items-center md:justify-between">
          <div className="min-w-0 flex-1">
            <h2 id="account-heading" className="text-sm font-semibold text-body">当前账号</h2>
            <div className="mt-4 flex items-start gap-4">
              <span className="flex size-14 shrink-0 items-center justify-center rounded-xl bg-accent-soft text-accent"><UserRound className="size-7" aria-hidden /></span>
              <div className="min-w-0 flex-1">
                <p className="break-words text-lg font-semibold leading-7 text-ink">{user.displayName || user.username}</p>
                <p className="mt-1 break-words text-sm leading-6 text-muted">
                  {user.username} · {user.role === 'admin' ? '管理员' : '普通账号'}
                </p>
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
      <section aria-labelledby="management-heading" className="min-w-0 space-y-3">
        <h2 id="management-heading" className="text-sm font-semibold text-body">管理入口</h2>
        <div className={`grid min-w-0 gap-3 ${access.admin ? 'sm:grid-cols-2 lg:grid-cols-3' : ''}`}>
          {access.can('read') && <ManagementCard to="/machines" label="机台管理" title="机台" description="管理机台，查看关联图纸和程序" icon={Factory} />}
          {access.admin && <ManagementCard to="/admin" label="账号管理" title="账号管理" description="管理账号，查看操作审计" icon={Users} />}
          {access.admin ? <ManagementCard to="/me/backup" label="备份与迁移" title="备份与迁移" description="创建备份，导入迁移和恢复数据" icon={Archive} /> : <p className="text-sm leading-6 text-muted">整站备份与导入请联系管理员。</p>}
        </div>
      </section>
      <footer className="border-t border-line pt-4">
        <Button variant="secondary" loading={logout.isPending} onClick={() => logout.mutate()} className="w-full sm:w-auto">
          <LogOut className="size-4" aria-hidden />退出登录
        </Button>
      </footer>
      {sheet && <AccountSheet kind={sheet} user={user} onClose={() => setSheet(null)} onSaved={() => { setSheet(null); setSaved(true) }} />}
    </main>
  )
}
