import { act, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Route, Routes } from 'react-router-dom'
import { afterEach, expect, test, vi } from 'vitest'
import { detail, mockApi, operation, program, renderApp, version } from '../test-utils'
import DrawingPage from './DrawingPage'
import ProgramPage from './ProgramPage'
import DeleteSheet from '../components/DeleteSheet'

afterEach(() => vi.unstubAllGlobals())
const cases = [
  { name: '工序', button: '删除工序 10', path: '/operations/1', route: '/drawings/1', outcome: '暂无工序' },
  { name: '程序', button: '删除程序', path: '/programs/1', route: '/drawings/1/programs/1', outcome: '暂无程序' },
]
function setup(fail = false) {
  let removed = ''
  let failNext = fail
  const calls = mockApi((r) => {
    if (r.method === 'DELETE') {
      if (failNext) { failNext = false; return { status: 500, body: { error: '暂时无法删除' } } }
      removed = r.path
      return { status: 204 }
    }
    if (r.path === '/drawings/1/detail') return removed === '/drawings/1' ? { status: 404 } : { body: detail(removed === '/operations/1' ? [] : [operation({ programs: removed === '/programs/1' ? [] : [program()] })]) }
    if (r.path === '/programs/1/versions') return { body: [version()] }
    if (r.path === '/versions/5/tools') return { body: { items: [], revision: 'empty' } }
  })
  return calls
}
function view(route: string) {
  return renderApp(<Routes>
    <Route path="/" element={<h1>总览测试页</h1>} />
    <Route path="/drawings/:id" element={<DrawingPage />} />
    <Route path="/drawings/:id/programs/:programId" element={<ProgramPage />} />
  </Routes>, route)
}

test('图纸详情页不显示删除图纸入口', async () => {
  setup()
  view('/drawings/1')
  expect(await screen.findByRole('heading', { name: 'D-1001' })).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '删除图纸' })).not.toBeInTheDocument()
})

for (const c of cases) {
  test(`${c.name}删除：取消不提交，确认后删除并刷新/返回父级`, async () => {
    const calls = setup()
    const { client } = view(c.route)
    client.setQueryDefaults(['drawings'], { gcTime: Infinity })
    client.setQueryDefaults(['version-content'], { gcTime: Infinity })
    client.setQueryData(['drawings', '', 1], { items: ['old'] })
    client.setQueryData(['version-content', 5], { content: 'old' })
    const u = userEvent.setup()
    await u.click(await screen.findByRole('button', { name: c.button }))
    expect(screen.getByRole('dialog', { name: `删除${c.name}` })).toHaveTextContent('无法恢复')
    expect(calls.filter((r) => r.method === 'DELETE')).toHaveLength(0)
    await u.click(screen.getByRole('button', { name: '取消' }))
    expect(calls.filter((r) => r.method === 'DELETE')).toHaveLength(0)
    if (c.name === '工序') expect(screen.getByRole('button', { name: '展开工序 10' })).toHaveAttribute('aria-expanded', 'false')
    await u.click(screen.getByRole('button', { name: c.button }))
    await u.click(screen.getByRole('button', { name: '确认删除' }))
    if (c.name === '程序') await u.click(await screen.findByRole('button', { name: '展开工序 10' }))
    expect(await screen.findByText(c.outcome)).toBeInTheDocument()
    expect(calls.filter((r) => r.method === 'DELETE')).toEqual([expect.objectContaining({ path: c.path })])
    expect(client.getQueryState(['drawings', '', 1])?.isInvalidated).toBe(true)
    expect(client.getQueryState(['version-content', 5])?.isInvalidated).toBe(true)
  })
  test(`${c.name}删除失败保留确认框，可重试`, async () => {
    setup(true)
    view(c.route)
    const u = userEvent.setup()
    await u.click(await screen.findByRole('button', { name: c.button }))
    await u.click(screen.getByRole('button', { name: '确认删除' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('暂时无法删除')
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    await u.click(screen.getByRole('button', { name: '确认删除' }))
    if (c.name === '程序') await u.click(await screen.findByRole('button', { name: '展开工序 10' }))
    expect(await screen.findByText(c.outcome)).toBeInTheDocument()
  })
}

test('删除请求中禁止重复提交、取消、Esc 和关闭', async () => {
  let finish!: () => void
  const remove = vi.fn(() => new Promise<void>((resolve) => { finish = resolve }))
  const onClose = vi.fn(), onDeleted = vi.fn()
  renderApp(<DeleteSheet title="删除程序" message="测试" remove={remove} onClose={onClose} onDeleted={onDeleted} />)
  const u = userEvent.setup()
  await u.click(screen.getByRole('button', { name: '确认删除' }))
  expect(screen.getByRole('button', { name: '确认删除' })).toBeDisabled()
  expect(screen.getByRole('button', { name: '取消' })).toBeDisabled()
  await u.click(screen.getByRole('button', { name: '关闭' }))
  await u.keyboard('{Escape}')
  expect(onClose).not.toHaveBeenCalled()
  expect(remove).toHaveBeenCalledOnce()
  await act(async () => finish())
  await waitFor(() => expect(onDeleted).toHaveBeenCalledOnce())
})

test('程序已被其他用户删除时仍返回父级', async () => {
  let deleted = false
  mockApi((r) => {
    if (r.method === 'DELETE') {
      deleted = true
      return { status: 404, body: { error: '不存在' } }
    }
    if (r.path === '/programs/1/versions') return { body: [version()] }
    if (r.path === '/versions/5/tools') return { body: { items: [], revision: 'empty' } }
    return { body: detail([operation({ programs: deleted ? [] : [program()] })]) }
  })
  view('/drawings/1/programs/1')
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '删除程序' }))
  await u.click(screen.getByRole('button', { name: '确认删除' }))
  await u.click(await screen.findByRole('button', { name: '展开工序 10' }))
  expect(await screen.findByText('暂无程序')).toBeInTheDocument()
})
