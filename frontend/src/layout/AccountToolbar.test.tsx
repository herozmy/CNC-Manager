import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, test, vi } from 'vitest'
import App from '../App'
import { mockApi, renderApp, user } from '../test-utils'

function server() {
  return mockApi(r => {
    if (r.path === '/auth/status') return { body: { setupRequired: false } }
    if (r.path === '/auth/me') return { body: user }
    if (r.path === '/meta') return { body: { version: 'local-test' } }
    if (r.path === '/admin/users') return { body: [user] }
  })
}
afterEach(() => vi.unstubAllGlobals())

test('头像菜单按需打开，再次点击、外部点击和Tab移出时收起', async () => {
  const calls = server()
  renderApp(<App />, '/me')
  const u = userEvent.setup()
  const trigger = await screen.findByRole('button', { name: '账号菜单' })
  expect(trigger).toHaveAttribute('aria-expanded', 'false')
  expect(screen.queryByRole('menu')).not.toBeInTheDocument()
  await u.click(trigger)
  expect(trigger).toHaveAttribute('aria-expanded', 'true')
  const menu = screen.getByRole('menu', { name: '账号菜单' })
  expect(within(menu).getAllByRole('menuitem')).toHaveLength(2)
  expect(within(menu).getByRole('menuitem', { name: '账号管理' })).toHaveFocus()
  expect(within(screen.getByRole('banner', { name: '顶部工具栏' })).getByText('管理员')).toBeVisible()
  await u.click(trigger)
  expect(screen.queryByRole('menu')).not.toBeInTheDocument()
  await u.click(trigger)
  await u.click(screen.getByRole('heading', { name: '我的' }))
  expect(screen.queryByRole('menu')).not.toBeInTheDocument()
  await u.click(trigger)
  await u.tab()
  expect(screen.queryByRole('menu')).not.toBeInTheDocument()
  expect(calls.filter(r => r.method !== 'GET')).toHaveLength(0)
})

test('菜单支持方向键循环、Home/End和Escape恢复头像焦点', async () => {
  server()
  renderApp(<App />, '/me')
  const u = userEvent.setup()
  const trigger = await screen.findByRole('button', { name: '账号菜单' })
  trigger.focus()
  await u.keyboard('{ArrowUp}')
  expect(screen.getByRole('menuitem', { name: '退出登录' })).toHaveFocus()
  await u.keyboard('{ArrowDown}')
  expect(screen.getByRole('menuitem', { name: '账号管理' })).toHaveFocus()
  await u.keyboard('{End}')
  expect(screen.getByRole('menuitem', { name: '退出登录' })).toHaveFocus()
  await u.keyboard('{Home}')
  expect(screen.getByRole('menuitem', { name: '账号管理' })).toHaveFocus()
  await u.keyboard('{Escape}')
  expect(screen.queryByRole('menu')).not.toBeInTheDocument()
  expect(trigger).toHaveFocus()
  await u.keyboard('{ArrowDown}{Enter}')
  expect(await screen.findByRole('heading', { name: '账号管理', level: 1 })).toBeVisible()
  expect(screen.queryByRole('menu')).not.toBeInTheDocument()
})

test('退出在头像菜单内可用，请求中阻止重复提交，网络失败也清除本机会话', async () => {
  const calls = server()
  const { client } = renderApp(<App />, '/me')
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '账号菜单' }))
  client.setQueryData(['drawing', 1], { secret: true })
  let reject!: (reason: Error) => void
  const original = fetch
  const request = vi.fn(() => new Promise<Response>((_, fail) => { reject = fail }))
  vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => String(input).endsWith('/auth/logout') ? request() : original(input, init)))
  await u.click(screen.getByRole('menuitem', { name: '退出登录' }))
  const pending = await screen.findByRole('menuitem', { name: '正在退出…' })
  expect(pending).toHaveAttribute('aria-disabled', 'true')
  await u.click(pending)
  expect(request).toHaveBeenCalledOnce()
  await act(async () => reject(new TypeError('offline')))
  await waitFor(() => expect(screen.getByRole('button', { name: '登录' })).toBeVisible())
  expect(client.getQueryData(['drawing', 1])).toBeUndefined()
  expect(calls.some(r => r.path.startsWith('/admin'))).toBe(false)
})
