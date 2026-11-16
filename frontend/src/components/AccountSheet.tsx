import { useQueryClient } from '@tanstack/react-query'
import { useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { api } from '../api'
import { ApiError } from '../api/client'
import type { User } from '../api/types'
import { SESSION_KEY, setSession } from '../auth/session'
import { Button } from './ui/Button'
import { Field } from './ui/Field'
import { Sheet } from './ui/Sheet'

export default function AccountSheet({ kind, user, onClose, onSaved }: {
  kind: 'name' | 'password'; user: User; onClose: () => void; onSaved: () => void
}) {
  const qc = useQueryClient()
  const [name, setName] = useState(user.displayName || user.username)
  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const pending = useRef(false)
  const password = kind === 'password'
  const dirtyTools = qc.getQueriesData<{ dirty: boolean }>({ queryKey: ['version-tools-draft'] }).some(([, draft]) => draft?.dirty)

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (pending.current) return
    setError('')
    if (!password && (!name.trim() || Array.from(name.trim()).length > 64)) return setError('显示名称应为 1~64 个字符')
    if (password) {
      if (!current) return setError('请输入原密码')
      if (Array.from(next).length < 8 || Array.from(next).length > 128) return setError('新密码应为 8~128 个字符')
      if (next !== confirm) return setError('两次输入的新密码不一致')
      if (current === next) return setError('新密码不能与原密码相同')
      if (dirtyTools) return setError('请先保存或撤销未保存的刀具表修改，再修改密码')
    }
    pending.current = true
    setBusy(true)
    try {
      if (password) {
        await api.changePassword(current, next)
        await qc.cancelQueries()
        setSession(qc, { state: 'out', username: user.username, notice: '密码已修改，请使用新密码登录' })
      } else {
        const updated = await api.updateMe(name.trim())
        // Updating a name must not discard unsaved tool drafts or other data.
        qc.setQueryData(SESSION_KEY, { state: 'in', user: updated })
        onSaved()
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : '无法连接服务器，请稍后重试')
    } finally {
      pending.current = false
      setBusy(false)
    }
  }

  return <Sheet open title={password ? '修改密码' : '修改名称'} onClose={() => { if (!pending.current) onClose() }}>
    <form onSubmit={submit} noValidate className="space-y-4">
      {password ? <>
        <p className="text-sm leading-6 text-muted">修改后所有设备需要重新登录，请先保存未完成的编辑。</p>
        <input type="text" name="username" autoComplete="username" value={user.username} readOnly hidden />
        <Field label="原密码" type="password" autoComplete="current-password" value={current} onChange={(e) => setCurrent(e.target.value)} disabled={busy} autoFocus />
        <Field label="新密码" type="password" autoComplete="new-password" hint="8~128 个字符" value={next} onChange={(e) => setNext(e.target.value)} disabled={busy} />
        <Field label="确认新密码" type="password" autoComplete="new-password" value={confirm} onChange={(e) => setConfirm(e.target.value)} disabled={busy} />
        {dirtyTools && <p className="rounded-md bg-warning-soft p-3 text-sm text-warning">请先保存或撤销未保存的刀具表修改，再修改密码。</p>}
      </> : <>
        <p className="text-sm text-muted">登录用户名：{user.username}</p>
        <Field label="显示名称" value={name} onChange={(e) => setName(e.target.value)} hint="1~64 个字符，修改后登录用户名保持不变" disabled={busy} autoFocus />
      </>}
      {error && <p role="alert" className="rounded-md bg-danger-soft p-3 text-sm text-danger">{error}</p>}
      <div className="grid grid-cols-2 gap-3">
        <Button variant="secondary" onClick={onClose} disabled={busy}>取消</Button>
        <Button type="submit" loading={busy} disabled={password && dirtyTools}>{password ? '确认修改' : '保存名称'}</Button>
      </div>
    </form>
  </Sheet>
}
