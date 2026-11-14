import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Route, Routes } from 'react-router-dom'
import { afterEach, expect, test, vi } from 'vitest'
import { drawing, mockApi, renderApp } from '../test-utils'
import NewDrawingPage from './NewDrawingPage'

afterEach(() => vi.unstubAllGlobals())

function page() {
  return (
    <Routes>
      <Route path="/drawings/new" element={<NewDrawingPage />} />
      <Route path="/drawings/:id" element={<p>详情页</p>} />
    </Routes>
  )
}

test('图纸号为空时不提交并提示', async () => {
  const calls = mockApi(() => undefined)
  renderApp(page(), '/drawings/new')
  await userEvent.setup().click(screen.getByRole('button', { name: '创建图纸' }))
  expect(await screen.findByText('请输入图纸号')).toBeInTheDocument()
  expect(calls).toHaveLength(0)
})

test('创建成功后跳转到详情页，并提交全部字段', async () => {
  const calls = mockApi((r) => (r.method === 'POST' ? { status: 201, body: drawing({ id: 7 }) } : undefined))
  renderApp(page(), '/drawings/new')
  const u = userEvent.setup()
  await u.type(screen.getByLabelText('图纸号'), 'D-2001')
  await u.type(screen.getByLabelText('类型'), '端盖类')
  expect(screen.queryByLabelText('客户')).not.toBeInTheDocument()
  await u.type(screen.getByLabelText('材料'), '45#')
  await u.type(screen.getByLabelText('价格（元）'), '12.30')
  expect(screen.queryByLabelText('图纸版本')).not.toBeInTheDocument()
  await u.type(screen.getByLabelText('备注'), '急件')
  await u.click(screen.getByRole('button', { name: '创建图纸' }))
  expect(await screen.findByText('详情页')).toBeInTheDocument()
  expect(calls[0]).toMatchObject({
    method: 'POST',
    path: '/drawings',
    body: { drawingNo: 'D-2001', name: '端盖类', customer: '', material: '45#', price: '12.30', drawingVersion: '', remark: '急件' },
  })
})

test.each(['-1', '12.345', 'abc', '1e2', '1000000000'])('无效价格 %s 保留输入并阻止提交', async (price) => {
  const calls = mockApi(() => undefined)
  renderApp(page(), '/drawings/new')
  const u = userEvent.setup()
  await u.type(screen.getByLabelText('图纸号'), 'INVALID-PRICE')
  await u.type(screen.getByLabelText('价格（元）'), price)
  await u.click(screen.getByRole('button', { name: '创建图纸' }))
  expect(await screen.findByText('价格应为 0~999999999.99 元，最多两位小数')).toBeInTheDocument()
  expect(screen.getByLabelText('价格（元）')).toHaveValue(price)
  expect(calls).toHaveLength(0)
})

test('图纸号重复时标在输入框上，且停留在表单', async () => {
  mockApi(() => ({ status: 409, body: { error: '记录已存在，或与现有数据冲突: 该图纸号已存在' } }))
  renderApp(page(), '/drawings/new')
  const u = userEvent.setup()
  await u.type(screen.getByLabelText('图纸号'), 'D-1')
  await u.click(screen.getByRole('button', { name: '创建图纸' }))
  expect(await screen.findByText('该图纸号已存在')).toBeInTheDocument()
  expect(screen.getByLabelText('图纸号')).toHaveAttribute('aria-invalid', 'true')
  expect(screen.queryByText('详情页')).not.toBeInTheDocument()
})

test('其他错误显示在表单底部', async () => {
  mockApi(() => ({ status: 400, body: { error: '参数不合法: 类型 不能超过 100 字节' } }))
  renderApp(page(), '/drawings/new')
  await userEvent.setup().type(screen.getByLabelText('图纸号'), 'D-1')
  await userEvent.setup().click(screen.getByRole('button', { name: '创建图纸' }))
  expect(await screen.findByText(/类型 不能超过/)).toBeInTheDocument()
})
