import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, test, vi } from 'vitest'
import App from './App'
import { drawing, mockApi, renderApp, user } from './test-utils'

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
