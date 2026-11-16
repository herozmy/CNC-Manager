import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { useState } from 'react'
import { Archive, ChevronRight, Factory, Users } from 'lucide-react'
import { api } from '../api'
import { useAccess } from '../auth/permissions'
import { setSession, useSession } from '../auth/session'
import { Button } from '../components/ui/Button'
import AccountSheet from '../components/AccountSheet'


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
    <main className="mx-auto max-w-2xl space-y-4 px-4 pt-4 pb-8">
      <h1 className="text-xl font-semibold">我的</h1>
      <section className="rounded-lg border border-line bg-surface p-4">
        <p className="break-words text-base font-semibold">{user.displayName || user.username}</p>
        <p className="break-words text-sm text-muted">
          {user.username} · {user.role === 'admin' ? '管理员' : '普通账号'}
        </p>
        <div className="mt-4 grid grid-cols-2 gap-3">
          <Button variant="secondary" className="px-3" onClick={() => { setSaved(false); setSheet('name') }}>修改名称</Button>
          <Button variant="secondary" className="px-3" onClick={() => { setSaved(false); setSheet('password') }}>修改密码</Button>
        </div>
      </section>
      {saved && <p role="status" className="rounded-md bg-success-soft p-3 text-sm text-success">名称已保存</p>}
      {access.can('read') && <Link to="/machines" aria-label="机台管理" className="flex items-center gap-3 rounded-lg border border-line bg-surface p-4 shadow-sm hover:border-accent active:bg-inset focus-visible:outline-2 focus-visible:outline-accent">
        <span className="flex size-12 shrink-0 items-center justify-center rounded-md bg-warning-soft text-warning"><Factory className="size-6" aria-hidden /></span>
        <span className="min-w-0 flex-1"><span className="block text-base font-semibold">机台</span><span className="mt-1 block text-sm text-muted">管理机台，查看关联图纸和程序</span></span>
        <ChevronRight className="size-5 shrink-0 text-subtle" aria-hidden />
      </Link>}
      {access.admin && <Link to="/admin" aria-label="账号管理" className="flex items-center gap-3 rounded-lg border border-line bg-surface p-4 shadow-sm hover:border-accent active:bg-inset focus-visible:outline-2 focus-visible:outline-accent">
        <span className="flex size-12 shrink-0 items-center justify-center rounded-md bg-warning-soft text-warning"><Users className="size-6" aria-hidden /></span>
        <span className="min-w-0 flex-1"><span className="block text-base font-semibold">账号管理</span><span className="mt-1 block text-sm text-muted">管理账号，查看操作审计</span></span>
        <ChevronRight className="size-5 shrink-0 text-subtle" aria-hidden />
      </Link>}
      {access.admin ? <Link to="/me/backup" aria-label="备份与迁移" className="flex items-center gap-3 rounded-lg border border-line bg-surface p-4 shadow-sm hover:border-accent active:bg-inset focus-visible:outline-2 focus-visible:outline-accent">
        <span className="flex size-12 shrink-0 items-center justify-center rounded-md bg-warning-soft text-warning"><Archive className="size-6" aria-hidden /></span>
        <span className="min-w-0 flex-1"><span className="block text-base font-semibold">备份与迁移</span><span className="mt-1 block text-sm text-muted">创建备份，导入迁移和恢复数据</span></span>
        <ChevronRight className="size-5 shrink-0 text-subtle" aria-hidden />
      </Link> : <p className="text-sm text-muted">整站备份与导入请联系管理员。</p>}
      <Button variant="secondary" loading={logout.isPending} onClick={() => logout.mutate()} className="w-full">
        退出登录
      </Button>
      {meta.data && <p className="text-center text-sm text-subtle">版本 {meta.data.version}</p>}
      {sheet && <AccountSheet kind={sheet} user={user} onClose={() => setSheet(null)} onSaved={() => { setSheet(null); setSaved(true) }} />}
    </main>
  )
}
