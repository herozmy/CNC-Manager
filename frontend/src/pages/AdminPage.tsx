import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { ArrowLeft, Check, History, KeyRound, Pencil, Plus, ShieldCheck, UserRound } from 'lucide-react'
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
import AccountToolbar from '../layout/AccountToolbar'
import CurrentAccount from '../components/CurrentAccount'

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
  const otherUsers = users.data?.filter(u => u.id !== access.user?.id)
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

  return <main className="page-width min-w-0 space-y-4 px-4 pt-3 pb-8 md:px-6">
    <header className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 lg:grid-cols-[minmax(0,1fr)_auto_auto]">
      <div className="flex min-w-0 items-center gap-3">
        <Link to="/me" aria-label="返回我的" title="返回我的" className="flex size-12 shrink-0 items-center justify-center rounded-lg text-muted hover:bg-hover focus-visible:outline-2 focus-visible:outline-accent"><ArrowLeft className="size-5" aria-hidden /></Link>
        <div className="min-w-0"><h1 className="text-xl font-semibold text-ink">账号管理</h1><p className="mt-1 hidden text-sm text-muted md:block">个人设置与其他账号统一管理</p></div>
      </div>
      <div className="order-3 col-span-2 flex flex-wrap gap-2 lg:order-2 lg:col-span-1">
        <Link aria-label="查看操作日志 →" className="inline-flex min-h-12 items-center justify-center gap-2 rounded-md border border-line-strong bg-surface px-4 text-sm font-medium text-body hover:bg-hover focus-visible:outline-2 focus-visible:outline-accent" to="/admin/audit"><History className="size-4" aria-hidden />操作日志</Link>
        <Button onClick={() => openUser('new')}><Plus className="size-4" aria-hidden />新增账号</Button>
      </div>
      <AccountToolbar className="order-2 lg:order-3" />
    </header>
    <CurrentAccount><Button variant="ghost" onClick={() => { if (access.user) openUser(access.user) }}>账号身份</Button></CurrentAccount>
    <div className="flex items-start gap-2 rounded-lg border border-line bg-inset px-4 py-3 text-sm leading-6 text-muted"><ShieldCheck className="mt-0.5 size-5 shrink-0 text-accent" aria-hidden /><p>普通账号只管理自己的数据，管理员可查看全部数据。</p></div>
    {notice && <p role="status" className="flex items-center gap-2 rounded-md bg-success-soft p-3 text-sm text-success"><Check className="size-4 shrink-0" aria-hidden />{notice}</p>}
    <section aria-labelledby="accounts-heading" className="panel min-w-0 overflow-hidden">
      <div className="flex flex-wrap items-center justify-between gap-2 border-b border-line px-4 py-4 md:px-5">
        <h2 id="accounts-heading" className="text-base font-semibold text-ink">其他账号</h2>
        {users.isSuccess && <p className="text-sm text-muted">共 {otherUsers?.length} 个账号<span className="mx-2 text-subtle" aria-hidden>·</span>{otherUsers?.filter(u => u.enabled).length} 个已启用</p>}
      </div>
      {users.isPending && <Spinner label="正在加载账号…" />}
      {users.isError && <ErrorState message="管理数据加载失败" onRetry={() => { void users.refetch() }} />}
      {users.isSuccess && otherUsers?.length === 0 && <p className="px-4 py-12 text-center text-sm text-muted">暂无其他账号，可点击“新增账号”创建。</p>}
      {!!otherUsers?.length && <div aria-hidden className="hidden grid-cols-[minmax(0,1fr)_8rem_6rem_16rem] items-center gap-4 border-b border-line bg-inset px-5 py-3 text-xs font-medium text-muted xl:grid"><span>账号 / 用户名</span><span>账号身份</span><span>启用状态</span><span className="text-right">账号操作</span></div>}
      <ul aria-label="账号列表" className="divide-y divide-line">
        {otherUsers?.map((u) => <li key={u.id}>
          <article aria-label={`账号 ${u.username}`} className="grid min-w-0 grid-cols-1 items-center gap-4 p-4 md:p-5 xl:grid-cols-[minmax(0,1fr)_8rem_6rem_16rem]">
            <div className="flex min-w-0 items-start gap-3">
              <span className={`flex size-10 shrink-0 items-center justify-center rounded-lg ${u.role === 'admin' ? 'bg-accent-soft text-accent' : 'bg-inset text-muted'}`}><UserRound className="size-5" aria-hidden /></span>
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-x-2 gap-y-1"><h3 className="min-w-0 break-all text-sm font-semibold text-ink">{u.displayName || u.username}</h3></div>
                <p className="mt-1 break-all font-mono text-xs text-muted">{u.username}</p>
              </div>
            </div>
            <div className="flex flex-wrap items-center gap-3 xl:contents">
              <span className={`inline-flex items-center gap-1.5 text-sm ${u.role === 'admin' ? 'text-accent' : 'text-body'}`}>{u.role === 'admin' && <ShieldCheck className="size-4 shrink-0" aria-hidden />}{u.role === 'admin' ? '系统管理员' : '普通账号'}</span>
              <span className={`inline-flex w-fit items-center gap-1.5 rounded-md px-2 py-1 text-xs font-medium ${u.enabled ? 'bg-success-soft text-success' : 'bg-inset text-muted'}`}><span className={`size-1.5 rounded-full ${u.enabled ? 'bg-success' : 'bg-subtle'}`} aria-hidden />{u.enabled ? '已启用' : '已停用'}</span>
            </div>
            <div className="flex flex-wrap gap-2 border-t border-line pt-3 xl:justify-end xl:border-0 xl:pt-0">
              <Button variant="secondary" aria-label={`编辑账号 ${u.username}`} onClick={() => openUser(u)}><Pencil className="size-4" aria-hidden />编辑账号</Button>
              <Button variant="ghost" aria-label={`重置密码 ${u.username}`} onClick={() => { setError(''); setPassword(''); setReset(u) }}><KeyRound className="size-4" aria-hidden />重置密码</Button>
            </div>
          </article>
        </li>)}
      </ul>
    </section>
    {user && <Sheet open title={user === 'new' ? '新增账号' : user.id === access.user?.id ? '设置账号身份' : '编辑账号'} onClose={() => { if (!busy) setUser(null) }}><form className="space-y-4" onSubmit={(e) => void save(e, async () => {
      if (user === 'new') return api.createUser(userForm)
      const result = await api.updateUser(user.id, { displayName: user.id === access.user?.id ? access.user.displayName : userForm.displayName, role: userForm.role, enabled: userForm.enabled })
      if (user.id === access.user?.id) setSession(qc, { state: 'out', notice: '账号信息已更新，请重新登录。' })
      return result
    }, () => setUser(null))}><fieldset disabled={busy} className="space-y-4"><Field label="用户名" required disabled={user !== 'new'} maxLength={64} value={userForm.username} onChange={(e) => setUserForm({ ...userForm, username: e.target.value })} />{(user === 'new' || user.id !== access.user?.id) && <Field label="显示名称" required maxLength={64} value={userForm.displayName} onChange={(e) => setUserForm({ ...userForm, displayName: e.target.value })} />}{user === 'new' && <Field label="初始密码" required type="password" autoComplete="new-password" minLength={8} maxLength={128} value={userForm.password} onChange={(e) => setUserForm({ ...userForm, password: e.target.value })} />}<label className="block space-y-2">账号身份<select aria-label="账号身份" className={selectClass} value={userForm.role} onChange={(e) => setUserForm({ ...userForm, role: e.target.value as 'admin' | 'editor' })}><option value="editor">普通账号</option><option value="admin">系统管理员</option></select></label>{user !== 'new' && <label className="flex min-h-12 items-center gap-3"><input type="checkbox" checked={userForm.enabled} onChange={(e) => setUserForm({ ...userForm, enabled: e.target.checked })} />启用账号</label>}<p className="text-sm text-muted">更换身份不改变数据归属。系统至少保留一个启用的管理员。</p></fieldset>{error && <p role="alert" className="text-danger">{error}</p>}<Button type="submit" className="w-full" loading={busy}>保存账号</Button></form></Sheet>}
    {reset && <Sheet open title={`重置密码：${reset.username}`} onClose={() => { if (!busy) setReset(null) }}><form className="space-y-4" onSubmit={(e) => void save(e, async () => { await api.resetPassword(reset.id, password) }, () => { setReset(null); setPassword('') })}><p>重置后该账号所有设备需重新登录。</p><Field label="新密码" required type="password" minLength={8} maxLength={128} autoComplete="new-password" value={password} onChange={(e) => setPassword(e.target.value)} />{error && <p role="alert" className="text-danger">{error}</p>}<Button type="submit" loading={busy}>确认重置密码</Button></form></Sheet>}
  </main>
}
