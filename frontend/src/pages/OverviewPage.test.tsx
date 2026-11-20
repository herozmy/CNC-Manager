import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, test, vi } from 'vitest'
import { drawing, mockApi, renderApp, user } from '../test-utils'
import type { DrawingInput } from '../api/types'
import OverviewPage from './OverviewPage'

afterEach(() => vi.unstubAllGlobals())

test('显示图纸卡片、数量统计与相对时间', async () => {
  mockApi(() => ({ body: { items: [drawing({ drawingNo: 'D-1', name: '壳体' }), drawing({ id: 2, drawingNo: 'D-2', name: '', customer: '' })], total: 2, page: 1, size: 20 } }))
  renderApp(<OverviewPage />, '/', { ...user, role: 'editor' })
  expect(await screen.findByText('D-1')).toBeInTheDocument()
  expect(screen.getByText('共 2 张图纸')).toBeInTheDocument()
  expect(screen.getAllByText('2 道工序 · 3 个程序')).toHaveLength(2)
  expect(screen.getAllByText('刚刚').length).toBeGreaterThan(0)
  expect(screen.getByRole('link', { name: /D-1/ })).toHaveAttribute('href', '/drawings/1')
  expect(screen.getByRole('button', { name: '更多操作 D-1' })).toBeInTheDocument()
  expect(screen.getByText('类型：壳体')).toBeInTheDocument()
  expect(screen.getByText('类型：未分类')).toBeInTheDocument()
  expect(screen.queryByText(/客户A|版本 B/)).not.toBeInTheDocument()
})

test('总览直接编辑选中的图纸，回填信息并刷新卡片，不进入详情页', async () => {
  let second = drawing({ id: 2, drawingNo: 'D-2', price: '12.30', remark: '原备注' })
  const calls = mockApi((r) => {
    if (r.method === 'PUT' && r.path === '/drawings/2') {
      second = { ...second, ...(r.body as DrawingInput) }
      return { body: second }
    }
    if (r.method === 'GET' && r.path.startsWith('/drawings?')) return { body: { items: [drawing({ drawingNo: 'D-1' }), second], total: 2, page: 1, size: 20 } }
  })
  renderApp(<OverviewPage />, '/', { ...user, role: 'editor' })
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '更多操作 D-2' }))
  await u.click(await screen.findByRole('button', { name: '编辑图纸 D-2' }))
  const dialog = within(screen.getByRole('dialog', { name: '编辑图纸' }))
  expect(dialog.getByLabelText('图纸号')).toHaveValue('D-2')
  expect(dialog.getByLabelText('价格（元）')).toHaveValue('12.30')
  await u.clear(dialog.getByLabelText('图纸号'))
  await u.type(dialog.getByLabelText('图纸号'), 'D-2-新版')
  await u.clear(dialog.getByLabelText('材料'))
  await u.type(dialog.getByLabelText('材料'), '304')
  await u.click(dialog.getByRole('button', { name: '保存修改' }))
  expect(await screen.findByText('图纸已保存')).toBeInTheDocument()
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  expect(screen.getByRole('link', { name: /D-2-新版.*304/ })).toHaveAttribute('href', '/drawings/2')
  expect(screen.getByRole('button', { name: '更多操作 D-1' })).toBeInTheDocument()
  expect(calls.filter(r => r.method === 'PUT')).toEqual([expect.objectContaining({ path: '/drawings/2', body: expect.objectContaining({ drawingNo: 'D-2-新版', material: '304', price: '12.30', remark: '原备注', customer: '客户A', drawingVersion: 'B' }) })])
  expect(calls.some(r => r.path.endsWith('/detail'))).toBe(false)
  await u.click(await screen.findByRole('button', { name: '更多操作 D-2-新版' }))
  await u.click(screen.getByRole('button', { name: '编辑图纸 D-2-新版' }))
  expect(screen.getByLabelText('图纸号')).toHaveValue('D-2-新版')
  expect(screen.getByLabelText('材料')).toHaveValue('304')
})

test('筛选结果直接编辑后，重新匹配当前搜索条件', async () => {
  let current = drawing({ name: '壳体' })
  mockApi(r => {
    if (r.method === 'PUT') {
      current = { ...current, ...(r.body as DrawingInput) }
      return { body: current }
    }
    const q = new URL(r.path, 'http://x').searchParams.get('q')
    const items = !q || current.name.includes(q) ? [current] : []
    return { body: { items, total: items.length, page: 1, size: 20 } }
  })
  renderApp(<OverviewPage />, '/', { ...user, role: 'editor' })
  const u = userEvent.setup()
  await screen.findByText('D-1001')
  await u.type(screen.getByRole('searchbox'), '壳体')
  await screen.findByText('找到 1 张图纸')
  await u.click(await screen.findByRole('button', { name: '更多操作 D-1001' }))
  await u.click(screen.getByRole('button', { name: '编辑图纸 D-1001' }))
  await u.clear(screen.getByLabelText('类型'))
  await u.type(screen.getByLabelText('类型'), '轴类')
  await u.click(screen.getByRole('button', { name: '保存修改' }))
  expect(await screen.findByText('没有找到匹配的图纸')).toBeInTheDocument()
  expect(screen.getByRole('searchbox')).toHaveValue('壳体')
  await u.clear(screen.getByRole('searchbox'))
  expect(await screen.findByText('类型：轴类')).toBeInTheDocument()
})

test.each([409, 500, 'offline'] as const)('总览编辑遇到 %s 时保留草稿，重试成功后刷新列表', async failure => {
  let current = drawing()
  let fail = true
  const calls = mockApi(r => {
    if (r.method === 'PUT') {
      if (fail) {
        if (failure === 'offline') throw new TypeError('Failed to fetch')
        return { status: failure, body: { error: failure === 409 ? '记录已存在: 该图纸号已存在' : '保存失败，请重试' } }
      }
      current = { ...current, ...(r.body as DrawingInput) }
      return { body: current }
    }
    return { body: { items: [current], total: 1, page: 1, size: 20 } }
  })
  renderApp(<OverviewPage />, '/', { ...user, role: 'editor' })
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '更多操作 D-1001' }))
  await u.click(await screen.findByRole('button', { name: '编辑图纸 D-1001' }))
  await u.clear(screen.getByLabelText('图纸号'))
  await u.type(screen.getByLabelText('图纸号'), 'D-草稿')
  await u.click(screen.getByRole('button', { name: '保存修改' }))
  expect(await screen.findByText(failure === 409 ? '该图纸号已存在' : failure === 500 ? '保存失败，请重试' : '无法连接服务器，请重试')).toBeInTheDocument()
  expect(screen.getByLabelText('图纸号')).toHaveValue('D-草稿')
  expect(screen.getByRole('link', { name: /D-1001/ })).toBeInTheDocument()
  fail = false
  await u.click(screen.getByRole('button', { name: '保存修改' }))
  expect(await screen.findByText('图纸已保存')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '更多操作 D-草稿' })).toBeInTheDocument()
  expect(calls.filter(r => r.method === 'PUT')).toHaveLength(2)
})

test('总览编辑取消时确认放弃，再次打开回填已保存的信息', async () => {
  const calls = mockApi(() => ({ body: { items: [drawing()], total: 1, page: 1, size: 20 } }))
  renderApp(<OverviewPage />, '/', { ...user, role: 'editor' })
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '更多操作 D-1001' }))
  await u.click(await screen.findByRole('button', { name: '编辑图纸 D-1001' }))
  await u.type(screen.getByLabelText('图纸号'), '-草稿')
  await u.click(screen.getByRole('button', { name: '取消' }))
  expect(screen.getByText('有未保存的修改，确定放弃吗？')).toBeInTheDocument()
  await u.click(screen.getByRole('button', { name: '继续编辑' }))
  expect(screen.getByLabelText('图纸号')).toHaveValue('D-1001-草稿')
  await u.click(screen.getByRole('button', { name: '取消' }))
  await u.click(screen.getByRole('button', { name: '放弃修改' }))
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  await u.click(await screen.findByRole('button', { name: '更多操作 D-1001' }))
  await u.click(screen.getByRole('button', { name: '编辑图纸 D-1001' }))
  expect(screen.getByLabelText('图纸号')).toHaveValue('D-1001')
  expect(calls.some(r => r.method === 'PUT')).toBe(false)
})

test('停用账号不能从总览获取或编辑图纸', () => {
  const calls = mockApi(() => ({ body: {} }))
  renderApp(<OverviewPage />, '/', { ...user, enabled: false })
  expect(screen.getByText('账号不可用，请联系管理员。')).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: /编辑图纸/ })).not.toBeInTheDocument()
  expect(calls).toHaveLength(0)
})

test('搜索类型后，结果显示产品类型并排除其他类型的图纸', async () => {
  const all = [drawing({ drawingNo: 'D-1', name: '壳体类' }), drawing({ id: 2, drawingNo: 'D-2', name: '轴类' })]
  const calls = mockApi((r) => {
    const keyword = new URL(r.path, 'http://x').searchParams.get('q')
    const items = keyword === '壳体' ? [all[0]] : all
    return { body: { items, total: items.length, page: 1, size: 20 } }
  })
  renderApp(<OverviewPage />, '/', { ...user, role: 'editor' })
  await screen.findByText('D-2')
  await userEvent.setup().type(screen.getByPlaceholderText('图纸号、类型、材料、程序号'), '壳体')
  expect(await screen.findByText('找到 1 张图纸')).toBeInTheDocument()
  expect(screen.getByRole('link', { name: /D-1.*类型：壳体类/ })).toBeInTheDocument()
  expect(screen.queryByText('D-2')).not.toBeInTheDocument()
  expect(calls.some((r) => new URL(r.path, 'http://x').searchParams.get('q') === '壳体')).toBe(true)
})

test('从总览删除图纸：显示级联范围，取消不提交，确认后从列表移除', async () => {
  let items = [drawing({ drawingNo: 'D-1', operationCount: 2, programCount: 3 })]
  const calls = mockApi((r) => {
    if (r.method === 'DELETE' && r.path === '/drawings/1') {
      items = []
      return { status: 204 }
    }
    return { body: { items, total: items.length, page: 1, size: 20 } }
  })
  renderApp(<OverviewPage />, '/', { ...user, role: 'editor' })
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '更多操作 D-1' }))
  await u.click(await screen.findByRole('button', { name: '删除图纸 D-1' }))
  expect(screen.getByRole('dialog', { name: '删除图纸' })).toHaveTextContent('2 道工序、3 个程序和全部版本')
  await u.click(screen.getByRole('button', { name: '取消' }))
  expect(calls.some((r) => r.method === 'DELETE')).toBe(false)
  await u.click(await screen.findByRole('button', { name: '更多操作 D-1' }))
  await u.click(screen.getByRole('button', { name: '删除图纸 D-1' }))
  await u.click(screen.getByRole('button', { name: '确认删除' }))
  expect(await screen.findByText('还没有图纸')).toBeInTheDocument()
  expect(calls.filter((r) => r.method === 'DELETE')).toHaveLength(1)
})

test('搜索会防抖并带上转义后的关键字', async () => {
  const calls = mockApi(() => ({ body: { items: [], total: 0, page: 1, size: 20 } }))
  renderApp(<OverviewPage />, '/', { ...user, role: 'editor' })
  await screen.findByText('还没有图纸')
  const u = userEvent.setup()
  await u.type(screen.getByRole('searchbox'), 'O12 34&')
  await waitFor(() => expect(calls.some((c) => c.path.includes('q=O12%2034%26'))).toBe(true))
  // 防抖：中途的半截关键字不应各发一次请求
  expect(calls.filter((c) => c.path.includes('q=O')).length).toBeLessThanOrEqual(2)
  expect(await screen.findByText('没有找到匹配的图纸')).toBeInTheDocument()
})

test('分页：点击加载更多追加下一页', async () => {
  mockApi((r) => {
    const page = Number(new URL(r.path, 'http://x').searchParams.get('page'))
    return { body: { items: [drawing({ id: page, drawingNo: `D-P${page}` })], total: 2, page, size: 1 } }
  })
  renderApp(<OverviewPage />, '/', { ...user, role: 'editor' })
  expect(await screen.findByText('D-P1')).toBeInTheDocument()
  expect(screen.queryByText('D-P2')).not.toBeInTheDocument()
  await userEvent.setup().click(screen.getByRole('button', { name: '加载更多' }))
  expect(await screen.findByText('D-P2')).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '加载更多' })).not.toBeInTheDocument()
})

test('加载失败可重试', async () => {
  let fail = true
  mockApi(() => (fail ? { status: 500, body: { error: 'x' } } : { body: { items: [drawing()], total: 1, page: 1, size: 20 } }))
  renderApp(<OverviewPage />, '/', { ...user, role: 'editor' })
  expect(await screen.findByText('图纸加载失败')).toBeInTheDocument()
  fail = false
  await userEvent.setup().click(screen.getByRole('button', { name: '重试' }))
  expect(await screen.findByText('D-1001')).toBeInTheDocument()
})

test('卡片与台账表共享筛选结果、编辑和删除流程，切换视图不重复加载', async () => {
  let current = drawing({ drawingNo: 'D-TABLE', price: '12.30' })
  const calls = mockApi(r => {
    if (r.method === 'PUT' && r.path === '/drawings/1') { current = { ...current, ...(r.body as DrawingInput) }; return { body: current } }
    return { body: { items: [current], total: 1, page: 1, size: 20 } }
  })
  renderApp(<OverviewPage />, '/', { ...user, role: 'editor' })
  const u = userEvent.setup()
  await screen.findByText('D-TABLE')
  const reads = calls.length
  await u.click(screen.getByRole('button', { name: '台账表' }))
  expect(screen.getByRole('table', { name: '图纸台账表' })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '台账表' })).toHaveAttribute('aria-pressed', 'true')
  expect(screen.getByRole('link', { name: 'D-TABLE' })).toHaveAttribute('href', '/drawings/1')
  expect(calls).toHaveLength(reads)
  await u.click(await screen.findByRole('button', { name: '更多操作 D-TABLE' }))
  await u.click(screen.getByRole('button', { name: '编辑图纸 D-TABLE' }))
  expect(screen.getByLabelText('价格（元）')).toHaveValue('12.30')
  await u.clear(screen.getByLabelText('材料'))
  await u.type(screen.getByLabelText('材料'), '304')
  await u.click(screen.getByRole('button', { name: '保存修改' }))
  await screen.findByText('图纸已保存')
  expect(screen.getByRole('table')).toHaveTextContent('304')
  await u.click(await screen.findByRole('button', { name: '更多操作 D-TABLE' }))
  await u.click(screen.getByRole('button', { name: '删除图纸 D-TABLE' }))
  expect(screen.getByRole('dialog', { name: '删除图纸' })).toHaveTextContent('全部版本')
  await u.click(screen.getByRole('button', { name: '取消' }))
  expect(calls.some(r => r.method === 'DELETE')).toBe(false)
  await u.click(screen.getByRole('button', { name: '卡片' }))
  expect(screen.queryByRole('table')).not.toBeInTheDocument()
  expect(screen.getByRole('link', { name: /D-TABLE.*304/ })).toBeInTheDocument()
})
