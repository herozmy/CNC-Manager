import { act, fireEvent, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Route, Routes } from 'react-router-dom'
import { afterEach, expect, test, vi } from 'vitest'
import { detail, mockApi, operation, renderApp } from '../test-utils'
import type { MachineOption, Operation } from '../api/types'
import DrawingPage from '../pages/DrawingPage'
import OperationSheet from './OperationSheet'

afterEach(() => vi.unstubAllGlobals())
const machines: MachineOption[] = [{ id: 5, code: 'CNC-01', name: '车床', enabled: true }, { id: 6, code: 'CNC-02', name: '加工中心', enabled: true }]
const view = <Routes><Route path="/drawings/:id" element={<DrawingPage />} /></Routes>

test('新建关联、详情显示编号名称，编辑回填、更换和清空，不改变程序', async () => {
  let ops: Operation[] = []
  const calls = mockApi((r) => {
    if (r.path === '/drawings/1/detail') return { body: detail(ops) }
    if (r.path === '/drawings/1/machines') return { body: machines }
    if (r.method === 'POST' || r.method === 'PUT') {
      const b = r.body as Partial<Operation>
      const machine = machines.find((m) => m.id === b.machineId)
      ops = [operation({ ...b, machineCode: machine?.code ?? '', machineName: machine?.name ?? '' })]
      return { body: ops[0] }
    }
  })
  renderApp(view, '/drawings/1')
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '新增第一道工序' }))
  await screen.findByRole('option', { name: 'CNC-01 · 车床' })
  await u.selectOptions(screen.getByLabelText('机台（可选）'), '5')
  await u.click(screen.getByRole('button', { name: '保存' }))
  await u.click(await screen.findByRole('button', { name: '展开工序 10' }))
  expect(await screen.findByText('机台：CNC-01 · 车床')).toBeVisible()
  expect(screen.getByRole('link', { name: /O1234/ })).toBeVisible()
  await u.click(screen.getByRole('button', { name: '编辑工序 10' }))
  expect(screen.getByLabelText('机台（可选）')).toHaveValue('5')
  await u.selectOptions(screen.getByLabelText('机台（可选）'), '6')
  await u.click(screen.getByRole('button', { name: '保存' }))
  expect(await screen.findByText('机台：CNC-02 · 加工中心')).toBeVisible()
  await u.click(screen.getByRole('button', { name: '编辑工序 10' }))
  await u.selectOptions(screen.getByLabelText('机台（可选）'), '')
  await u.click(screen.getByRole('button', { name: '保存' }))
  await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  expect(screen.queryByText(/机台：/)).not.toBeInTheDocument()
  expect(calls.filter((r) => r.method === 'PUT').at(-1)?.body).toMatchObject({ machineId: null })
  expect(screen.getByRole('link', { name: /O1234/ })).toBeVisible()
})

test('列表失败保留原关联并可重试；保存失败保留新选择', async () => {
  let listFail = true, saveFail = true
  const calls = mockApi((r) => {
    if (r.path === '/drawings/1/machines') return listFail ? { status: 500 } : { body: machines }
    if (r.method === 'PUT') return saveFail ? { status: 400, body: { error: '关联记录不存在或不属于当前账号' } } : { body: {} }
  })
  const closed = vi.fn()
  renderApp(<OperationSheet drawingId={1} operation={operation({ machineId: 5, machineName: '车床' })} onClose={closed} />)
  const u = userEvent.setup()
  await screen.findByRole('button', { name: '重试加载机台' })
  expect(screen.getByLabelText('机台（可选）')).toHaveValue('5')
  await u.click(screen.getByRole('button', { name: '保存' }))
  expect(calls.find((r) => r.method === 'PUT')?.body).toMatchObject({ machineId: 5 })
  listFail = false
  await u.click(screen.getByRole('button', { name: '重试加载机台' }))
  await screen.findByRole('option', { name: 'CNC-02 · 加工中心' })
  await u.selectOptions(screen.getByLabelText('机台（可选）'), '6')
  await u.click(screen.getByRole('button', { name: '保存' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('关联记录不存在')
  expect(screen.getByLabelText('机台（可选）')).toHaveValue('6')
  saveFail = false
  await u.click(screen.getByRole('button', { name: '保存' }))
  await waitFor(() => expect(closed).toHaveBeenCalledOnce())
})

test('新增机台失败保留输入，成功选中后单独保存工序', async () => {
  let fail = true
  const list: MachineOption[] = []
  const calls = mockApi((r) => {
    if (r.path === '/drawings/1/machines' && r.method === 'GET') return { body: list }
    if (r.path === '/drawings/1/machines') {
      if (fail) return { status: 409, body: { error: '该机台编号已存在' } }
      const m = { id: 8, enabled: true, ...(r.body as { code: string; name: string }) }; list.push(m); return { status: 201, body: m }
    }
    if (r.path === '/drawings/1/operations') return { body: {} }
  })
  const closed = vi.fn()
  renderApp(<OperationSheet drawingId={1} ownerUserId={2} onClose={closed} />)
  const u = userEvent.setup()
  await screen.findByText('暂无可选机台，可先不关联。')
  await u.click(screen.getByRole('button', { name: '新增机台' }))
  await u.type(screen.getByLabelText('机台编号'), ' CNC-03 ')
  await u.type(screen.getByLabelText('机台名称（可选）'), ' 新机台 ')
  await u.click(screen.getByRole('button', { name: '新增并选中' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('该机台编号已存在')
  expect(screen.getByLabelText('机台编号')).toHaveValue(' CNC-03 ')
  fail = false
  await u.click(screen.getByRole('button', { name: '新增并选中' }))
  await waitFor(() => expect(screen.getByLabelText('机台（可选）')).toHaveValue('8'))
  expect(closed).not.toHaveBeenCalled()
  expect(calls.some((r) => r.path.endsWith('/operations'))).toBe(false)
  await u.click(screen.getByRole('button', { name: '保存' }))
  await waitFor(() => expect(closed).toHaveBeenCalledOnce())
  expect(calls.at(-1)?.body).toMatchObject({ machineId: 8 })
})

test('新增机台等待时禁止关闭、重复提交及工序保存', async () => {
  let finish!: (response: Response) => void
  vi.stubGlobal('fetch', vi.fn((_: unknown, init?: RequestInit) => init?.method === 'POST'
    ? new Promise<Response>((resolve) => { finish = resolve }) : Promise.resolve(new Response('[]'))))
  const close = vi.fn()
  renderApp(<OperationSheet drawingId={1} ownerUserId={2} onClose={close} />)
  const u = userEvent.setup()
  await u.click(screen.getByRole('button', { name: '新增机台' }))
  await u.type(screen.getByLabelText('机台编号'), 'CNC-04')
  const form = screen.getByRole('button', { name: '保存' }).closest('form')!
  fireEvent.submit(form); fireEvent.submit(form)
  await u.click(screen.getByRole('button', { name: '关闭' })); await u.keyboard('{Escape}')
  expect(close).not.toHaveBeenCalled()
  expect(screen.getByLabelText('机台编号')).toBeDisabled()
  expect(vi.mocked(fetch).mock.calls.filter(([, init]) => init?.method === 'POST')).toHaveLength(1)
  await act(async () => finish(new Response(JSON.stringify({ id: 9, code: 'CNC-04', name: '' }))))
  await waitFor(() => expect(screen.getByRole('button', { name: '保存' })).toBeEnabled())
})

test('待分配图纸只允许选择已有机台，不显示新增入口', async () => {
  mockApi(() => ({ body: machines }))
  renderApp(<OperationSheet drawingId={1} ownerUserId={0} onClose={() => {}} />)
  expect(screen.queryByRole('button', { name: '新增机台' })).not.toBeInTheDocument()
  expect(await screen.findByRole('option', { name: 'CNC-01 · 车床' })).toBeInTheDocument()
})

test('停用机台仅保留当前关联，不提供其他停用机台供新选', async () => {
  const disabled = machines.map((m) => ({ ...m, enabled: false }))
  const calls = mockApi((r) => r.method === 'GET' ? { body: disabled } : { body: {} })
  renderApp(<OperationSheet drawingId={1} operation={operation({ machineId: 5 })} onClose={vi.fn()} />)
  const selected = await screen.findByRole('option', { name: /CNC-01.*已停用/ })
  expect(selected).toBeInTheDocument()
  expect(screen.queryByRole('option', { name: /CNC-02/ })).not.toBeInTheDocument()
  await userEvent.setup().click(screen.getByRole('button', { name: '保存' }))
  await waitFor(() => expect(calls.some((r) => r.method === 'PUT' && (r.body as { machineId: number }).machineId === 5)).toBe(true))
})
