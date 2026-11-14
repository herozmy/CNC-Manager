import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { api } from '../api'
import { setSession, useSession } from '../auth/session'
import { Button } from '../components/ui/Button'
import AccountSheet from '../components/AccountSheet'
import BackupSection from '../components/BackupSection'

const roleName = { admin: '管理员', editor: '编辑', viewer: '只读' } as const

export default function MePage() {
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
          {user.username} · {roleName[user.role]}
        </p>
        <div className="mt-4 grid grid-cols-2 gap-3">
          <Button variant="secondary" className="px-3" onClick={() => { setSaved(false); setSheet('name') }}>修改名称</Button>
          <Button variant="secondary" className="px-3" onClick={() => { setSaved(false); setSheet('password') }}>修改密码</Button>
        </div>
      </section>
      {saved && <p role="status" className="rounded-xl bg-green-50 p-3 text-sm text-green-900">名称已保存</p>}
      {user.role === 'admin' ? <BackupSection /> : <p className="text-sm text-slate-500">整站备份与导入请联系管理员。</p>}
      <Button variant="secondary" loading={logout.isPending} onClick={() => logout.mutate()} className="w-full">
        退出登录
      </Button>
      {meta.data && <p className="text-center text-sm text-slate-400">版本 {meta.data.version}</p>}
      {sheet && <AccountSheet kind={sheet} user={user} onClose={() => setSheet(null)} onSaved={() => { setSheet(null); setSaved(true) }} />}
    </main>
  )
}
