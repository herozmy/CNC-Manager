import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Route, Routes } from 'react-router-dom'
import { afterEach, expect, test, vi } from 'vitest'
import { detail, mockApi, operation, program, renderApp } from '../test-utils'
import DrawingPage from './DrawingPage'

afterEach(() => vi.unstubAllGlobals())

const view = () => (
  <Routes>
    <Route path="/drawings/:id" element={<DrawingPage />} />
    <Route path="/drawings/:id/programs/:programId" element={<p>程序页</p>} />
  </Routes>
)

/** 有状态的假服务器：写操作后详情会变化，用来验证界面确实重新加载。 */
function server(initial: unknown[]) {
  let ops = initial
  const calls = mockApi((r) => {
    if (r.method === 'GET' && r.path === '/drawings/1/detail') return { body: detail(ops) }
    if (r.method === 'GET' && r.path.startsWith('/drawings?')) return { body: { items: [], total: 0, page: 1, size: 20 } }
    if (r.method === 'POST' && r.path === '/drawings/1/operations') {
      const b = r.body as { opNo: number; opName: string }
      ops = [...ops, operation({ id: 9, opNo: b.opNo, opName: b.opName, programs: [] })]
      return { status: 201, body: ops[ops.length - 1] }
    }
    if (r.method === 'PUT' && r.path.startsWith('/operations/')) return { body: {} }
    if (r.method === 'DELETE' && r.path.startsWith('/operations/')) {
      ops = []
      return { status: 204 }
    }
    if (r.method === 'POST' && r.path === '/operations/1/programs') return { status: 201, body: program({ id: 42, currentVersionNo: null }) }
  })
  return calls
}

test('空图纸：新增第一道工序，工序号默认 10，成功后列表刷新', async () => {
  const calls = server([])
  renderApp(view(), '/drawings/1')
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '新增第一道工序' }))
  const dialog = screen.getByRole('dialog', { name: '新增工序' })
  expect(within(dialog).getByLabelText('工序号')).toHaveValue('10')
  await u.type(within(dialog).getByLabelText('工序名称'), '粗车')
  await u.type(within(dialog).getByLabelText('Z 轴垫高（mm）'), '25.5')
  await u.click(within(dialog).getByRole('button', { name: '保存' }))

  expect(await screen.findByRole('heading', { name: '10# 粗车' })).toBeInTheDocument()
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  expect(calls.find((c) => c.method === 'POST')?.body).toEqual({ opNo: 10, opName: '粗车', machineId: null, fixture: '', zHeight: 25.5, remark: '' })
})

test('建议工序号 = 现有最大号 + 10', async () => {
  server([operation({ id: 1, opNo: 10 }), operation({ id: 2, opNo: 20, programs: [] })])
  renderApp(view(), '/drawings/1')
  await userEvent.setup().click(await screen.findByRole('button', { name: '新增工序' }))
  expect(screen.getByLabelText('工序号')).toHaveValue('30')
})

test('工序表单校验：工序号与垫高非法时不提交', async () => {
  const calls = server([])
  renderApp(view(), '/drawings/1')
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '新增第一道工序' }))
  await u.clear(screen.getByLabelText('工序号'))
  await u.type(screen.getByLabelText('工序号'), '0')
  await u.click(screen.getByRole('button', { name: '保存' }))
  expect(await screen.findByText('工序号应为 1~9999 的整数')).toBeInTheDocument()

  await u.clear(screen.getByLabelText('工序号'))
  await u.type(screen.getByLabelText('工序号'), '10')
  await u.type(screen.getByLabelText('Z 轴垫高（mm）'), '-3')
  await u.click(screen.getByRole('button', { name: '保存' }))
  expect(await screen.findByText('垫高应为不小于 0 的数字')).toBeInTheDocument()
  expect(calls.some((c) => c.method === 'POST')).toBe(false)
})

test('工序号重复时标在输入框上，弹层保持打开', async () => {
  mockApi((r) => {
    if (r.path === '/drawings/1/detail') return { body: detail([]) }
    if (r.method === 'POST') return { status: 409, body: { error: '记录已存在，或与现有数据冲突: 该图纸下已存在相同工序号' } }
  })
  renderApp(view(), '/drawings/1')
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '新增第一道工序' }))
  await u.click(screen.getByRole('button', { name: '保存' }))
  expect(await screen.findByText('该图纸下已存在相同工序号')).toBeInTheDocument()
  expect(screen.getByRole('dialog')).toBeInTheDocument()
})

test('编辑工序时原样带回机台，避免被后端清空', async () => {
  const calls = server([operation({ machineId: 5, machineName: 'CK6150', fixture: '三爪', zHeight: 25.5 })])
  renderApp(view(), '/drawings/1')
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '编辑工序 10' }))
  const dialog = screen.getByRole('dialog', { name: '编辑工序' })
  expect(within(dialog).getByLabelText('工序名称')).toHaveValue('粗车')
  expect(within(dialog).getByLabelText('Z 轴垫高（mm）')).toHaveValue('25.5')
  await u.clear(within(dialog).getByLabelText('工序名称'))
  await u.type(within(dialog).getByLabelText('工序名称'), '精车')
  await u.click(within(dialog).getByRole('button', { name: '保存' }))
  await waitFor(() => expect(calls.some((c) => c.method === 'PUT')).toBe(true))
  expect(calls.find((c) => c.method === 'PUT')).toMatchObject({ path: '/operations/1', body: { opNo: 10, opName: '精车', machineId: 5, zHeight: 25.5 } })
})

test('删除工序需二次确认，并说明会连带删除程序', async () => {
  const calls = server([operation()])
  renderApp(view(), '/drawings/1')
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '编辑工序 10' }))
  await u.click(screen.getByRole('button', { name: '删除此工序' }))
  expect(screen.getByText(/及其下的 1 个程序和全部版本/)).toBeInTheDocument()
  expect(calls.some((c) => c.method === 'DELETE')).toBe(false)

  await u.click(screen.getByRole('button', { name: '确认删除' }))
  expect(await screen.findByText('暂无工序')).toBeInTheDocument()
  expect(calls.find((c) => c.method === 'DELETE')?.path).toBe('/operations/1')
})

test('取消删除不会发请求', async () => {
  const calls = server([operation()])
  renderApp(view(), '/drawings/1')
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '编辑工序 10' }))
  await u.click(screen.getByRole('button', { name: '删除此工序' }))
  await u.click(screen.getByRole('button', { name: '取消' }))
  expect(calls.some((c) => c.method === 'DELETE')).toBe(false)
})

test('新增程序：校验程序号，成功后进入程序页', async () => {
  const calls = server([operation({ programs: [] })])
  renderApp(view(), '/drawings/1')
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '在 10# 下新增程序' }))
  await u.click(screen.getByRole('button', { name: '创建并上传文件' }))
  expect(await screen.findByText('请输入程序号')).toBeInTheDocument()
  expect(calls.some((c) => c.method === 'POST')).toBe(false)

  await u.type(screen.getByLabelText('程序号'), 'O1234')
  await u.type(screen.getByLabelText('数控系统'), 'FANUC')
  await u.click(screen.getByRole('button', { name: '创建并上传文件' }))
  expect(await screen.findByText('程序页')).toBeInTheDocument()
  expect(calls.find((c) => c.method === 'POST')?.body).toEqual({ programNo: 'O1234', programName: '', controller: 'FANUC', remark: '' })
})

test('Esc 关闭弹层', async () => {
  server([])
  renderApp(view(), '/drawings/1')
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '新增第一道工序' }))
  expect(screen.getByRole('dialog')).toBeInTheDocument()
  await u.keyboard('{Escape}')
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
})
