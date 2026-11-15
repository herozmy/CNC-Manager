import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Route, Routes } from 'react-router-dom'
import { afterEach, expect, test, vi } from 'vitest'
import { detail, drawing, mockApi, operation, program, renderApp } from '../test-utils'
import DrawingPage from './DrawingPage'

afterEach(() => vi.unstubAllGlobals())

const op = operation
const view = () => (
  <Routes>
    <Route path="/drawings/:id" element={<DrawingPage />} />
  </Routes>
)

test('工序默认收起，按需独立展开详情和操作，可再次收起', async () => {
  mockApi(() => ({ body: detail([op({ machineId: 5, machineName: 'CK6150', centeringMethod: '四面分中' }), op({ id: 2, opNo: 20, opName: '精车', zHeight: 0, programs: [program({ id: 2, programNo: 'O2', currentVersionNo: null })] })], { remark: '注意变形' }) }))
  renderApp(view(), '/drawings/1')
  const u = userEvent.setup()
  expect(await screen.findByRole('heading', { name: 'D-1001' })).toBeInTheDocument()
  expect(screen.getByText('注意变形')).toBeInTheDocument()
  expect(screen.getByRole('heading', { name: '10# 粗车' })).toBeInTheDocument()
  expect(screen.getAllByText('1 个程序')).toHaveLength(2)
  expect(screen.getByRole('button', { name: '展开工序 10' })).toHaveAttribute('aria-expanded', 'false')
  expect(screen.queryByRole('region', { name: '工序 10 详情' })).not.toBeInTheDocument()
  expect(screen.queryByText('X（mm）')).not.toBeInTheDocument()
  expect(screen.queryByText('分中方式：四面分中')).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '编辑工序 10' })).not.toBeInTheDocument()
  expect(screen.queryByRole('link', { name: /O1234/ })).not.toBeInTheDocument()
  await u.click(screen.getByRole('button', { name: '展开工序 10' }))
  expect(screen.getByRole('button', { name: '收起工序 10' })).toHaveAttribute('aria-expanded', 'true')
  expect(screen.getByRole('region', { name: '工序 10 详情' })).toBeVisible()
  expect(screen.getByText('分中方式：四面分中')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '编辑工序 10' })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '删除工序 10' })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '在 10# 下新增程序' })).toBeInTheDocument()
  expect(screen.getAllByText('X（mm）')).toHaveLength(1)
  expect(screen.queryByText('无版本')).not.toBeInTheDocument()
  await u.click(screen.getByRole('button', { name: '展开工序 20' }))
  expect(screen.getByText('机台：CK6150 · 装夹 三爪')).toBeInTheDocument()
  expect(screen.getAllByText('X（mm）')).toHaveLength(2)
  expect(screen.getAllByText('Y（mm）')).toHaveLength(2)
  expect(screen.getAllByText('Z（mm）')).toHaveLength(2)
  expect(screen.getByText('25.5')).toBeInTheDocument()
  expect(screen.getByText('V3')).toBeInTheDocument()
  expect(screen.getByText('无版本')).toBeInTheDocument()
  const collapse = screen.getByRole('button', { name: '收起工序 10' })
  collapse.focus()
  await u.keyboard('{Enter}')
  expect(screen.queryByRole('region', { name: '工序 10 详情' })).not.toBeInTheDocument()
  expect(screen.queryByText('分中方式：四面分中')).not.toBeInTheDocument()
  expect(screen.queryByText('V3')).not.toBeInTheDocument()
  expect(screen.getByRole('region', { name: '工序 20 详情' })).toBeVisible()
  await u.keyboard(' ')
  expect(screen.getByRole('region', { name: '工序 10 详情' })).toBeVisible()
})

test('无工序、无程序的空状态', async () => {
  mockApi(() => ({ body: { ...drawing(), operations: [] } }))
  renderApp(view(), '/drawings/1')
  expect(await screen.findByText('暂无工序')).toBeInTheDocument()
})

test('工序下没有程序时提示', async () => {
  mockApi(() => ({ body: { ...drawing(), operations: [op({ programs: [] })] } }))
  renderApp(view(), '/drawings/1')
  expect(await screen.findByText('0 个程序')).toBeInTheDocument()
  expect(screen.queryByText('暂无程序')).not.toBeInTheDocument()
  await userEvent.setup().click(screen.getByRole('button', { name: '展开工序 10' }))
  expect(await screen.findByText('暂无程序')).toBeInTheDocument()
})

test('图纸不存在显示提示；非法 ID 不发请求', async () => {
  mockApi(() => ({ status: 404, body: { error: '记录不存在' } }))
  const first = renderApp(view(), '/drawings/999')
  expect(await screen.findByText('图纸不存在或已被删除')).toBeInTheDocument()
  first.unmount()

  const calls = mockApi(() => undefined)
  renderApp(view(), '/drawings/abc')
  expect(await screen.findByText('图纸不存在或已被删除')).toBeInTheDocument()
  expect(calls).toHaveLength(0)
})

test('服务器错误可重试', async () => {
  mockApi(() => ({ status: 500, body: { error: 'x' } }))
  renderApp(view(), '/drawings/1')
  expect(await screen.findByText('图纸加载失败')).toBeInTheDocument()
})
