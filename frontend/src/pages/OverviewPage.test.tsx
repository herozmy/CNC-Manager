import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, test, vi } from 'vitest'
import { drawing, mockApi, renderApp } from '../test-utils'
import OverviewPage from './OverviewPage'

afterEach(() => vi.unstubAllGlobals())

test('显示图纸卡片、数量统计与相对时间', async () => {
  mockApi(() => ({ body: { items: [drawing({ drawingNo: 'D-1', name: '壳体' }), drawing({ id: 2, drawingNo: 'D-2', name: '', customer: '' })], total: 2, page: 1, size: 20 } }))
  renderApp(<OverviewPage />)
  expect(await screen.findByText('D-1')).toBeInTheDocument()
  expect(screen.getByText('共 2 张图纸')).toBeInTheDocument()
  expect(screen.getAllByText('2 道工序 · 3 个程序')).toHaveLength(2)
  expect(screen.getAllByText('刚刚').length).toBeGreaterThan(0)
  expect(screen.getByRole('link', { name: /D-1/ })).toHaveAttribute('href', '/drawings/1')
  expect(screen.getByRole('button', { name: '删除图纸 D-1' })).toBeInTheDocument()
  expect(screen.getByText('类型：壳体')).toBeInTheDocument()
  expect(screen.getByText('类型：未分类')).toBeInTheDocument()
  expect(screen.queryByText(/客户A|版本 B/)).not.toBeInTheDocument()
})

test('搜索类型后，结果显示产品类型并排除其他类型的图纸', async () => {
  const all = [drawing({ drawingNo: 'D-1', name: '壳体类' }), drawing({ id: 2, drawingNo: 'D-2', name: '轴类' })]
  const calls = mockApi((r) => {
    const keyword = new URL(r.path, 'http://x').searchParams.get('q')
    const items = keyword === '壳体' ? [all[0]] : all
    return { body: { items, total: items.length, page: 1, size: 20 } }
  })
  renderApp(<OverviewPage />)
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
  renderApp(<OverviewPage />)
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '删除图纸 D-1' }))
  expect(screen.getByRole('dialog', { name: '删除图纸' })).toHaveTextContent('2 道工序、3 个程序和全部版本')
  await u.click(screen.getByRole('button', { name: '取消' }))
  expect(calls.some((r) => r.method === 'DELETE')).toBe(false)
  await u.click(screen.getByRole('button', { name: '删除图纸 D-1' }))
  await u.click(screen.getByRole('button', { name: '确认删除' }))
  expect(await screen.findByText('还没有图纸')).toBeInTheDocument()
  expect(calls.filter((r) => r.method === 'DELETE')).toHaveLength(1)
})

test('搜索会防抖并带上转义后的关键字', async () => {
  const calls = mockApi(() => ({ body: { items: [], total: 0, page: 1, size: 20 } }))
  renderApp(<OverviewPage />)
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
  renderApp(<OverviewPage />)
  expect(await screen.findByText('D-P1')).toBeInTheDocument()
  expect(screen.queryByText('D-P2')).not.toBeInTheDocument()
  await userEvent.setup().click(screen.getByRole('button', { name: '加载更多' }))
  expect(await screen.findByText('D-P2')).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '加载更多' })).not.toBeInTheDocument()
})

test('加载失败可重试', async () => {
  let fail = true
  mockApi(() => (fail ? { status: 500, body: { error: 'x' } } : { body: { items: [drawing()], total: 1, page: 1, size: 20 } }))
  renderApp(<OverviewPage />)
  expect(await screen.findByText('图纸加载失败')).toBeInTheDocument()
  fail = false
  await userEvent.setup().click(screen.getByRole('button', { name: '重试' }))
  expect(await screen.findByText('D-1001')).toBeInTheDocument()
})
