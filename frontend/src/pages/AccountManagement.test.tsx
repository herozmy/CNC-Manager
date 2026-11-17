import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Route, Routes } from 'react-router-dom'
import { afterEach, expect, test, vi } from 'vitest'
import { detail, mockApi, operation, program, renderApp, user, version } from '../test-utils'
import type { User } from '../api/types'
import App from '../App'
import AdminPage from './AdminPage'
import AuditPage from './AuditPage'
import DrawingPage from './DrawingPage'
import ProgramPage from './ProgramPage'
import ProgramSheet from '../components/ProgramSheet'
import type { Program } from '../api/types'

const member: User = { ...user, id: 2, username: 'worker', displayName: '车工', role: 'editor' }
afterEach(() => vi.unstubAllGlobals())

function navigationServer(currentUser: User = user, failUsers = false) {
  let fail = failUsers
  return mockApi(r => {
    if (r.path === '/auth/status') return { body: { setupRequired: false } }
    if (r.path === '/auth/me') return { body: currentUser }
    if (r.path === '/meta') return { body: { version: 'local-test' } }
    if (r.path === '/admin/users') {
      if (fail) { fail = false; return { status: 500, body: { error: '暂时不可用' } } }
      return { body: [currentUser] }
    }
    if (r.path.startsWith('/admin/audit-logs?')) return { body: { items: [], total: 0, page: 1, size: 20 } }
  })
}

test('头像下拉菜单进入统一账号页，当前账号只展示一次，日志可逐级返回', async () => {
  const calls = navigationServer()
  renderApp(<App />, '/me')
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '账号菜单' }))
  const entry = screen.getByRole('menuitem', { name: '账号管理' })
  expect(entry).toHaveAttribute('href', '/me/account')
  expect(calls.filter(r => r.path.startsWith('/admin'))).toHaveLength(0)
  await u.click(entry)
  expect(await screen.findByRole('heading', { name: '账号管理', level: 1 })).toBeVisible()
  expect(screen.queryByRole('menu')).not.toBeInTheDocument()
  expect(screen.getAllByRole('region', { name: '当前账号' })).toHaveLength(1)
  expect(await screen.findByText('暂无其他账号，可点击“新增账号”创建。')).toBeVisible()
  expect(within(screen.getByRole('list', { name: '账号列表' })).queryByText('admin')).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: '修改名称' })).toBeEnabled()
  expect(screen.getByRole('button', { name: '修改密码' })).toBeEnabled()
  expect(screen.getAllByRole('button', { name: '账号菜单' })).toHaveLength(1)
  expect(calls.some(r => r.path.startsWith('/admin/audit-logs'))).toBe(false)
  await u.click(screen.getByRole('link', { name: '查看操作日志 →' }))
  expect(await screen.findByText('没有匹配的记录')).toBeVisible()
  expect(screen.getAllByRole('link', { name: '我的' }).every(link => link.getAttribute('aria-current') === 'page')).toBe(true)
  await u.click(screen.getByRole('link', { name: '返回账号管理' }))
  await u.click(await screen.findByRole('link', { name: '返回我的' }))
  expect(await screen.findByRole('link', { name: '账号管理' })).toBeVisible()
  expect(screen.queryByRole('button', { name: '新增账号' })).not.toBeInTheDocument()
})

test.each(['/admin', '/admin/audit'])('%s直达链接仍有效，并归属我的导航', async route => {
  navigationServer()
  renderApp(<App />, route)
  const heading = route === '/admin' ? '账号管理' : '操作日志'
  expect(await screen.findByRole('heading', { name: heading, level: 1 })).toBeVisible()
  expect(screen.getAllByRole('link', { name: '我的' }).every(link => link.getAttribute('aria-current') === 'page')).toBe(true)
})

test('账号列表加载失败可重试，失败时仍能返回我的', async () => {
  const calls = navigationServer(user, true)
  renderApp(<App />, '/me')
  const u = userEvent.setup()
  await u.click(await screen.findByRole('link', { name: '账号管理' }))
  expect(await screen.findByText('管理数据加载失败')).toBeVisible()
  expect(screen.getByRole('link', { name: '返回我的' })).toHaveAttribute('href', '/me')
  await u.click(screen.getByRole('button', { name: '重试' }))
  expect(await screen.findByText('暂无其他账号，可点击“新增账号”创建。')).toBeVisible()
  expect(calls.filter(r => r.path === '/admin/users')).toHaveLength(2)
})

test.each([member, { ...user, enabled: false }])('$username启用状态$enabled进入统一账号页，只能操作自己且不请求管理接口', async currentUser => {
  const calls = navigationServer(currentUser)
  const view = renderApp(<App />, '/me')
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '账号菜单' }))
  await u.click(screen.getByRole('menuitem', { name: '账号管理' }))
  expect(await screen.findByRole('heading', { name: '账号管理', level: 1 })).toBeVisible()
  expect(screen.getByRole('button', { name: '修改名称' })).toBeEnabled()
  expect(screen.queryByRole('button', { name: '新增账号' })).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '账号身份' })).not.toBeInTheDocument()
  view.unmount()
  renderApp(<App />, '/admin')
  expect(await screen.findByRole('heading', { name: '账号管理', level: 1 })).toBeVisible()
  expect(calls.filter(r => r.path.startsWith('/admin'))).toHaveLength(0)
})

test('新增账号只选择管理员或普通账号，无自定义角色和权限', async () => {
  let users: User[] = [user]
  const calls = mockApi((r) => {
    if (r.path === '/admin/users' && r.method === 'POST') { users = [...users, { ...member, ...r.body as object }]; return { status: 201, body: users[1] } }
    if (r.path === '/admin/users/2' && r.method === 'PUT') { users[1] = { ...users[1], ...r.body as object }; return { body: users[1] } }
    if (r.path === '/admin/users') return { body: users }
  })
  renderApp(<AdminPage />)
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '新增账号' }))
  expect(screen.queryByText('业务角色')).not.toBeInTheDocument()
  expect(screen.queryByRole('checkbox')).not.toBeInTheDocument()
  expect(screen.getAllByRole('option').map((o) => o.textContent)).toEqual(['普通账号', '系统管理员'])
  await u.type(screen.getByLabelText('用户名'), 'worker')
  await u.type(screen.getByLabelText('显示名称'), '车工')
  await u.type(screen.getByLabelText('初始密码'), 'password123')
  await u.click(screen.getByRole('button', { name: '保存账号' }))
  expect(await screen.findByRole('button', { name: '编辑账号 worker' })).toBeInTheDocument()
  expect(calls.find((r) => r.method === 'POST')?.body).toEqual({ username:'worker',displayName:'车工',password:'password123',role:'editor',enabled:true })
  expect(calls.some((r) => r.path.includes('/roles'))).toBe(false)
  await u.click(screen.getByRole('button',{name:'编辑账号 worker'}))
  await u.selectOptions(screen.getByLabelText('账号身份'),'admin')
  await u.click(screen.getByLabelText('启用账号'))
  await u.click(screen.getByRole('button',{name:'保存账号'}))
  await waitFor(() => expect(calls.find((r) => r.method === 'PUT')?.body).toEqual({displayName:'车工',role:'admin',enabled:false}))
})

test('普通账号不请求管理员接口', () => {
  const calls = mockApi(() => undefined)
  renderApp(<AdminPage />, '/admin', member)
  expect(screen.getByText('仅系统管理员可访问账号管理。')).toBeInTheDocument()
  expect(calls).toHaveLength(0)
})

test('普通账号拥有自己数据的编辑和下载入口', async () => {
  mockApi((r) => {
    if (r.path === '/drawings/1/detail') return { body: detail([operation()]) }
    if (r.path === '/programs/1/versions') return { body: [version()] }
    if (r.path === '/versions/5/tools') return { body: { items: [], revision: 'empty' } }
  })
  renderApp(<Routes><Route path="/drawings/:id/programs/:programId" element={<ProgramPage />} /></Routes>, '/drawings/1/programs/1', member)
  await screen.findByRole('heading', { name: 'O1234' })
  for (const name of ['编辑程序信息', '删除程序', '上传新版本', '保存刀具表', '手动添加']) expect(await screen.findByRole('button', { name })).toBeInTheDocument()
  expect(screen.getByRole('link', { name: '下载当前版本' })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '查看 / 编辑当前版本' })).toBeInTheDocument()
})

test('待分配图纸通过管理员确认归属', async () => {
  let assigned = false
  const calls = mockApi((r) => {
    if (r.path === '/drawings/1/detail') return { body: detail([], { ownerUserId: assigned ? 2 : 0, ownerName: assigned ? member.displayName : '待分配' }) }
    if (r.path === '/admin/users') return { body: [user,member] }
    if (r.path === '/admin/drawings/1/owner') { assigned = true; return { body: { ownerUserId: 2 } } }
  })
  const { client } = renderApp(<Routes><Route path="/drawings/:id" element={<DrawingPage />} /></Routes>, '/drawings/1')
  act(() => client.setQueryData(['admin-users'], [user, member]))
  const u = userEvent.setup()
  await screen.findByRole('option', { name: '车工（worker）' })
  await u.selectOptions(screen.getByLabelText('所属账号'), '2')
  await u.click(screen.getByRole('button', { name: '确认分配图纸' }))
  expect(await screen.findByText('所属账号：车工')).toBeInTheDocument()
  expect(calls.find((r) => r.method === 'PUT')?.body).toEqual({ ownerUserId: 2 })
  expect(screen.queryByRole('button', { name: '确认分配图纸' })).not.toBeInTheDocument()
})

test('日志按账号、结果过滤，分页保持筛选条件', async () => {
  const calls = mockApi((r) => {
    if (r.path === '/admin/users') return { body: [user, member] }
    if (r.path.startsWith('/admin/audit-logs?')) return { body: { items: [], total: 25, page: 1, size: 20 } }
  })
  const { client } = renderApp(<AuditPage />)
  act(() => client.setQueryData(['admin-users'], [user, member]))
  const u = userEvent.setup()
  await screen.findByRole('option', { name: '车工 (worker)' })
  await u.selectOptions(screen.getByLabelText('操作者'), '2')
  await u.selectOptions(screen.getByLabelText('结果'), 'failed')
  await u.click(screen.getByRole('button', { name: '筛选日志' }))
  await waitFor(() => expect(calls.some((r) => r.path.includes('userId=2') && r.path.includes('result=failed'))).toBe(true))
  await u.click(await screen.findByRole('button', { name: '下一页' }))
  await waitFor(() => expect(calls.some((r) => r.path.includes('page=2') && r.path.includes('userId=2') && r.path.includes('result=failed'))).toBe(true))
})

test('程序冲突保留草稿，确认载入后使用最新修订号', async () => {
  const calls = mockApi((r) => {
    if (r.method === 'GET' && r.path === '/programs/1') return { body: program({ programName: '另一用户的新名称', revision: 2 }) }
    if (r.method === 'PUT') {
      const body = r.body as { expectedRevision: number }
      return body.expectedRevision === 1 ? { status: 409, body: { error: '程序信息已被修改' } } : { body: program({ ...r.body as object, revision: 3 }) }
    }
  })
  const saved = vi.fn()
  renderApp(<ProgramSheet drawingId={1} operationId={1} program={program() as Program} onClose={saved} />)
  const u = userEvent.setup()
  await u.clear(screen.getByLabelText('程序名称')); await u.type(screen.getByLabelText('程序名称'), '本地草稿')
  await u.click(screen.getByRole('button', { name: '保存修改' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('程序信息已被修改')
  expect(screen.getByLabelText('程序名称')).toHaveValue('本地草稿')
  expect(screen.getByRole('button', { name: '保存修改' })).toBeDisabled()
  await u.click(screen.getByRole('button', { name: '重新载入程序信息' }))
  expect(calls.filter((r) => r.method === 'GET' && r.path === '/programs/1')).toHaveLength(0)
  await u.click(screen.getByRole('button', { name: '放弃并载入' }))
  await waitFor(() => expect(screen.getByLabelText('程序名称')).toHaveValue('另一用户的新名称'))
  await u.click(screen.getByRole('button', { name: '保存修改' }))
  await waitFor(() => expect(saved).toHaveBeenCalledOnce())
  expect(calls.filter((r) => r.method === 'PUT').at(-1)?.body).toMatchObject({ expectedRevision: 2 })
})

test('自己的身份设置不重复提供名称编辑，最后管理员保护失败后保留设置', async () => {
  const calls = mockApi(r => {
    if (r.path === '/admin/users' && r.method === 'GET') return { body: [user, member] }
    if (r.path === '/admin/users/1') return { status: 400, body: { error: '系统至少保留一个启用的管理员' } }
  })
  renderApp(<AdminPage />)
  const u = userEvent.setup()
  await u.click(screen.getByRole('button', { name: '账号身份' }))
  expect(screen.queryByLabelText('显示名称')).not.toBeInTheDocument()
  await u.selectOptions(screen.getByLabelText('账号身份'), 'editor')
  await u.click(screen.getByRole('button', { name: '保存账号' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('至少保留一个启用的管理员')
  expect(screen.getByLabelText('账号身份')).toHaveValue('editor')
  expect(calls.find(r => r.method === 'PUT')?.body).toEqual({ displayName: '管理员', role: 'editor', enabled: true })
  await u.click(screen.getByRole('button', { name: '关闭' }))
  expect(screen.getByRole('button', { name: '修改名称' })).toBeEnabled()
})

test('其他账号密码重置失败保留输入可重试，并不影响当前登录', async () => {
  let fail = true
  const calls = mockApi(r => {
    if (r.path === '/admin/users') return { body: [user, member] }
    if (r.path === '/admin/users/2/reset-password') {
      if (fail) { fail = false; return { status: 500, body: { error: '重置失败' } } }
      return { status: 204 }
    }
  })
  renderApp(<AdminPage />)
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '重置密码 worker' }))
  await u.type(screen.getByLabelText('新密码'), 'new-password')
  await u.click(screen.getByRole('button', { name: '确认重置密码' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('重置失败')
  expect(screen.getByLabelText('新密码')).toHaveValue('new-password')
  await u.click(screen.getByRole('button', { name: '确认重置密码' }))
  expect(await screen.findByRole('status')).toHaveTextContent('保存成功')
  expect(screen.getByRole('button', { name: '修改密码' })).toBeEnabled()
  expect(calls.filter(r => r.path.endsWith('/reset-password'))).toHaveLength(2)
})
