import { act, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, test, vi } from 'vitest'
import App from '../App'
import { mockApi, renderApp, user } from '../test-utils'
import AuthPage from './AuthPage'

afterEach(() => vi.unstubAllGlobals())

function server() {
  let currentUser = { ...user }
  let failName = true
  const calls = mockApi((r) => {
    if (r.path === '/admin/users') return { body: [user] }
    if (r.path === '/auth/status') return { body: { setupRequired: false } }
    if (r.path === '/auth/me' && r.method === 'GET') return { body: currentUser }
    if (r.path === '/meta') return { body: { version: '2.1.0-dev' } }
    if (r.path === '/auth/me' && r.method === 'PUT') {
      if (failName) { failName = false; return { status: 500, body: { error: '保存失败' } } }
      currentUser = { ...currentUser, ...r.body as object }
      return { body: currentUser }
    }
    if (r.path === '/auth/password') return (r.body as { currentPassword: string }).currentPassword === 'old-password'
      ? { status: 204 } : { status: 400, body: { error: '原密码不正确' } }
  })
  return calls
}

test.each([
  user,
  { ...user, role: 'editor' },
  { ...user, enabled: false },
])('$role启用状态$enabled的当前账号只显示入口，进入后才显示操作，返回后收起', async currentUser => {
  const calls = mockApi(r => {
    if (r.path === '/admin/users') return { body: [user] }
    if (r.path === '/auth/status') return { body: { setupRequired: false } }
    if (r.path === '/auth/me') return { body: currentUser }
    if (r.path === '/meta') return { body: { version: 'local-test' } }
  })
  renderApp(<App />, '/me')
  const u = userEvent.setup()
  const entry = await screen.findByRole('link', { name: '账号管理' })
  expect(entry).toHaveAttribute('href', '/me/account')
  expect(entry).toHaveTextContent(currentUser.displayName)
  expect(screen.queryByRole('button', { name: '修改名称' })).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '修改密码' })).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '退出登录' })).not.toBeInTheDocument()
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  await u.click(entry)
  expect(await screen.findByRole('heading', { name: '账号管理', level: 1 })).toBeVisible()
  expect(screen.getByRole('button', { name: '修改名称' })).toBeEnabled()
  expect(screen.getByRole('button', { name: '修改密码' })).toBeEnabled()
  expect(screen.queryByRole('button', { name: '退出登录' })).not.toBeInTheDocument()
  await u.click(screen.getByRole('button', { name: '账号菜单' }))
  expect(screen.getByRole('menuitem', { name: '退出登录' })).toBeEnabled()
  await u.keyboard('{Escape}')
  expect(screen.getAllByRole('link', { name: '高级管理' }).every(link => link.getAttribute('aria-current') === 'page')).toBe(true)
  await u.click(screen.getByRole('link', { name: '返回高级管理' }))
  expect(await screen.findByRole('link', { name: '账号管理' })).toBeVisible()
  expect(screen.queryByRole('button', { name: '修改名称' })).not.toBeInTheDocument()
  expect(calls.filter(r => /^(\/backup|\/machines)/.test(r.path))).toHaveLength(0)
  if (currentUser.role !== 'admin' || !currentUser.enabled) expect(calls.filter(r => r.path.startsWith('/admin'))).toHaveLength(0)
  expect(calls.filter(r => r.method !== 'GET')).toHaveLength(0)
})

test('未登录直达当前账号页仍进入登录流程', async () => {
  mockApi(r => {
    if (r.path === '/admin/users') return { body: [user] }
    if (r.path === '/auth/status') return { body: { setupRequired: false } }
    if (r.path === '/auth/me') return { status: 401 }
  })
  renderApp(<App />, '/me/account')
  expect(await screen.findByRole('button', { name: '登录' })).toBeVisible()
  expect(screen.queryByRole('button', { name: '修改名称' })).not.toBeInTheDocument()
})

test('未设置显示名称时回退用户名，取消名称编辑不提交也不改变账号信息', async () => {
  const calls = mockApi(r => {
    if (r.path === '/admin/users') return { body: [user] }
    if (r.path === '/auth/status') return { body: { setupRequired: false } }
    if (r.path === '/auth/me') return { body: { ...user, displayName: '' } }
    if (r.path === '/meta') return { body: { version: 'local-test' } }
  })
  renderApp(<App />, '/me/account')
  const u = userEvent.setup()
  const account = await screen.findByRole('region', { name: '当前账号' })
  expect(account).toHaveTextContent('admin · 管理员')
  await u.click(screen.getByRole('button', { name: '修改名称' }))
  expect(screen.getByLabelText('显示名称')).toHaveValue('admin')
  await u.clear(screen.getByLabelText('显示名称'))
  await u.type(screen.getByLabelText('显示名称'), '未保存的名称')
  await u.click(screen.getByRole('button', { name: '取消' }))
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  expect(account).toHaveTextContent('admin')
  expect(account).not.toHaveTextContent('未保存的名称')
  expect(screen.queryByText('名称已保存')).not.toBeInTheDocument()
  expect(calls.filter(r => r.method === 'PUT')).toHaveLength(0)
})

test('名称回填、非空校验、失败保留后重试，保存只更新显示名称并保留草稿', async () => {
  const calls = server()
  const { client } = renderApp(<App />, '/me/account')
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '修改名称' }))
  expect(screen.getByLabelText('显示名称')).toHaveValue('管理员')
  await u.clear(screen.getByLabelText('显示名称'))
  await u.click(screen.getByRole('button', { name: '保存名称' }))
  expect(screen.getByRole('alert')).toHaveTextContent('1~64')
  expect(calls.filter((r) => r.method === 'PUT')).toHaveLength(0)
  await u.type(screen.getByLabelText('显示名称'), '  车间负责人  ')
  await u.click(screen.getByRole('button', { name: '保存名称' }))
  expect(screen.getByRole('alert')).toHaveTextContent('保存失败')
  expect(screen.getByLabelText('显示名称')).toHaveValue('  车间负责人  ')
  client.setQueryDefaults(['version-tools-draft'], { gcTime: Infinity })
  client.setQueryData(['version-tools-draft', 5], { dirty: true })
  await u.click(screen.getByRole('button', { name: '保存名称' }))
  expect(await screen.findByRole('status')).toHaveTextContent('名称已保存')
  expect(screen.getByText('车间负责人')).toBeInTheDocument()
  expect(screen.getByText('admin · 管理员')).toBeInTheDocument()
  expect(client.getQueryData(['version-tools-draft', 5])).toEqual({ dirty: true })
  expect(calls.filter((r) => r.method === 'PUT').at(-1)?.body).toEqual({ displayName: '车间负责人' })
  await u.click(screen.getByRole('link', { name: '返回高级管理' }))
  expect(await screen.findByRole('link', { name: '账号管理' })).toHaveTextContent('车间负责人')
  expect(screen.queryByRole('button', { name: '修改名称' })).not.toBeInTheDocument()
  expect(client.getQueryData(['version-tools-draft', 5])).toEqual({ dirty: true })
})

test('修改密码校验一致性，原密码错误保留表单，成功后清空会话并提示重新登录', async () => {
  const calls = server()
  const { client } = renderApp(<App />, '/me/account')
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '修改密码' }))
  await u.type(screen.getByLabelText('原密码'), 'wrong-password')
  await u.type(screen.getByLabelText('新密码', { exact: true }), 'new-password')
  await u.type(screen.getByLabelText('确认新密码'), 'different')
  await u.click(screen.getByRole('button', { name: '确认修改' }))
  expect(screen.getByRole('alert')).toHaveTextContent('两次输入的新密码不一致')
  expect(calls.filter((r) => r.path === '/auth/password')).toHaveLength(0)
  await u.clear(screen.getByLabelText('确认新密码'))
  await u.type(screen.getByLabelText('确认新密码'), 'new-password')
  await u.click(screen.getByRole('button', { name: '确认修改' }))
  expect(screen.getByRole('alert')).toHaveTextContent('原密码不正确')
  expect(screen.getByLabelText('新密码', { exact: true })).toHaveValue('new-password')
  expect(screen.getByRole('dialog', { name: '修改密码' })).toBeVisible()
  await u.clear(screen.getByLabelText('原密码'))
  await u.type(screen.getByLabelText('原密码'), 'old-password')
  client.setQueryData(['drawing', 1], { old: true })
  await u.click(screen.getByRole('button', { name: '确认修改' }))
  expect(await screen.findByRole('status')).toHaveTextContent('密码已修改，请使用新密码登录')
  expect(screen.getByLabelText('用户名')).toHaveValue('admin')
  expect(screen.getByLabelText('密码')).toHaveValue('')
  expect(client.getQueryData(['drawing', 1])).toBeUndefined()
  expect(calls.filter((r) => r.path === '/auth/password').at(-1)?.body).toEqual({ currentPassword: 'old-password', newPassword: 'new-password' })
})

test('修改密码前保护未保存刀具表，取消关闭不提交密码', async () => {
  const calls = server()
  const { client } = renderApp(<App />, '/me/account')
  const u = userEvent.setup()
  await screen.findByRole('button', { name: '修改密码' })
  client.setQueryDefaults(['version-tools-draft'], { gcTime: Infinity })
  client.setQueryData(['version-tools-draft', 5], { dirty: true })
  await u.click(screen.getByRole('button', { name: '修改密码' }))
  expect(screen.getByRole('button', { name: '确认修改' })).toBeDisabled()
  expect(screen.getByText('请先保存或撤销未保存的刀具表修改，再修改密码。')).toBeInTheDocument()
  await u.click(screen.getByRole('button', { name: '取消' }))
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  expect(calls.filter((r) => r.method === 'PUT')).toHaveLength(0)
})

test('请求中不能重复提交或关闭密码表单，失败后可重试', async () => {
  server()
  renderApp(<App />, '/me/account')
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '修改密码' }))
  await u.type(screen.getByLabelText('原密码'), 'old-password')
  await u.type(screen.getByLabelText('新密码', { exact: true }), 'new-password')
  await u.type(screen.getByLabelText('确认新密码'), 'new-password')
  let finish!: (r: Response) => void
  const original = fetch
  const pending = vi.fn(() => new Promise<Response>((resolve) => { finish = resolve }))
  vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => String(input).endsWith('/auth/password') ? pending() : original(input, init)))
  await u.click(screen.getByRole('button', { name: '确认修改' }))
  expect(screen.getByRole('button', { name: '确认修改' })).toBeDisabled()
  expect(screen.getByRole('button', { name: '取消' })).toBeDisabled()
  await u.click(screen.getByRole('button', { name: '关闭' }))
  await u.keyboard('{Escape}')
  expect(screen.getByRole('dialog')).toBeVisible()
  expect(pending).toHaveBeenCalledOnce()
  await act(async () => finish(new Response(JSON.stringify({ error: '暂时无法保存' }), { status: 500 })))
  await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('暂时无法保存'))
  expect(screen.getByLabelText('新密码', { exact: true })).toHaveValue('new-password')
  expect(screen.getByRole('button', { name: '确认修改' })).toBeEnabled()
})

test('登录页可展开忘记密码说明，初始化页面不提供找回入口', async () => {
  const u = userEvent.setup()
  const view = renderApp(<AuthPage setup={false} />)
  await u.click(screen.getByText('忘记密码？'))
  expect(screen.getByText(/请联系负责部署本系统的服务器管理员/)).toBeVisible()
  expect(screen.getByText(/暂不支持短信或邮箱找回/)).toBeVisible()
  view.unmount()
  renderApp(<AuthPage setup />)
  expect(screen.queryByText('忘记密码？')).not.toBeInTheDocument()
})
