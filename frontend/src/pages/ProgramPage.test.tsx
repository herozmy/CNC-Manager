import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Route, Routes } from 'react-router-dom'
import { afterEach, expect, test, vi } from 'vitest'
import { detail, mockApi, operation, program, renderApp, version } from '../test-utils'
import ProgramPage from './ProgramPage'

afterEach(() => vi.unstubAllGlobals())

const view = () => (
  <Routes>
    <Route path="/drawings/:id/programs/:programId" element={<ProgramPage />} />
  </Routes>
)
const URL = '/drawings/1/programs/1'

function server(versions: unknown[], extra?: (r: { method: string; path: string; body: unknown }) => { status?: number; body?: unknown } | undefined) {
  return mockApi((r) => {
    const custom = extra?.(r)
    if (custom) return custom
    if (r.path === '/drawings/1/detail') return { body: detail([operation({ programs: [program()] })]) }
    if (r.path === '/programs/1/versions' && r.method === 'GET') return { body: versions }
  })
}

test('显示程序信息、当前版本下载入口与版本历史', async () => {
  server([version({ id: 5, versionNo: 3, changeNote: '调整进给' }), version({ id: 4, versionNo: 2, isCurrent: false, encoding: 'gbk', fileName: 'old.nc', createdBy: '' })])
  renderApp(view(), URL)
  expect(await screen.findByRole('heading', { name: 'O1234' })).toBeInTheDocument()
  expect(screen.getByText('D-1001 · 10# 粗车')).toBeInTheDocument()
  expect(screen.getByRole('link', { name: '下载当前版本' })).toHaveAttribute('href', '/api/versions/5/download')
  expect(screen.getByText('调整进给')).toBeInTheDocument()
  expect(screen.getByText('2.0 KB · UTF-8 · alice')).toBeInTheDocument()
  expect(screen.getByText('2.0 KB · GBK')).toBeInTheDocument()
  expect(screen.getByRole('link', { name: '下载 V2' })).toHaveAttribute('href', '/api/versions/4/download')
  // 当前版本没有"设为当前"，历史版本有
  expect(screen.queryByRole('button', { name: '将 V3 设为当前' })).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: '将 V2 设为当前' })).toBeInTheDocument()
})

test('没有版本时的空状态，且没有下载入口', async () => {
  server([])
  renderApp(view(), URL)
  expect(await screen.findByText('还没有版本')).toBeInTheDocument()
  expect(screen.getByText('还没有 NC 文件')).toBeInTheDocument()
  expect(screen.queryByRole('link', { name: '下载当前版本' })).not.toBeInTheDocument()
})

test('上传新版本：以 multipart 提交文件与说明，并展示识别出的刀具', async () => {
  const calls = server([version()], (r) => {
    if (r.method === 'POST' && r.path === '/programs/1/versions') {
      return { status: 201, body: { ...version({ id: 6, versionNo: 4 }), parse: { controller: 'FANUC', lineCount: 5, warnings: ['按加工中心规则跳过了 1 处备刀调用（T2）'], tools: [{ seq: 1, toolNo: 'T01', offsetNo: 'D01', lineNo: 3, raw: 'T0101' }, { seq: 2, toolNo: 'T05', offsetNo: '', lineNo: 9, raw: 'T05 M06' }] } } }
    }
  })
  renderApp(view(), URL)
  const u = userEvent.setup()
  await screen.findByRole('heading', { name: 'O1234' })
  const file = new File(['O1234\nT0101\nM30\n'], 'O1234_v4.nc', { type: 'text/plain' })
  await u.upload(screen.getByLabelText('选择 NC 文件'), file)

  const dialog = await screen.findByRole('dialog', { name: '上传新版本' })
  expect(within(dialog).getByText('O1234_v4.nc')).toBeInTheDocument()
  await u.type(within(dialog).getByLabelText('变更说明（可选）'), '改精车')
  await u.click(within(dialog).getByRole('button', { name: '上传' }))

  expect(await screen.findByText('已上传 V4')).toBeInTheDocument()
  expect(screen.getByText('识别到 2 把刀：T01/D01、T05')).toBeInTheDocument()
  expect(screen.getByText(/跳过了 1 处备刀调用/)).toBeInTheDocument()
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()

  const post = calls.find((c) => c.method === 'POST')!
  const form = post.body as FormData
  expect((form.get('file') as File).name).toBe('O1234_v4.nc')
  expect(form.get('changeNote')).toBe('改精车')
})

test('上传失败（文件过大）显示服务端提示，弹层保持打开可重试', async () => {
  server([], (r) => (r.method === 'POST' ? { status: 413, body: { error: '文件过大: 文件超过 64 MB 上限' } } : undefined))
  renderApp(view(), URL)
  const u = userEvent.setup()
  await screen.findByRole('heading', { name: 'O1234' })
  await u.upload(screen.getByLabelText('选择 NC 文件'), new File(['x'], 'big.nc'))
  await u.click(await screen.findByRole('button', { name: '上传' }))
  expect(await screen.findByText(/文件超过 64 MB 上限/)).toBeInTheDocument()
  expect(screen.getByRole('dialog', { name: '上传新版本' })).toBeInTheDocument()
})

test('回滚：设为当前需确认，确认后提交并刷新', async () => {
  let current = 5
  const calls = mockApi((r) => {
    if (r.path === '/drawings/1/detail') return { body: detail([operation({ programs: [program()] })]) }
    if (r.path === '/programs/1/versions') return { body: [version({ id: 5, versionNo: 3, isCurrent: current === 5 }), version({ id: 4, versionNo: 2, isCurrent: current === 4 })] }
    if (r.method === 'PUT' && r.path === '/programs/1/current-version') {
      current = (r.body as { versionId: number }).versionId
      return { body: program({ currentVersionId: current, currentVersionNo: 2 }) }
    }
  })
  renderApp(view(), URL)
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '将 V2 设为当前' }))
  expect(screen.getByText(/确定将 V2 设为当前版本吗/)).toBeInTheDocument()
  expect(calls.some((c) => c.method === 'PUT')).toBe(false)

  await u.click(screen.getByRole('button', { name: '确认切换' }))
  await waitFor(() => expect(screen.getByRole('link', { name: '下载当前版本' })).toHaveAttribute('href', '/api/versions/4/download'))
  expect(calls.find((c) => c.method === 'PUT')?.body).toEqual({ versionId: 4 })
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
})

test('程序不存在（不在该图纸下）或 ID 非法', async () => {
  mockApi((r) => (r.path === '/drawings/1/detail' ? { body: detail([operation({ programs: [] })]) } : r.path === '/programs/1/versions' ? { body: [] } : undefined))
  const first = renderApp(view(), URL)
  expect(await screen.findByText('程序不存在或已被删除')).toBeInTheDocument()
  first.unmount()

  const calls = mockApi(() => undefined)
  renderApp(view(), '/drawings/1/programs/xyz')
  expect(await screen.findByText('程序不存在或已被删除')).toBeInTheDocument()
  expect(calls).toHaveLength(0)
})

test('加载失败可重试', async () => {
  mockApi(() => ({ status: 500, body: { error: 'x' } }))
  renderApp(view(), URL)
  expect(await screen.findByText('程序加载失败')).toBeInTheDocument()
})
