import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Route, Routes } from 'react-router-dom'
import { afterEach, expect, test, vi } from 'vitest'
import { api } from '../api'
import type { Drawing, DrawingInput } from '../api/types'
import { detail, drawing, mockApi, operation, renderApp } from '../test-utils'
import DrawingPage from './DrawingPage'
import OverviewPage from './OverviewPage'

afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks() })

const view = <Routes>
  <Route path="/" element={<OverviewPage />} />
  <Route path="/drawings/:id" element={<DrawingPage />} />
</Routes>

function server(errorStatus = 0, price: string | null = null) {
  let current = detail([operation()], { price })
  let fail = errorStatus
  const calls = mockApi((r) => {
    if (r.method === 'GET' && r.path === '/drawings/1/detail') return { body: current }
    if (r.method === 'GET' && r.path.startsWith('/drawings?')) return { body: { items: [current], page: 1, size: 20, total: 1 } }
    if (r.method === 'PUT' && r.path === '/drawings/1') {
      if (fail) {
        const status = fail
        fail = 0
        return { status, body: { error: status === 409 ? '记录已存在: 该图纸号已存在' : '保存失败，请重试' } }
      }
      const input = r.body as DrawingInput
      current = { ...current, ...input, price: input.price.trim() ? Number(input.price).toFixed(2) : null }
      return { body: drawing(current) }
    }
  })
  return calls
}

async function edit() {
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '编辑图纸' }))
  return { u, dialog: within(screen.getByRole('dialog', { name: '编辑图纸' })) }
}

test('价格回填、失败保留草稿重试，保存后详情和再次编辑显示新价格', async () => {
  const calls = server(500, '12.30')
  renderApp(view, '/drawings/1')
  expect(await screen.findByText('价格：12.30 元')).toBeInTheDocument()
  const { u, dialog } = await edit()
  expect(dialog.getByLabelText('价格（元）')).toHaveValue('12.30')
  await u.clear(dialog.getByLabelText('价格（元）'))
  await u.type(dialog.getByLabelText('价格（元）'), '0.29')
  await u.click(dialog.getByRole('button', { name: '保存修改' }))
  expect(await screen.findByText('保存失败，请重试')).toBeInTheDocument()
  expect(dialog.getByLabelText('价格（元）')).toHaveValue('0.29')
  await u.click(dialog.getByRole('button', { name: '保存修改' }))
  expect(await screen.findByText('价格：0.29 元')).toBeInTheDocument()
  await edit()
  expect(screen.getByLabelText('价格（元）')).toHaveValue('0.29')
  expect(calls.filter(r => r.method === 'PUT').every(r => (r.body as DrawingInput).price === '0.29')).toBe(true)
})

test('价格区分未填写和零，允许清空；无效编辑不提交', async () => {
  const calls = server()
  renderApp(view, '/drawings/1')
  expect(await screen.findByText('价格：未填写')).toBeInTheDocument()
  const { u, dialog } = await edit()
  await u.type(dialog.getByLabelText('价格（元）'), '-1')
  await u.click(dialog.getByRole('button', { name: '保存修改' }))
  expect(await screen.findByText('价格应为 0~999999999.99 元，最多两位小数')).toBeInTheDocument()
  expect(calls.filter(r => r.method === 'PUT')).toHaveLength(0)
  await u.clear(dialog.getByLabelText('价格（元）'))
  await u.type(dialog.getByLabelText('价格（元）'), '0')
  await u.click(dialog.getByRole('button', { name: '保存修改' }))
  expect(await screen.findByText('价格：0.00 元')).toBeInTheDocument()
  await edit()
  await u.clear(screen.getByLabelText('价格（元）'))
  await u.click(screen.getByRole('button', { name: '保存修改' }))
  expect(await screen.findByText('价格：未填写')).toBeInTheDocument()
})

test('编辑已有图纸：回填字段，保存后详情和总览更新，保留工序程序及停用字段', async () => {
  const calls = server()
  renderApp(view)
  const u = userEvent.setup()
  await u.click(await screen.findByRole('link', { name: /D-1001/ }))
  expect(await screen.findByRole('heading', { name: 'D-1001' })).toBeInTheDocument()
  expect(screen.queryByText(/客户A|版本 B/)).not.toBeInTheDocument()
  const { dialog } = await edit()
  expect(dialog.getByLabelText('图纸号')).toHaveValue('D-1001')
  expect(dialog.getByLabelText('类型')).toHaveValue('壳体')
  expect(dialog.getByLabelText('材料')).toHaveValue('6061')
  expect(dialog.queryByLabelText('客户')).not.toBeInTheDocument()
  expect(dialog.queryByLabelText('图纸版本')).not.toBeInTheDocument()
  for (const [label, value] of [['图纸号', 'D-2002'], ['类型', '轴类'], ['材料', '45#'], ['备注', '调整装夹']]) {
    await u.clear(dialog.getByLabelText(label))
    await u.type(dialog.getByLabelText(label), value)
  }
  await u.click(dialog.getByRole('button', { name: '保存修改' }))
  expect(await screen.findByText('图纸已保存')).toBeInTheDocument()
  expect(screen.getByRole('heading', { name: 'D-2002' })).toBeInTheDocument()
  expect(screen.getByText('类型：轴类')).toBeInTheDocument()
  expect(screen.getByText('45#')).toBeInTheDocument()
  expect(screen.getByText('调整装夹')).toBeInTheDocument()
  expect(screen.getByRole('heading', { name: '10# 粗车' })).toBeInTheDocument()
  await u.click(screen.getByRole('button', { name: '展开工序 10' }))
  expect(screen.getByRole('link', { name: /O1234/ })).toBeInTheDocument()
  expect(calls.find((r) => r.method === 'PUT')?.body).toEqual({ drawingNo: 'D-2002', name: '轴类', material: '45#', price: '', remark: '调整装夹', customer: '客户A', drawingVersion: 'B' })
  await u.click(screen.getByRole('link', { name: '返回总览' }))
  expect(await screen.findByRole('link', { name: /D-2002.*类型：轴类/ })).toBeInTheDocument()
  expect(screen.queryByText('D-1001')).not.toBeInTheDocument()
})

test('取消未修改表单直接关闭，修改后可继续编辑或放弃，均不保存', async () => {
  const calls = server()
  renderApp(view, '/drawings/1')
  const { u, dialog } = await edit()
  await u.click(dialog.getByRole('button', { name: '取消' }))
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  await edit()
  await u.type(screen.getByLabelText('类型'), '草稿')
  await u.keyboard('{Escape}')
  expect(screen.getByText('有未保存的修改，确定放弃吗？')).toBeInTheDocument()
  await u.click(screen.getByRole('button', { name: '继续编辑' }))
  expect(screen.getByLabelText('类型')).toHaveValue('壳体草稿')
  await u.click(screen.getByRole('button', { name: '取消' }))
  await u.click(screen.getByRole('button', { name: '放弃修改' }))
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  await edit()
  expect(screen.getByLabelText('类型')).toHaveValue('壳体')
  expect(calls.filter((r) => r.method === 'PUT')).toHaveLength(0)
})

test('编辑时图纸号必填，其他字段可以清空', async () => {
  const calls = server()
  renderApp(view, '/drawings/1')
  const { u, dialog } = await edit()
  await u.clear(dialog.getByLabelText('图纸号'))
  await u.click(dialog.getByRole('button', { name: '保存修改' }))
  expect(screen.getByText('请输入图纸号')).toBeInTheDocument()
  expect(calls.filter((r) => r.method === 'PUT')).toHaveLength(0)
  await u.type(dialog.getByLabelText('图纸号'), 'D-1001')
  await u.clear(dialog.getByLabelText('类型'))
  await u.clear(dialog.getByLabelText('材料'))
  await u.click(dialog.getByRole('button', { name: '保存修改' }))
  expect(await screen.findByText('类型：未分类')).toBeInTheDocument()
})

test.each([409, 500])('保存错误 %s 保留草稿并允许修正重试', async (status) => {
  server(status)
  renderApp(view, '/drawings/1')
  const { u, dialog } = await edit()
  await u.clear(dialog.getByLabelText('图纸号'))
  await u.type(dialog.getByLabelText('图纸号'), 'D-NEW')
  await u.type(dialog.getByLabelText('备注'), '保留草稿')
  await u.click(dialog.getByRole('button', { name: '保存修改' }))
  expect(await screen.findByRole('alert')).toHaveTextContent(status === 409 ? '该图纸号已存在' : '保存失败，请重试')
  expect(dialog.getByLabelText('图纸号')).toHaveValue('D-NEW')
  expect(dialog.getByLabelText('备注')).toHaveValue('保留草稿')
  if (status === 409) expect(dialog.getByLabelText('图纸号')).toHaveAttribute('aria-invalid', 'true')
  await u.click(dialog.getByRole('button', { name: '保存修改' }))
  expect(await screen.findByText('图纸已保存')).toBeInTheDocument()
})

test('保存中禁止重复提交、编辑和关闭', async () => {
  server()
  let finish!: (value: Drawing) => void
  const save = vi.spyOn(api, 'updateDrawing').mockImplementation(() => new Promise<Drawing>((resolve) => { finish = resolve }))
  renderApp(view, '/drawings/1')
  const { u, dialog } = await edit()
  await u.dblClick(dialog.getByRole('button', { name: '保存修改' }))
  expect(save).toHaveBeenCalledTimes(1)
  expect(dialog.getByLabelText('图纸号')).toBeDisabled()
  expect(dialog.getByRole('button', { name: '取消' })).toBeDisabled()
  await u.click(dialog.getByRole('button', { name: '关闭' }))
  await u.keyboard('{Escape}')
  expect(screen.getByRole('dialog', { name: '编辑图纸' })).toBeInTheDocument()
  await act(async () => finish(drawing()))
  await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
})
