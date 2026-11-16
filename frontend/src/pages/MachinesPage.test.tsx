import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Route, Routes } from 'react-router-dom'
import { afterEach, expect, test, vi } from 'vitest'
import type { Machine, MachineInput } from '../api/types'
import { detail, mockApi, operation, renderApp, user } from '../test-utils'
import MachineSheet from '../components/MachineSheet'
import MachinePage from './MachinePage'
import MachinesPage from './MachinesPage'

const member = { ...user, id: 2, role: 'editor' as const }
const machine = (over: Partial<Machine> = {}): Machine => ({ id: 1, code: 'CNC-01', name: '立式加工中心', enabled: true, controller: 'FANUC', remark: '', ownerUserId: 2, ownerName: '加工员', drawingCount: 1, operationCount: 1, programCount: 1, createdAt: '', updatedAt: '', ...over })
const page = (items: unknown[], total = items.length, current = 1, size = 20) => ({ items, total, page: current, size })
afterEach(() => vi.unstubAllGlobals())

test('机台筛选保留查询条件，管理员可筛选所属账号', async () => {
  const calls = mockApi((r) => {
    if (r.path === '/admin/users') return { body: [user, member] }
    if (r.path.startsWith('/machines?')) return { body: page([machine({ enabled: false })]) }
  })
  renderApp(<MachinesPage />, '/machines?q=CNC&status=disabled&owner=2')
  await screen.findByText('CNC-01')
  expect(screen.getByLabelText('搜索机台')).toHaveValue('CNC')
  expect(screen.getByLabelText('机台状态')).toHaveValue('disabled')
  expect(screen.getByLabelText('筛选所属账号')).toHaveValue('2')
  expect(calls.some((r) => r.path.includes('q=CNC') && r.path.includes('enabled=false') && r.path.includes('ownerUserId=2'))).toBe(true)
  expect(screen.getByRole('link', { name: /CNC-01/ })).toHaveAttribute('href', '/machines/1')
  await userEvent.setup().selectOptions(screen.getByLabelText('机台状态'), 'enabled')
  await waitFor(() => expect(calls.some((r) => r.path.includes('enabled=true'))).toBe(true))
})

test('普通用户不发送外来归属筛选，翻页后改变搜索从第一页开始', async () => {
  const calls = mockApi((r) => {
    if (r.path.startsWith('/machines?')) {
      const params = new URL(r.path, 'http://x').searchParams
      const n = Number(params.get('page'))
      return { body: page([machine({ id: n, code: `CNC-${n}` })], 2, n, 1) }
    }
  })
  renderApp(<MachinesPage />, '/machines?owner=99', member)
  await screen.findByText('CNC-1')
  expect(screen.queryByLabelText('筛选所属账号')).not.toBeInTheDocument()
  const u = userEvent.setup()
  await u.click(screen.getByRole('button', { name: '加载更多机台' }))
  await screen.findByText('CNC-2')
  await u.type(screen.getByLabelText('搜索机台'), 'FANUC')
  await waitFor(() => expect(calls.some((r) => r.path.includes('q=FANUC') && r.path.includes('page=1'))).toBe(true))
  expect(calls.every((r) => !r.path.includes('ownerUserId'))).toBe(true)
})

test('新增机台重名失败保留草稿，取消可继续编辑，成功只提交一次', async () => {
  let fail = true
  const calls = mockApi((r) => {
    if (r.path === '/machines') return fail ? { status: 409, body: { error: '机台编号已存在' } } : { status: 201, body: machine(r.body as MachineInput) }
  })
  const saved = vi.fn(), closed = vi.fn()
  renderApp(<MachineSheet onClose={closed} onSaved={saved} />, '/', member)
  const u = userEvent.setup()
  await u.type(screen.getByLabelText('机台编号'), ' CNC-01 ')
  await u.type(screen.getByLabelText('控制系统（可选）'), 'FANUC')
  await u.click(screen.getByRole('button', { name: '创建机台' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('机台编号已存在')
  expect(screen.getByLabelText('机台编号')).toHaveValue(' CNC-01 ')
  await u.click(screen.getByRole('button', { name: '取消' }))
  expect(closed).not.toHaveBeenCalled()
  await u.click(screen.getByRole('button', { name: '继续编辑' }))
  fail = false
  await u.click(screen.getByRole('button', { name: '创建机台' }))
  await waitFor(() => expect(saved).toHaveBeenCalledOnce())
  expect(calls).toHaveLength(2)
  expect(calls.at(-1)?.body).toMatchObject({ code: 'CNC-01', controller: 'FANUC', ownerUserId: 2 })
})

test('保存过程中禁止重复提交及关闭，网络失败可重试', async () => {
  let reject!: (reason: unknown) => void
  const fetcher = vi.fn(() => new Promise<Response>((_, r) => { reject = r }))
  vi.stubGlobal('fetch', fetcher)
  const closed = vi.fn()
  renderApp(<MachineSheet machine={machine()} onClose={closed} onSaved={vi.fn()} />, '/', member)
  const u = userEvent.setup()
  await u.type(screen.getByLabelText('备注'), '草稿')
  await u.dblClick(screen.getByRole('button', { name: '保存修改' }))
  expect(fetcher).toHaveBeenCalledOnce()
  await u.click(screen.getByRole('button', { name: '关闭' }))
  await u.keyboard('{Escape}')
  expect(closed).not.toHaveBeenCalled()
  reject(new TypeError('offline'))
  expect(await screen.findByRole('alert')).toHaveTextContent('保存失败')
  expect(screen.getByLabelText('备注')).toHaveValue('草稿')
})

function renderMachine() {
  return renderApp(<Routes><Route path="/machines/:id" element={<MachinePage />} /></Routes>, '/machines/1', member)
}

test('关联清单展示工序、程序和当前版本，搜索失败后可重试', async () => {
  let fail = false
  const calls = mockApi((r) => {
    if (r.path === '/machines/1') return { body: machine() }
    if (r.path.startsWith('/machines/1/drawings')) return fail ? { status: 503 } : { body: page([detail([operation()])]) }
  })
  renderMachine()
  expect(await screen.findByRole('link', { name: /O1234/ })).toHaveAttribute('href', '/drawings/1/programs/1')
  expect(screen.getByText('10# 粗车')).toBeInTheDocument()
  expect(screen.getByText('V3')).toBeInTheDocument()
  fail = true
  await userEvent.setup().type(screen.getByLabelText('搜索关联记录'), 'O1234')
  await screen.findByText('关联记录加载失败')
  expect(calls.some((r) => r.path.includes('q=O1234'))).toBe(true)
  fail = false
  await userEvent.setup().click(screen.getByRole('button', { name: '重试' }))
  expect(await screen.findByRole('link', { name: /O1234/ })).toBeInTheDocument()
})

test('停用失败保留确认，成功保留关联清单且可以重新启用', async () => {
  let m = machine(), fail = true
  const calls = mockApi((r) => {
    if (r.path === '/machines/1') return { body: m }
    if (r.path.startsWith('/machines/1/drawings')) return { body: page([detail([operation()])]) }
    if (r.path === '/machines/1/enabled') {
      if (fail) return { status: 503 }
      m = { ...m, ...(r.body as { enabled: boolean }) }; return { body: m }
    }
  })
  renderMachine()
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '停用机台' }))
  await u.click(screen.getByRole('button', { name: '确认停用' }))
  expect(await screen.findByRole('alert')).toBeInTheDocument()
  fail = false
  await u.click(screen.getByRole('button', { name: '确认停用' }))
  await screen.findByRole('button', { name: '启用机台' })
  expect(screen.getByRole('link', { name: /O1234/ })).toBeInTheDocument()
  await u.click(screen.getByRole('button', { name: '启用机台' }))
  await u.click(within(screen.getByRole('dialog')).getByRole('button', { name: '确认启用' }))
  await screen.findByRole('button', { name: '停用机台' })
  expect(calls.filter((r) => r.method === 'PUT').map((r) => r.body)).toEqual([{ enabled: false }, { enabled: false }, { enabled: true }])
})

test('不存在或无权访问的机台不请求关联记录', async () => {
  const calls = mockApi(() => ({ status: 404 }))
  renderMachine()
  expect(await screen.findByText('机台不存在或无权访问')).toBeInTheDocument()
  expect(calls).toHaveLength(1)
})

test('删除机台需要确认，取消无请求，失败可重试，成功返回列表并清理关联缓存', async () => {
  let fail = true
  const calls = mockApi((r) => {
    if (r.path === '/machines/1' && r.method === 'GET') return { body: machine() }
    if (r.path.startsWith('/machines/1/drawings')) return { body: page([]) }
    if (r.path.startsWith('/machines?')) return { body: page([]) }
    if (r.method === 'DELETE') return fail ? { status: 503, body: { error: '暂时不可用' } } : { status: 204 }
  })
  const { client } = renderApp(<Routes><Route path="/machines/:id" element={<MachinePage />} /><Route path="/machines" element={<MachinesPage />} /></Routes>, '/machines/1', member)
  client.setQueryDefaults(['drawing-machines'], { gcTime: Infinity })
  client.setQueryData(['drawing-machines', 1], [machine()])
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '删除机台' }))
  expect(screen.getByRole('dialog')).toHaveTextContent('NC 文件、全部版本及版本刀具表保留')
  await u.click(screen.getByRole('button', { name: '取消' }))
  expect(calls.some((r) => r.method === 'DELETE')).toBe(false)
  await u.click(screen.getByRole('button', { name: '删除机台' }))
  await u.click(screen.getByRole('button', { name: '确认删除' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('删除失败')
  fail = false
  await u.click(screen.getByRole('button', { name: '确认删除' }))
  await screen.findByRole('heading', { name: '机台管理' })
  expect(calls.filter((r) => r.method === 'DELETE')).toHaveLength(2)
  expect(client.getQueryState(['drawing-machines', 1])?.isInvalidated).toBe(true)
})
