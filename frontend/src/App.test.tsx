import { act, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, test, vi } from 'vitest'
import App from './App'
import { drawing, mockApi, renderApp, user } from './test-utils'
import { SESSION_KEY } from './auth/session'
import { FORBIDDEN_EVENT, UNAUTHORIZED_EVENT } from './api/client'

afterEach(() => vi.unstubAllGlobals())

test('尚无用户时显示创建管理员表单，并校验密码', async () => {
  const calls = mockApi((r) => (r.path === '/auth/status' ? { body: { setupRequired: true } } : undefined))
  renderApp(<App />)
  expect(await screen.findByText('首次使用：创建管理员账号')).toBeInTheDocument()

  const u = userEvent.setup()
  await u.type(screen.getByLabelText('用户名'), 'admin')
  await u.type(screen.getByLabelText('密码'), 'short')
  await u.click(screen.getByRole('button', { name: '创建并进入' }))
  expect(await screen.findByText('密码至少 8 位')).toBeInTheDocument()

  await u.clear(screen.getByLabelText('密码'))
  await u.type(screen.getByLabelText('密码'), 'password123')
  await u.type(screen.getByLabelText('确认密码'), 'different123')
  await u.click(screen.getByRole('button', { name: '创建并进入' }))
  expect(await screen.findByText('两次输入的密码不一致')).toBeInTheDocument()
  expect(calls.some((c) => c.path === '/auth/setup')).toBe(false)
})

test('未登录显示登录页；密码错误给出提示；登录成功进入总览', async () => {
  let loggedIn = false
  mockApi((r) => {
    if (r.path === '/auth/status') return { body: { setupRequired: false } }
    if (r.path === '/auth/me') return loggedIn ? { body: user } : { status: 401, body: { error: '请先登录' } }
    if (r.path === '/auth/login') {
      const body = r.body as { password: string }
      if (body.password !== 'right-pass') return { status: 401, body: { error: '用户名或密码错误' } }
      loggedIn = true
      return { body: user }
    }
    if (r.path.startsWith('/drawings?')) return { body: { items: [drawing()], total: 1, page: 1, size: 20 } }
  })
  renderApp(<App />)
  const u = userEvent.setup()
  await u.type(await screen.findByLabelText('用户名'), 'admin')
  await u.type(screen.getByLabelText('密码'), 'wrong')
  await u.click(screen.getByRole('button', { name: '登录' }))
  expect(await screen.findByText('用户名或密码错误')).toBeInTheDocument()

  await u.clear(screen.getByLabelText('密码'))
  await u.type(screen.getByLabelText('密码'), 'right-pass')
  await u.click(screen.getByRole('button', { name: '登录' }))
  expect(await screen.findByText('D-1001')).toBeInTheDocument()
  expect(screen.getByRole('heading', { name: '总览' })).toBeInTheDocument()
})

test('已登录直接进入总览，并提供新建图纸入口', async () => {
  mockApi((r) => {
    if (r.path === '/auth/status') return { body: { setupRequired: false } }
    if (r.path === '/auth/me') return { body: user }
    if (r.path.startsWith('/drawings?')) return { body: { items: [], total: 0, page: 1, size: 20 } }
  })
  renderApp(<App />)
  expect(await screen.findByText('还没有图纸')).toBeInTheDocument()
  expect(screen.getAllByRole('link', { name: /新建/ }).length).toBeGreaterThan(0)
})

test('会话过期（接口返回 401）后回到登录页', async () => {
  let expired = false
  mockApi((r) => {
    if (r.path === '/auth/status') return { body: { setupRequired: false } }
    if (r.path === '/auth/me') return expired ? { status: 401, body: { error: 'x' } } : { body: user }
    if (r.path.startsWith('/drawings?')) {
      expired = true
      return { status: 401, body: { error: '登录已失效，请重新登录' } }
    }
  })
  renderApp(<App />)
  await waitFor(() => expect(screen.getByText('登录后查看数控程序台账')).toBeInTheDocument())
})

test('服务器不可用时给出重试入口', async () => {
  vi.stubGlobal('fetch', vi.fn(async () => Promise.reject(new TypeError('network'))))
  renderApp(<App />)
  expect(await screen.findByText('无法连接服务器')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '重试' })).toBeInTheDocument()
})

test('退出登录回到登录页', async () => {
  let out = false
  mockApi((r) => {
    if (r.path === '/auth/status') return { body: { setupRequired: false } }
    if (r.path === '/auth/me') return out ? { status: 401, body: {} } : { body: user }
    if (r.path === '/auth/logout') {
      out = true
      return { status: 204 }
    }
    if (r.path === '/meta') return { body: { version: '2.0.0' } }
  })
  renderApp(<App />, '/me')
  const u = userEvent.setup()
  expect(await screen.findByText('版本 2.0.0')).toBeInTheDocument()
  expect(screen.getByText('admin · 管理员')).toBeInTheDocument()
  await u.click(screen.getByRole('button', { name: '退出登录' }))
  expect(await screen.findByText('登录后查看数控程序台账')).toBeInTheDocument()
})

test('页面外的刀具草稿也会在刷新前提示，退出后清除草稿', async () => {
  let out = false
  mockApi((r) => {
    if (r.path === '/auth/status') return { body: { setupRequired: false } }
    if (r.path === '/auth/me') return out ? { status: 401 } : { body: user }
    if (r.path === '/meta') return { body: { version: '2.1.0-dev' } }
    if (r.path === '/auth/logout') { out = true; return { status: 204 } }
  })
  const { client } = renderApp(<App />, '/me')
  await screen.findByText('版本 2.1.0-dev')
  client.setQueryData(['version-tools-draft', 5], { dirty: true })
  const before = new Event('beforeunload', { cancelable: true })
  window.dispatchEvent(before)
  expect(before.defaultPrevented).toBe(true)
  await userEvent.click(screen.getByRole('button', { name: '退出登录' }))
  await screen.findByText('登录后查看数控程序台账')
  expect(client.getQueryData(['version-tools-draft', 5])).toBeUndefined()
  const after = new Event('beforeunload', { cancelable: true })
  window.dispatchEvent(after)
  expect(after.defaultPrevented).toBe(false)
})

test.each(['/drawings/1', '/drawings/1/programs/1'])('停用账号访问详情 %s 不请求业务数据', async (route) => {
  const member = { ...user, id: 2, role: 'editor', enabled: false }
  const calls = mockApi((r) => {
    if (r.path === '/auth/status') return { body: { setupRequired: false } }
    if (r.path === '/auth/me') return { body: member }
    return { status: 403, body: { error: '没有此操作的权限' } }
  })
  renderApp(<App />, route)
  expect(await screen.findByText('账号不可用，请联系管理员。')).toBeInTheDocument()
  expect(calls.every((r) => r.path.startsWith('/auth/'))).toBe(true)
})

test('权限未变的持续 403 显示拒绝提示，不自动重挂请求循环', async () => {
  const calls = mockApi((r) => {
    if (r.path === '/auth/status') return { body: { setupRequired: false } }
    if (r.path === '/auth/me') return { body: user }
    if (r.path === '/drawings/1/detail') return { status: 403, body: { error: '没有此操作的权限' } }
    if (r.path.startsWith('/drawings?')) return { body: { items: [], total: 0, page: 1, size: 20 } }
  })
  renderApp(<App />, '/drawings/1')
  expect(await screen.findByText('没有此操作的权限，请联系管理员。')).toBeInTheDocument()
  expect(calls.filter((r) => r.path === '/drawings/1/detail')).toHaveLength(1)
  await userEvent.click(screen.getAllByRole('link', { name: '总览' })[0])
  expect(await screen.findByText('还没有图纸')).toBeInTheDocument()
})

test('退出完成后，较早发出的会话刷新响应不能恢复登录', async () => {
  mockApi((r) => {
    if (r.path === '/auth/status') return { body: { setupRequired: false } }
    if (r.path === '/auth/me') return { body: user }
    if (r.path === '/auth/logout') return { status: 204 }
    if (r.path === '/meta') return { body: { version: 'review' } }
  })
  const { client } = renderApp(<App />, '/me')
  await screen.findByText('版本 review')
  const original = globalThis.fetch
  let release!: () => void
  let reached!: () => void
  const gate = new Promise<void>((resolve) => { release = resolve })
  const started = new Promise<void>((resolve) => { reached = resolve })
  let once = true
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const response = await original(input, init)
    if (String(input) === '/api/auth/me' && once) { once = false; reached(); await gate }
    return response
  }))
  let refetch!: Promise<void>
  act(() => { refetch = client.refetchQueries({ queryKey: SESSION_KEY }) })
  await started
  await userEvent.click(screen.getByRole('button', { name: '退出登录' }))
  await screen.findByText('登录后查看数控程序台账')
  await act(async () => { release(); await refetch })
  expect(client.getQueryData(SESSION_KEY)).toEqual({ state: 'out' })
  expect(screen.getByText('登录后查看数控程序台账')).toBeInTheDocument()
})

test('权限拒绝触发的身份检查迟于会话失效返回时，保持退出状态', async () => {
  mockApi((r) => {
    if (r.path === '/auth/status') return { body: { setupRequired: false } }
    if (r.path === '/auth/me') return { body: user }
    if (r.path.startsWith('/drawings?')) return { body: { items: [], total: 0, page: 1, size: 20 } }
  })
  const { client } = renderApp(<App />)
  await screen.findByText('还没有图纸')
  const original = globalThis.fetch
  let release!: (response: Response) => void
  vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => String(input) === '/api/auth/me'
    ? new Promise<Response>((resolve) => { release = resolve }) : original(input, init)))
  act(() => window.dispatchEvent(new Event(FORBIDDEN_EVENT)))
  act(() => window.dispatchEvent(new Event(UNAUTHORIZED_EVENT)))
  await act(async () => { release(new Response(JSON.stringify(user))) })
  expect(await screen.findByText('登录后查看数控程序台账')).toBeInTheDocument()
  expect(client.getQueryData(SESSION_KEY)).toEqual({ state: 'out' })
})

test('轮询确认账号停用后，较早的 403 身份检查不能恢复权限', async () => {
  const member = { ...user, role: 'editor', enabled: false }
  mockApi((r) => {
    if (r.path === '/auth/status') return { body: { setupRequired: false } }
    if (r.path === '/auth/me') return { body: user }
    if (r.path.startsWith('/drawings?')) return { body: { items: [], total: 0, page: 1, size: 20 } }
  })
  const { client } = renderApp(<App />)
  await screen.findByText('还没有图纸')
  const original = globalThis.fetch
  let release!: (response: Response) => void
  let first = true
  vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
    if (String(input) !== '/api/auth/me') return original(input, init)
    if (first) { first = false; return new Promise<Response>((resolve) => { release = resolve }) }
    return Promise.resolve(new Response(JSON.stringify(member)))
  }))
  act(() => window.dispatchEvent(new Event(FORBIDDEN_EVENT)))
  await act(async () => { await client.refetchQueries({ queryKey: SESSION_KEY }) })
  await act(async () => { release(new Response(JSON.stringify(user))) })
  expect(await screen.findByText('账号不可用，请联系管理员。')).toBeInTheDocument()
  expect(client.getQueryData(SESSION_KEY)).toMatchObject({ state: 'in', user: member })
})
