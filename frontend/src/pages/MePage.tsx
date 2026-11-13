import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '../api'
import { setSession, useSession } from '../auth/session'
import { Button } from '../components/ui/Button'

const roleName = { admin: '管理员', editor: '编辑', viewer: '只读' } as const

export default function MePage() {
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
        <p className="text-lg font-bold">{user.displayName || user.username}</p>
        <p className="text-sm text-slate-500">
          {user.username} · {roleName[user.role]}
        </p>
      </section>
      <Button variant="secondary" loading={logout.isPending} onClick={() => logout.mutate()} className="w-full">
        退出登录
      </Button>
      {meta.data && <p className="text-center text-sm text-slate-400">版本 {meta.data.version}</p>}
    </main>
  )
}
