import { act, fireEvent, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Route, Routes } from 'react-router-dom'
import { afterEach, expect, test, vi } from 'vitest'
import { detail, mockApi, operation, program, renderApp, version } from '../test-utils'
import type { ProgramInput } from '../api/types'
import ProgramPage from './ProgramPage'
import DrawingPage from './DrawingPage'

afterEach(() => vi.unstubAllGlobals())

function renderPage() {
  return renderApp(<Routes>
    <Route path="/drawings/:id/programs/:programId" element={<ProgramPage />} />
    <Route path="/drawings/:id" element={<DrawingPage />} />
  </Routes>, '/drawings/1/programs/1')
}

function server(extra?: Parameters<typeof mockApi>[0]) {
  let item = program({ remark: '原备注' })
  return mockApi((r) => {
    const custom = extra?.(r)
    if (custom) return custom
    if (r.path === '/drawings/1/detail') return { body: detail([operation({ programs: [item] })]) }
    if (r.path === '/programs/1/versions') return { body: [version(), version({ id: 4, versionNo: 2, isCurrent: false })] }
    if (r.path.match(/^\/versions\/\d+\/tools$/)) return { body: { items: [], revision: 'empty' } }
    if (r.method === 'PUT' && r.path === '/programs/1') {
      item = { ...item, ...r.body as ProgramInput }
      return { body: item }
    }
  })
}

async function open() {
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '编辑程序信息' }))
  return u
}

test('回填并保存四个字段，刷新详情与图纸列表、搜索缓存，保留版本选择和下载', async () => {
  const calls = server()
  const { client } = renderPage()
  client.setQueryDefaults(['drawings'], { gcTime: Infinity })
  client.setQueryData(['drawings', 'O1234', 1], { items: [] })
  const u = userEvent.setup()
  await u.selectOptions(await screen.findByLabelText('选择 NC 程序版本'), '4')
  await u.click(screen.getByRole('button', { name: '编辑程序信息' }))
  for (const [label, old, value] of [['程序号', 'O1234', ' O9999 '], ['程序名称', '粗车', ' 精加工 '], ['数控系统', 'FANUC', ' SIEMENS '], ['备注', '原备注', '注意装夹']]) {
    expect(screen.getByLabelText(label)).toHaveValue(old)
    await u.clear(screen.getByLabelText(label))
    await u.type(screen.getByLabelText(label), value)
  }
  await u.click(screen.getByRole('button', { name: '保存修改' }))
  expect(await screen.findByText('程序信息已保存')).toBeInTheDocument()
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  expect(screen.getByRole('heading', { name: 'O9999' })).toBeInTheDocument()
  expect(screen.getByText('精加工 · SIEMENS')).toBeInTheDocument()
  expect(screen.getByText('注意装夹')).toBeInTheDocument()
  expect(screen.getByLabelText('选择 NC 程序版本')).toHaveValue('4')
  expect(screen.getByRole('link', { name: '下载当前版本' })).toHaveAttribute('href', '/api/versions/5/download')
  expect(screen.getByRole('link', { name: '下载 V2' })).toHaveAttribute('href', '/api/versions/4/download')
  expect(client.getQueryState(['drawings', 'O1234', 1])?.isInvalidated).toBe(true)
  expect(calls.filter((r) => r.method !== 'GET')).toEqual([{ method: 'PUT', path: '/programs/1', body: { expectedRevision: 1, programNo: 'O9999', programName: '精加工', controller: 'SIEMENS', remark: '注意装夹' } }])
  await u.click(screen.getByRole('link', { name: '返回图纸' }))
  await u.click(await screen.findByRole('button', { name: '展开工序 10' }))
  expect(screen.getByText('O9999')).toBeInTheDocument()
  expect(screen.queryByText('O1234')).not.toBeInTheDocument()
})

test('空程序号及 UTF-8 字节超长提交前拒绝，允许清空选填字段', async () => {
  const calls = server()
  renderPage()
  const u = await open()
  await u.clear(screen.getByLabelText('程序号'))
  await u.click(screen.getByRole('button', { name: '保存修改' }))
  expect(screen.getByRole('alert')).toHaveTextContent('请输入程序号')
  await u.type(screen.getByLabelText('程序号'), 'O1234')
  fireEvent.change(screen.getByLabelText('程序名称'), { target: { value: '中'.repeat(34) } })
  fireEvent.change(screen.getByLabelText('备注'), { target: { value: '注'.repeat(667) } })
  await u.click(screen.getByRole('button', { name: '保存修改' }))
  expect(screen.getByText('程序名称不能超过 100 字节')).toBeInTheDocument()
  expect(screen.getByText('备注不能超过 2000 字节')).toBeInTheDocument()
  expect(calls.filter((r) => r.method !== 'GET')).toHaveLength(0)
  for (const label of ['程序名称', '数控系统', '备注']) await u.clear(screen.getByLabelText(label))
  await u.click(screen.getByRole('button', { name: '保存修改' }))
  await screen.findByText('程序信息已保存')
  expect(calls.find((r) => r.method === 'PUT')?.body).toEqual({ expectedRevision: 1, programNo: 'O1234', programName: '', controller: '', remark: '' })
})

test.each([500])('保存失败 %i 保留输入，可重试', async (status) => {
  let fail = true
  const calls = server((r) => fail && r.method === 'PUT' ? { status, body: { error: status === 409 ? '数据冲突: 程序号重复' : '保存失败' } } : undefined)
  renderPage()
  const u = await open()
  await u.clear(screen.getByLabelText('程序号'))
  await u.type(screen.getByLabelText('程序号'), 'O9999')
  await u.click(screen.getByRole('button', { name: '保存修改' }))
  expect(await screen.findByRole('alert')).toHaveTextContent(status === 409 ? '程序号重复' : '保存失败')
  expect(screen.getByLabelText('程序号')).toHaveValue('O9999')
  expect(screen.getByLabelText('备注')).toHaveValue('原备注')
  fail = false
  await u.click(screen.getByRole('button', { name: '保存修改' }))
  await screen.findByText('程序信息已保存')
  expect(calls.filter((r) => r.method === 'PUT')).toHaveLength(2)
})

test('取消未修改表单直接退出；有修改可继续或放弃，再次打开回填原值', async () => {
  const calls = server()
  renderPage()
  const u = await open()
  await u.click(screen.getByRole('button', { name: '取消' }))
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  await u.click(screen.getByRole('button', { name: '编辑程序信息' }))
  await u.type(screen.getByLabelText('备注'), '未保存')
  await u.keyboard('{Escape}')
  expect(screen.getByText('有未保存的修改，确定放弃吗？')).toBeInTheDocument()
  await u.click(screen.getByRole('button', { name: '继续编辑' }))
  expect(screen.getByLabelText('备注')).toHaveValue('原备注未保存')
  await u.click(screen.getByRole('button', { name: '关闭' }))
  await u.click(screen.getByRole('button', { name: '放弃修改' }))
  await u.click(screen.getByRole('button', { name: '编辑程序信息' }))
  expect(screen.getByLabelText('备注')).toHaveValue('原备注')
  expect(calls.filter((r) => r.method !== 'GET')).toHaveLength(0)
})

test('保存期间禁用输入、取消和重复提交，并阻止关闭', async () => {
  server()
  const originalFetch = globalThis.fetch
  let release!: () => void
  const gate = new Promise<void>((resolve) => { release = resolve })
  const fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    if (init?.method === 'PUT') await gate
    return originalFetch(input, init)
  })
  vi.stubGlobal('fetch', fetch)
  renderPage()
  const u = await open()
  const dialog = screen.getByRole('dialog', { name: '编辑程序信息' })
  await u.click(screen.getByRole('button', { name: '保存修改' }))
  expect(screen.getByLabelText('程序号')).toBeDisabled()
  expect(screen.getByRole('button', { name: '保存修改' })).toBeDisabled()
  expect(screen.getByRole('button', { name: '取消' })).toBeDisabled()
  fireEvent.submit(dialog.querySelector('form')!)
  await u.keyboard('{Escape}')
  await u.click(screen.getByRole('button', { name: '关闭' }))
  expect(dialog).toBeInTheDocument()
  expect(fetch.mock.calls.filter(([, init]) => init?.method === 'PUT')).toHaveLength(1)
  await act(async () => release())
  await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
})
