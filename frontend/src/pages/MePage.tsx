import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { useState } from 'react'
import { ChevronRight, Factory } from 'lucide-react'
import { api } from '../api'
import { useAccess } from '../auth/permissions'
import { setSession, useSession } from '../auth/session'
import { Button } from '../components/ui/Button'
import AccountSheet from '../components/AccountSheet'
import BackupSection from '../components/BackupSection'


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
    <main className="mx-auto max-w-xl space-y-4 px-4 pt-4 pb-8">
      <h1 className="text-2xl font-bold">我的</h1>
      <section className="rounded-2xl border border-slate-200 bg-white p-4">
        <p className="break-words text-lg font-bold">{user.displayName || user.username}</p>
        <p className="break-words text-sm text-slate-500">
          {user.username} · {user.role === 'admin' ? '管理员' : '普通账号'}
        </p>
        <div className="mt-4 grid grid-cols-2 gap-3">
          <Button variant="secondary" className="px-3" onClick={() => { setSaved(false); setSheet('name') }}>修改名称</Button>
          <Button variant="secondary" className="px-3" onClick={() => { setSaved(false); setSheet('password') }}>修改密码</Button>
        </div>
      </section>
      {saved && <p role="status" className="rounded-xl bg-green-50 p-3 text-sm text-green-900">名称已保存</p>}
      {access.can('read') && <Link to="/machines" aria-label="机台管理" className="flex items-center gap-3 rounded-2xl border border-slate-200 bg-white p-4 shadow-sm hover:border-amber-400 active:bg-slate-100 focus-visible:outline-2 focus-visible:outline-amber-500">
        <span className="flex size-12 shrink-0 items-center justify-center rounded-xl bg-amber-50 text-amber-700"><Factory className="size-6" aria-hidden /></span>
        <span className="min-w-0 flex-1"><span className="block text-lg font-bold">机台</span><span className="mt-1 block text-sm text-slate-500">管理机台，查看关联图纸和程序</span></span>
        <ChevronRight className="size-5 shrink-0 text-slate-400" aria-hidden />
      </Link>}
      {user.role === 'admin' && <section className="grid gap-3 rounded-2xl border border-slate-200 bg-white p-4"><Link className="rounded-xl bg-slate-100 p-3 font-semibold" to="/admin">账号管理</Link><Link className="rounded-xl bg-slate-100 p-3 font-semibold" to="/admin/audit">操作审计</Link></section>}
      {user.role === 'admin' ? <BackupSection /> : <p className="text-sm text-slate-500">整站备份与导入请联系管理员。</p>}
      <Button variant="secondary" loading={logout.isPending} onClick={() => logout.mutate()} className="w-full">
        退出登录
      </Button>
      {meta.data && <p className="text-center text-sm text-slate-400">版本 {meta.data.version}</p>}
      {sheet && <AccountSheet kind={sheet} user={user} onClose={() => setSheet(null)} onSaved={() => { setSheet(null); setSaved(true) }} />}
    </main>
  )
}
