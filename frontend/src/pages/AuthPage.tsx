import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import type { FormEvent } from 'react'
import { api } from '../api'
import { ApiError } from '../api/client'
import { setSession } from '../auth/session'
import { Button } from '../components/ui/Button'
import { Field } from '../components/ui/Field'

/** 首次启动创建管理员，其余时候登录。 */
export default function AuthPage({ setup }: { setup: boolean }) {
  const qc = useQueryClient()
  const [username, setUsername] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function submit(e: FormEvent) {
    e.preventDefault()
    setError('')
    if (!username.trim() || !password) return setError('请输入用户名和密码')
    if (setup) {
      if (password.length < 8) return setError('密码至少 8 位')
      if (password !== confirm) return setError('两次输入的密码不一致')
    }
    setBusy(true)
    try {
      const user = setup ? await api.setup(username.trim(), displayName.trim(), password) : await api.login(username.trim(), password)
      setSession(qc, { state: 'in', user }) // 换了账号，旧缓存一律丢弃
    } catch (err) {
      setError(err instanceof ApiError ? err.message : '无法连接服务器')
    } finally {
      setBusy(false)
    }
  }

  return (
    <main className="flex min-h-dvh items-center justify-center bg-slate-900 p-4">
      <form onSubmit={submit} className="w-full max-w-sm space-y-5 rounded-2xl bg-white p-6 shadow-xl" noValidate>
        <div>
          <h1 className="text-2xl font-bold text-slate-900">CNC-Manager</h1>
          <p className="mt-1 text-slate-600">{setup ? '首次使用：创建管理员账号' : '登录后查看数控程序台账'}</p>
        </div>
        <Field label="用户名" value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" autoCapitalize="none" autoFocus />
        {setup && <Field label="显示名称（可选）" value={displayName} onChange={(e) => setDisplayName(e.target.value)} />}
        <Field
          label="密码"
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          autoComplete={setup ? 'new-password' : 'current-password'}
          hint={setup ? '至少 8 位' : undefined}
        />
        {setup && <Field label="确认密码" type="password" value={confirm} onChange={(e) => setConfirm(e.target.value)} autoComplete="new-password" />}
        {error && (
          <p role="alert" className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">
            {error}
          </p>
        )}
        <Button type="submit" loading={busy} className="w-full">
          {setup ? '创建并进入' : '登录'}
        </Button>
      </form>
    </main>
  )
}
