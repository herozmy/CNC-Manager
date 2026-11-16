import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { ArrowLeft } from 'lucide-react'
import type { FormEvent } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api'
import type { User, UserInput } from '../api/types'
import { useAccess } from '../auth/permissions'
import { setSession } from '../auth/session'
import { Button } from '../components/ui/Button'
import { Field } from '../components/ui/Field'
import { Sheet } from '../components/ui/Sheet'
import { ErrorState, Spinner } from '../components/ui/State'

const emptyUser: UserInput = { username: '', displayName: '', password: '', role: 'editor' }
const selectClass = 'h-12 w-full rounded-md border border-line-strong bg-surface px-3'

export default function AdminPage() {
  const access = useAccess()
  const qc = useQueryClient()
  const users = useQuery({ queryKey: ['admin-users'], queryFn: api.users, enabled: access.admin })
  const [user, setUser] = useState<User | 'new' | null>(null)
  const [userForm, setUserForm] = useState<UserInput & { enabled: boolean }>({ ...emptyUser, enabled: true })
  const [reset, setReset] = useState<User | null>(null)
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  if (!access.admin) return <main className="p-6">仅系统管理员可访问账号管理。</main>

  async function save(e: FormEvent, action: () => Promise<unknown>, close: () => void) {
    e.preventDefault()
    if (busy) return
    setBusy(true); setError('')
    try {
      await action()
      close(); setNotice('保存成功')
      await Promise.all([qc.invalidateQueries({ queryKey: ['admin-users'] }), qc.invalidateQueries({ queryKey: ['drawings'] })])
    } catch (err) { setError(err instanceof Error ? err.message : '保存失败，请重试') }
    finally { setBusy(false) }
  }
  function openUser(value: User | 'new') { setError(''); setNotice(''); setUser(value); setUserForm(value === 'new' ? { ...emptyUser, enabled: true } : { username: value.username, displayName: value.displayName, password: '', role: value.role === 'admin' ? 'admin' : 'editor', enabled: value.enabled }) }

  return <main className="mx-auto max-w-3xl space-y-6 px-4 py-5">
    <header className="space-y-2"><Link to="/me" className="inline-flex min-h-12 items-center gap-2 rounded-md px-2 text-sm text-muted hover:bg-hover"><ArrowLeft className="size-5" aria-hidden />返回我的</Link><h1 className="text-2xl font-bold">账号管理</h1><p className="text-sm text-muted">普通账号只管理自己的数据，管理员可查看全部数据。</p><Link className="inline-flex min-h-12 items-center rounded-md px-2 font-medium text-warning hover:bg-warning-soft" to="/admin/audit">查看操作日志 →</Link></header>
    {notice && <p role="status" className="rounded-md bg-success-soft p-3 text-success">{notice}</p>}
    {users.isPending && <Spinner />}
    {users.isError && <ErrorState message="管理数据加载失败" onRetry={() => { void users.refetch() }} />}
    <section className="space-y-3"><div className="flex items-center justify-between"><h2 className="text-xl font-bold">账号</h2><Button onClick={() => openUser('new')}>新增账号</Button></div>
      {users.data?.map((u) => <article key={u.id} className="space-y-3 rounded-lg border border-line bg-surface p-4"><div><h3 className="break-words font-bold">{u.displayName} <span className="font-normal text-muted">{u.username}</span></h3><p className="mt-1 text-sm">{u.role === 'admin' ? '系统管理员' : '普通账号'} · {u.enabled ? '已启用' : '已停用'}</p></div><div className="flex flex-wrap gap-2"><Button variant="secondary" aria-label={`编辑账号 ${u.username}`} onClick={() => openUser(u)}>编辑账号</Button><Button variant="ghost" aria-label={`重置密码 ${u.username}`} onClick={() => { setError(''); setPassword(''); setReset(u) }}>重置密码</Button></div></article>)}
    </section>
    {user && <Sheet open title={user === 'new' ? '新增账号' : '编辑账号'} onClose={() => { if (!busy) setUser(null) }}><form className="space-y-4" onSubmit={(e) => void save(e, async () => {
      if (user === 'new') return api.createUser(userForm)
      const result = await api.updateUser(user.id, { displayName: userForm.displayName, role: userForm.role, enabled: userForm.enabled })
      if (user.id === access.user?.id) setSession(qc, { state: 'out', notice: '账号信息已更新，请重新登录。' })
      return result
    }, () => setUser(null))}><fieldset disabled={busy} className="space-y-4"><Field label="用户名" required disabled={user !== 'new'} maxLength={64} value={userForm.username} onChange={(e) => setUserForm({ ...userForm, username: e.target.value })} /><Field label="显示名称" required maxLength={64} value={userForm.displayName} onChange={(e) => setUserForm({ ...userForm, displayName: e.target.value })} />{user === 'new' && <Field label="初始密码" required type="password" autoComplete="new-password" minLength={8} maxLength={128} value={userForm.password} onChange={(e) => setUserForm({ ...userForm, password: e.target.value })} />}<label className="block space-y-2">账号身份<select aria-label="账号身份" className={selectClass} value={userForm.role} onChange={(e) => setUserForm({ ...userForm, role: e.target.value as 'admin' | 'editor' })}><option value="editor">普通账号</option><option value="admin">系统管理员</option></select></label>{user !== 'new' && <label className="flex min-h-12 items-center gap-3"><input type="checkbox" checked={userForm.enabled} onChange={(e) => setUserForm({ ...userForm, enabled: e.target.checked })} />启用账号</label>}<p className="text-sm text-muted">更换身份不改变数据归属。系统至少保留一个启用的管理员。</p></fieldset>{error && <p role="alert" className="text-danger">{error}</p>}<Button type="submit" className="w-full" loading={busy}>保存账号</Button></form></Sheet>}
    {reset && <Sheet open title={`重置密码：${reset.username}`} onClose={() => { if (!busy) setReset(null) }}><form className="space-y-4" onSubmit={(e) => void save(e, async () => { await api.resetPassword(reset.id, password); if (reset.id === access.user?.id) setSession(qc, { state: 'out', notice: '密码已重置，请重新登录。' }) }, () => { setReset(null); setPassword('') })}><p>重置后该账号所有设备需重新登录。</p><Field label="新密码" required type="password" minLength={8} maxLength={128} autoComplete="new-password" value={password} onChange={(e) => setPassword(e.target.value)} />{error && <p role="alert" className="text-danger">{error}</p>}<Button type="submit" loading={busy}>确认重置密码</Button></form></Sheet>}
  </main>
}
