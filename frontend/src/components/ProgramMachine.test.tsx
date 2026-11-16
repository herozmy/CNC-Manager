import { act, fireEvent, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, test, vi } from 'vitest'
import type { MachineOption, Program } from '../api/types'
import { mockApi, program, renderApp } from '../test-utils'
import ProgramSheet from './ProgramSheet'

const machines: MachineOption[] = [{ id: 5, code: 'A', name: '车床', enabled: true }, { id: 6, code: 'B', name: '铣床', enabled: true }, { id: 7, code: 'OLD', name: '', enabled: false }]
afterEach(() => vi.unstubAllGlobals())
const existing = () => program({ machineId: 5, machineCode: 'A', machineName: '车床' }) as Program

test('程序独立选择机台，保存失败保留选择，重试只更新程序并刷新机台关联缓存', async () => {
  let fail = true
  const calls = mockApi((r) => {
    if (r.path === '/drawings/1/machines') return { body: machines }
    if (r.path === '/programs/1' && r.method === 'PUT') return fail ? { status: 503, body: { error: '保存失败' } } : { body: program({ ...r.body as object }) }
  })
  const closed = vi.fn()
  const { client } = renderApp(<ProgramSheet drawingId={1} operationId={1} ownerUserId={2} program={existing()} onClose={closed} />)
  client.setQueryDefaults(['machine-drawings'], { gcTime: Infinity })
  client.setQueryData(['machine-drawings', 5, ''], {})
  const u = userEvent.setup()
  await screen.findByRole('option', { name: 'B · 铣床' })
  expect(screen.queryByRole('option', { name: 'OLD' })).not.toBeInTheDocument()
  expect(screen.getByLabelText('程序机台（可选）')).toHaveValue('5')
  await u.selectOptions(screen.getByLabelText('程序机台（可选）'), '6')
  await u.click(screen.getByRole('button', { name: '保存修改' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('保存失败')
  expect(screen.getByLabelText('程序机台（可选）')).toHaveValue('6')
  fail = false
  await u.click(screen.getByRole('button', { name: '保存修改' }))
  await waitFor(() => expect(closed).toHaveBeenCalledOnce())
  expect(calls.filter((r) => r.method !== 'GET').map((r) => [r.path, r.body])).toEqual(Array(2).fill(['/programs/1', { programNo: 'O1234', programName: '粗车', controller: 'FANUC', remark: '', machineId: 6, expectedRevision: 1 }]))
  expect(client.getQueryState(['machine-drawings', 5, ''])?.isInvalidated).toBe(true)
})

test('机台列表失败保留已有停用关联，重试加载后可解除关联', async () => {
  let fail = true
  const calls = mockApi((r) => {
    if (r.path === '/drawings/1/machines') return fail ? { status: 500 } : { body: machines }
    if (r.method === 'PUT') return { body: program() }
  })
  renderApp(<ProgramSheet drawingId={1} operationId={1} program={program({ machineId: 7, machineCode: 'OLD' }) as Program} onClose={vi.fn()} />)
  const u = userEvent.setup()
  await screen.findByRole('button', { name: '重试加载机台' })
  expect(screen.getByLabelText('程序机台（可选）')).toHaveValue('7')
  fail = false
  await u.click(screen.getByRole('button', { name: '重试加载机台' }))
  await screen.findByRole('option', { name: 'OLD（已停用，当前关联）' })
  await u.selectOptions(screen.getByLabelText('程序机台（可选）'), '')
  await u.click(screen.getByRole('button', { name: '保存修改' }))
  await waitFor(() => expect(calls.find((r) => r.method === 'PUT')?.body).toMatchObject({ machineId: null }))
})

test('在程序表单内新增机台，创建失败保留草稿，成功自动选中并可供后续程序选择', async () => {
  let options = [...machines], fail = true
  const calls = mockApi((r) => {
    if (r.path === '/drawings/1/machines' && r.method === 'GET') return { body: options }
    if (r.path === '/drawings/1/machines' && r.method === 'POST') {
      if (fail) return { status: 409, body: { error: '编号已存在' } }
      const m = { id: 8, code: 'NEW', name: '新机台', enabled: true }; options.push(m); return { status: 201, body: m }
    }
    if (r.method === 'PUT') return { body: program({ ...r.body as object }) }
  })
  const saved = vi.fn()
  const view = renderApp(<ProgramSheet drawingId={1} operationId={1} program={existing()} onClose={saved} />)
  const u = userEvent.setup()
  await u.click(screen.getByRole('button', { name: '新增机台' }))
  await u.type(screen.getByLabelText('机台编号'), 'NEW')
  await u.type(screen.getByLabelText('机台名称（可选）'), '新机台')
  fireEvent.submit(screen.getByRole('dialog').querySelector('form')!)
  expect(calls.some((r) => r.method === 'PUT')).toBe(false)
  await u.click(screen.getByRole('button', { name: '新增并选中' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('编号已存在')
  expect(screen.getByLabelText('机台编号')).toHaveValue('NEW')
  fail = false
  await u.click(screen.getByRole('button', { name: '新增并选中' }))
  await screen.findByRole('option', { name: 'NEW · 新机台' })
  expect(screen.getByLabelText('程序机台（可选）')).toHaveValue('8')
  await u.click(screen.getByRole('button', { name: '保存修改' }))
  await waitFor(() => expect(saved).toHaveBeenCalledOnce())
  expect(calls.find((r) => r.method === 'PUT')?.body).toMatchObject({ machineId: 8 })
  view.unmount()
  renderApp(<ProgramSheet drawingId={1} operationId={1} onClose={vi.fn()} />)
  await screen.findByRole('option', { name: 'NEW · 新机台' })
  expect(screen.getByLabelText('程序机台（可选）')).toHaveValue('')
  options = []
})

test('新增机台期间阻止重复创建、关闭及提交程序，网络失败保留两个草稿', async () => {
  mockApi((r) => r.path === '/drawings/1/machines' ? { body: machines } : undefined)
  const original = fetch
  let reject!: (reason: unknown) => void
  const gated = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => init?.method === 'POST' ? new Promise<Response>((_, no) => { reject = no }) : original(input, init))
  vi.stubGlobal('fetch', gated)
  const closed = vi.fn()
  renderApp(<ProgramSheet drawingId={1} operationId={1} onClose={closed} />)
  const u = userEvent.setup()
  await u.type(screen.getByLabelText('程序号'), 'O-NEW')
  await u.click(screen.getByRole('button', { name: '新增机台' }))
  await u.type(screen.getByLabelText('机台编号'), 'NEW')
  await u.dblClick(screen.getByRole('button', { name: '新增并选中' }))
  await u.keyboard('{Escape}')
  await u.click(screen.getByRole('button', { name: '关闭' }))
  fireEvent.submit(screen.getByRole('dialog').querySelector('form')!)
  expect(closed).not.toHaveBeenCalled()
  expect(gated.mock.calls.filter(([, init]) => init?.method === 'POST')).toHaveLength(1)
  await act(async () => reject(new TypeError('offline')))
  expect(await screen.findByRole('alert')).toHaveTextContent('机台创建失败')
  expect(screen.getByLabelText('程序号')).toHaveValue('O-NEW')
  expect(screen.getByLabelText('机台编号')).toHaveValue('NEW')
  await u.click(screen.getByRole('button', { name: '取消' }))
  expect(screen.getByText('有未保存的修改，确定放弃吗？')).toBeInTheDocument()
})

test('在机台管理中新建后，程序表单刷新即可选择；待分配图纸不能新增机台', async () => {
  let options = [...machines]
  mockApi((r) => r.path === '/drawings/1/machines' ? { body: options } : undefined)
  renderApp(<ProgramSheet drawingId={1} operationId={1} ownerUserId={0} onClose={vi.fn()} />)
  await screen.findByRole('option', { name: 'A · 车床' })
  expect(screen.queryByRole('button', { name: '新增机台' })).not.toBeInTheDocument()
  options = [...options, { id: 8, code: 'EXTERNAL', name: '', enabled: true }]
  await userEvent.setup().click(screen.getByRole('button', { name: '刷新机台列表' }))
  expect(await screen.findByRole('option', { name: 'EXTERNAL' })).toBeInTheDocument()
})
