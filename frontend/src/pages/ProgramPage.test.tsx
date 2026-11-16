import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Route, Routes } from 'react-router-dom'
import { afterEach, expect, test, vi } from 'vitest'
import { detail, mockApi, operation, program, renderApp, version } from '../test-utils'
import ProgramPage from './ProgramPage'

vi.mock('../components/NCCodeEditor', () => import('../test/MockNCCodeEditor'))

afterEach(() => vi.unstubAllGlobals())

const view = () => (
  <Routes>
    <Route path="/drawings/:id/programs/:programId" element={<ProgramPage />} />
  </Routes>
)
const URL = '/drawings/1/programs/1'

function pageApi(handler: Parameters<typeof mockApi>[0]) {
  return mockApi((r) => handler(r) ?? (r.method === 'GET' && /^\/versions\/\d+\/tools$/.test(r.path) ? { body: { items: [], revision: 'empty' } } : undefined))
}

function server(versions: unknown[], extra?: (r: { method: string; path: string; body: unknown }) => { status?: number; body?: unknown } | undefined) {
  return pageApi((r) => {
    const custom = extra?.(r)
    if (custom) return custom
    if (r.path === '/drawings/1/detail') return { body: detail([operation({ programs: [program()] })]) }
    if (r.path === '/programs/1/versions' && r.method === 'GET') return { body: versions }
  })
}

test('默认收起版本详情，下拉选择只显示所选版本，浏览历史不会切换当前版本', async () => {
  const calls = server([version({ id: 5, versionNo: 3, changeNote: '调整进给' }), version({ id: 4, versionNo: 2, isCurrent: false, encoding: 'gbk', fileName: 'old.nc', createdBy: '' })])
  renderApp(view(), URL)
  const u = userEvent.setup()
  expect(await screen.findByRole('heading', { name: 'O1234' })).toBeInTheDocument()
  expect(screen.getByText('D-1001 · 10# 粗车')).toBeInTheDocument()
  expect(screen.getByRole('link', { name: '下载当前版本' })).toHaveAttribute('href', '/api/versions/5/download')
  const select = screen.getByRole('combobox', { name: '选择 NC 程序版本' })
  expect(select).toHaveValue('')
  expect(within(select).getByRole('option', { name: 'V3（当前） · O1234.nc' })).toBeInTheDocument()
  expect(screen.queryByRole('region', { name: /版本详情/ })).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: /查看 V/ })).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: '查看 / 编辑当前版本' })).toHaveTextContent('V3')

  await u.selectOptions(select, '5')
  expect(screen.getByText('调整进给')).toBeInTheDocument()
  expect(screen.getByText('2.0 KB · UTF-8 · alice')).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '将 V3 设为当前' })).not.toBeInTheDocument()
  expect(screen.queryByRole('link', { name: '下载 V2' })).not.toBeInTheDocument()

  await u.selectOptions(select, '4')
  expect(screen.getAllByRole('region', { name: /版本详情/ })).toHaveLength(1)
  expect(screen.queryByText('调整进给')).not.toBeInTheDocument()
  expect(screen.queryByRole('link', { name: '下载 V3' })).not.toBeInTheDocument()
  expect(screen.getByText('2.0 KB · GBK')).toBeInTheDocument()
  expect(screen.getByRole('link', { name: '下载 V2' })).toHaveAttribute('href', '/api/versions/4/download')
  expect(screen.getByRole('button', { name: '将 V2 设为当前' })).toBeInTheDocument()
  expect(screen.getByRole('link', { name: '下载当前版本' })).toHaveAttribute('href', '/api/versions/5/download')
  expect(calls.every((r) => r.method === 'GET' && !r.path.endsWith('/content'))).toBe(true)

  await u.selectOptions(select, '')
  expect(screen.queryByRole('region', { name: /版本详情/ })).not.toBeInTheDocument()
})

test('没有版本时的空状态，且没有下载入口', async () => {
  server([])
  renderApp(view(), URL)
  expect(await screen.findByText('还没有版本')).toBeInTheDocument()
  expect(screen.getByText('还没有 NC 文件')).toBeInTheDocument()
  expect(screen.queryByRole('link', { name: '下载当前版本' })).not.toBeInTheDocument()
  expect(screen.queryByRole('combobox')).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '查看 / 编辑当前版本' })).not.toBeInTheDocument()
})

test('上传新版本：以 multipart 提交文件与说明，并展示识别出的刀具', async () => {
  let uploaded = false
  const calls = server([version()], (r) => {
    if (uploaded && r.method === 'GET' && r.path === '/programs/1/versions') {
      return { body: [version({ id: 6, versionNo: 4 }), version({ isCurrent: false })] }
    }
    if (r.method === 'POST' && r.path === '/programs/1/versions') {
      uploaded = true
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
  expect(screen.getByRole('combobox')).toHaveValue('6')
  expect(screen.getByRole('region', { name: 'V4 版本详情' })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '查看 / 编辑当前版本' })).toHaveTextContent('V4')

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
  const calls = pageApi((r) => {
    if (r.path === '/drawings/1/detail') return { body: detail([operation({ programs: [program()] })]) }
    if (r.path === '/versions/4/content') return { body: { versionId: 4, programId: 1, versionNo: 2, fileName: 'old.nc', encoding: 'utf-8', content: 'O1234\n(V2)\nM30', lineCount: 3, sizeBytes: 15, isCurrent: true } }
    if (r.path === '/programs/1/versions') return { body: [version({ id: 5, versionNo: 3, isCurrent: current === 5 }), version({ id: 4, versionNo: 2, isCurrent: current === 4 })] }
    if (r.method === 'PUT' && r.path === '/programs/1/current-version') {
      current = (r.body as { versionId: number }).versionId
      return { body: program({ currentVersionId: current, currentVersionNo: 2 }) }
    }
  })
  renderApp(view(), URL)
  const u = userEvent.setup()
  await u.selectOptions(await screen.findByRole('combobox'), '4')
  await u.click(screen.getByRole('button', { name: '将 V2 设为当前' }))
  expect(screen.getByText(/确定将 V2 设为当前版本吗/)).toBeInTheDocument()
  expect(calls.some((c) => c.method === 'PUT')).toBe(false)

  await u.click(screen.getByRole('button', { name: '确认切换' }))
  await waitFor(() => expect(screen.getByRole('link', { name: '下载当前版本' })).toHaveAttribute('href', '/api/versions/4/download'))
  expect(calls.find((c) => c.method === 'PUT')?.body).toEqual({ versionId: 4 })
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  expect(screen.getByRole('combobox')).toHaveValue('4')
  expect(screen.getByRole('option', { name: 'V2（当前） · O1234.nc' })).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '将 V2 设为当前' })).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: '查看 / 编辑当前版本' })).toHaveTextContent('V2')
  await u.click(screen.getByRole('button', { name: '查看 / 编辑当前版本' }))
  expect(await screen.findByRole('dialog', { name: 'NC 内容 · V2' })).toBeInTheDocument()
  expect(await screen.findByLabelText('NC 程序内容')).toHaveValue('O1234\n(V2)\nM30')
})

test('取消设为当前不写入；失败保留所选版本并支持重试', async () => {
  let fail = true
  let current = 5
  const calls = server([], (r) => {
    if (r.method === 'GET' && r.path === '/programs/1/versions') return { body: [version({ isCurrent: current === 5 }), version({ id: 4, versionNo: 2, isCurrent: current === 4 })] }
    if (r.method === 'PUT') {
      if (fail) return { status: 500, body: { error: '切换失败' } }
      current = 4
      return { body: program({ currentVersionId: 4, currentVersionNo: 2 }) }
    }
  })
  renderApp(view(), URL)
  const u = userEvent.setup()
  await u.selectOptions(await screen.findByRole('combobox'), '4')
  await u.click(screen.getByRole('button', { name: '将 V2 设为当前' }))
  await u.click(within(screen.getByRole('dialog')).getByRole('button', { name: '取消' }))
  expect(calls.some((c) => c.method === 'PUT')).toBe(false)
  expect(screen.getByRole('combobox')).toHaveValue('4')

  await u.click(screen.getByRole('button', { name: '将 V2 设为当前' }))
  await u.click(screen.getByRole('button', { name: '确认切换' }))
  expect(await screen.findByText(/切换失败，请重试/)).toBeInTheDocument()
  expect(screen.getByRole('combobox')).toHaveValue('4')
  expect(screen.getByRole('link', { name: '下载当前版本' })).toHaveAttribute('href', '/api/versions/5/download')
  fail = false
  await u.click(screen.getByRole('button', { name: '确认切换' }))
  await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  expect(screen.getByRole('link', { name: '下载当前版本' })).toHaveAttribute('href', '/api/versions/4/download')
})

test('程序不存在（不在该图纸下）或 ID 非法', async () => {
  pageApi((r) => (r.path === '/drawings/1/detail' ? { body: detail([operation({ programs: [] })]) } : r.path === '/programs/1/versions' ? { body: [] } : undefined))
  const first = renderApp(view(), URL)
  expect(await screen.findByText('程序不存在或已被删除')).toBeInTheDocument()
  first.unmount()

  const calls = pageApi(() => undefined)
  renderApp(view(), '/drawings/1/programs/xyz')
  expect(await screen.findByText('程序不存在或已被删除')).toBeInTheDocument()
  expect(calls).toHaveLength(0)
})

test('加载失败可重试', async () => {
  pageApi(() => ({ status: 500, body: { error: 'x' } }))
  renderApp(view(), URL)
  expect(await screen.findByText('程序加载失败')).toBeInTheDocument()
})

test('从历史版本打开 NC 查看入口，读取选中的版本', async () => {
  server([version({ id: 4, versionNo: 2, isCurrent: false })], (r) => r.path === '/versions/4/content' ? { body: { versionId: 4, programId: 1, versionNo: 2, fileName: 'old.nc', encoding: 'utf-8', content: 'O1234\nM30', lineCount: 2, sizeBytes: 10, isCurrent: false } } : undefined)
  renderApp(view(), URL)
  const u = userEvent.setup()
  await u.selectOptions(await screen.findByRole('combobox'), '4')
  await u.click(screen.getByRole('button', { name: '查看 V2 内容' }))
  expect(await screen.findByRole('dialog', { name: 'NC 内容 · V2' })).toBeInTheDocument()
  expect(screen.getByLabelText('NC 程序内容')).toHaveValue('O1234\nM30')
  expect(screen.getByLabelText('NC 程序内容')).toHaveAttribute('readonly')
})

test('历史详情收起或选中其他版本时，当前版本仍可直接查看和编辑', async () => {
  const calls = server([version(), version({ id: 4, versionNo: 2, isCurrent: false })], (r) => r.path === '/versions/5/content'
    ? { body: { versionId: 5, programId: 1, versionNo: 3, fileName: 'current.nc', encoding: 'utf-8', content: 'O1234\n(V3)\nM30', lineCount: 3, sizeBytes: 15, isCurrent: true } }
    : undefined)
  renderApp(view(), URL)
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '查看 / 编辑当前版本' }))
  expect(await screen.findByRole('dialog', { name: 'NC 内容 · V3' })).toBeInTheDocument()
  await u.click(screen.getByRole('button', { name: '关闭' }))
  await u.selectOptions(screen.getByRole('combobox'), '4')
  await u.click(screen.getByRole('button', { name: '查看 / 编辑当前版本' }))
  expect(await screen.findByLabelText('NC 程序内容')).toHaveValue('O1234\n(V3)\nM30')
  await u.click(screen.getByRole('button', { name: '编辑内容' }))
  expect(screen.getByLabelText('NC 程序内容')).not.toHaveAttribute('readonly')
  expect(calls.some((r) => r.path === '/versions/4/content')).toBe(false)
})

test.each([false, true])('保存后显示对应版本的操作区（覆盖=%s），当前入口跟随实际当前版本', async (overwrite) => {
  let entries = [version(), version({ id: 4, versionNo: 2, isCurrent: false })]
  const savedVersion = version({ id: overwrite ? 4 : 6, versionNo: overwrite ? 2 : 4, isCurrent: !overwrite })
  const content = 'O1234\n(edited)\nM30'
  let saved = false
  server([], (r) => {
    if (r.path === '/programs/1/versions') return { body: entries }
    if (r.path === '/versions/4/content' && r.method === 'GET') return { body: { versionId: 4, programId: 1, versionNo: 2, fileName: 'old.nc', encoding: 'utf-8', content: saved ? content : 'O1234\nM30', lineCount: 2, sizeBytes: 10, isCurrent: false } }
    if ((r.path === '/versions/4/content' && r.method === 'PUT') || (r.path === '/programs/1/versions/content' && r.method === 'POST')) {
      saved = true
      entries = overwrite ? [version(), savedVersion] : [savedVersion, ...entries.map((v) => ({ ...v, isCurrent: false }))]
      return { body: savedVersion }
    }
  })
  renderApp(view(), URL)
  const u = userEvent.setup()
  await u.selectOptions(await screen.findByRole('combobox'), '4')
  await u.click(screen.getByRole('button', { name: '查看 V2 内容' }))
  await u.click(await screen.findByRole('button', { name: '编辑内容' }))
  await u.clear(screen.getByLabelText('NC 程序内容'))
  await u.type(screen.getByLabelText('NC 程序内容'), content)
  await u.click(screen.getByRole('button', { name: overwrite ? '覆盖此版本' : '另存新版本' }))
  if (overwrite) await u.click(screen.getByRole('button', { name: '确认覆盖' }))
  expect(await screen.findByText(`已保存 V${savedVersion.versionNo}`)).toBeInTheDocument()
  expect(screen.getByRole('combobox')).toHaveValue(String(savedVersion.id))
  expect(screen.getByRole('button', { name: `查看 V${savedVersion.versionNo} 内容` })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '查看 / 编辑当前版本' })).toHaveTextContent(overwrite ? 'V3' : 'V4')
  expect(screen.getByRole('link', { name: '下载当前版本' })).toHaveAttribute('href', `/api/versions/${overwrite ? 5 : 6}/download`)
})

test('刀具表直接显示，选择历史版本后切换表格并保留各版本草稿', async () => {
  const calls = server([version(), version({ id: 4, versionNo: 2, isCurrent: false })])
  renderApp(view(), URL)
  const u = userEvent.setup()
  await screen.findByRole('region', { name: 'V3 刀具信息' })
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  await u.click(await screen.findByRole('button', { name: '手动添加' }))
  await u.type(screen.getByLabelText('刀具号'), 'T03')
  await u.type(screen.getByLabelText('刀具名称'), '当前版草稿')
  await u.selectOptions(screen.getByRole('combobox', { name: '选择 NC 程序版本' }), '4')
  await screen.findByRole('region', { name: 'V2 刀具信息' })
  await u.click(await screen.findByRole('button', { name: '手动添加' }))
  await u.type(screen.getByLabelText('刀具号'), 'T02')
  await u.selectOptions(screen.getByRole('combobox', { name: '选择 NC 程序版本' }), '')
  expect(await screen.findByLabelText('刀具号')).toHaveValue('T03')
  expect(screen.getByLabelText('刀具名称')).toHaveValue('当前版草稿')
  await u.selectOptions(screen.getByRole('combobox', { name: '选择 NC 程序版本' }), '4')
  expect(await screen.findByLabelText('刀具号')).toHaveValue('T02')
  expect(calls.some((r) => r.method !== 'GET')).toBe(false)
})

test('工作台显式加载所选 NC 预览，浏览历史和复制失败都不修改版本', async () => {
  const calls = server([version(), version({ id: 4, versionNo: 2, isCurrent: false })], r => {
    if (r.path === '/versions/5/content' || r.path === '/versions/4/content') {
      const old = r.path.includes('/4/')
      return { body: { versionId: old ? 4 : 5, programId: 1, versionNo: old ? 2 : 3, fileName: 'O1234.nc', encoding: 'utf-8', content: old ? 'O1234\n(V2)\nM30' : 'O1234\n(V3)\nM30', lineCount: 3, sizeBytes: 20, isCurrent: !old } }
    }
  })
  renderApp(view(), URL)
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '加载 NC 预览' }))
  expect(await screen.findByLabelText('NC 程序预览')).toHaveValue('O1234\n(V3)\nM30')
  expect(screen.getByLabelText('NC 程序预览')).toHaveAttribute('readonly')
  vi.spyOn(navigator.clipboard, 'writeText').mockRejectedValueOnce(new Error('denied'))
  await u.click(screen.getByRole('button', { name: '复制程序' }))
  expect(await screen.findByText('复制失败，请选择代码后手动复制')).toBeInTheDocument()
  await u.click(screen.getByRole('button', { name: '复制程序' }))
  expect(await screen.findByText('已复制程序')).toBeInTheDocument()
  await u.click(screen.getByRole('button', { name: '浏览 V2' }))
  expect(screen.getByLabelText('选择 NC 程序版本')).toHaveValue('4')
  expect(screen.getByText('正在浏览历史版本 · V2 · UTF-8')).toBeInTheDocument()
  expect(screen.queryByLabelText('NC 程序预览')).not.toBeInTheDocument()
  await u.click(screen.getByRole('button', { name: '加载 NC 预览' }))
  expect(await screen.findByLabelText('NC 程序预览')).toHaveValue('O1234\n(V2)\nM30')
  expect(screen.getByRole('link', { name: '下载当前版本' })).toHaveAttribute('href', '/api/versions/5/download')
  expect(calls.some(r => r.method !== 'GET')).toBe(false)
  vi.restoreAllMocks()
})

test('刀具草稿在工作台标签间保留，差异视图使用真实版本接口', async () => {
  const calls = server([version(), version({ id: 4, versionNo: 2, isCurrent: false })], r => {
    if (r.path.startsWith('/versions/4/diff?')) return { body: { leftVersionNo: 2, rightVersionNo: 3, identical: false, truncated: false, totalLines: 1, totalChanges: 1, lines: [{ type: 'change', leftNo: 2, rightNo: 2, leftText: 'G01 F1650', rightText: 'G01 F950' }] } }
  })
  renderApp(view(), URL)
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '手动添加' }))
  await u.type(screen.getByLabelText('刀具号'), 'T08')
  await u.type(screen.getByLabelText('刀具名称'), '未保存的精镗刀')
  await u.click(screen.getByRole('tab', { name: '版本差异' }))
  const table = await screen.findByRole('table', { name: '版本差异' })
  expect(table).toHaveTextContent('G01 F1650')
  expect(table).toHaveTextContent('G01 F950')
  expect(calls.some(r => r.path.startsWith('/versions/4/diff?with=5'))).toBe(true)
  await u.click(screen.getByRole('tab', { name: '版本刀具表' }))
  expect(screen.getByLabelText('刀具号')).toHaveValue('T08')
  expect(screen.getByLabelText('刀具名称')).toHaveValue('未保存的精镗刀')
  await u.keyboard('{ArrowLeft}')
  expect(screen.getByRole('tab', { name: 'NC 代码' })).toHaveAttribute('aria-selected', 'true')
  expect(screen.getByLabelText('刀具号')).toHaveValue('T08')
  expect(calls.some(r => r.method !== 'GET')).toBe(false)
})

test('NC 预览加载失败可重试，并保留版本与刀具操作入口', async () => {
  let fail = true
  server([version()], r => r.path === '/versions/5/content' ? fail
    ? { status: 500, body: { error: '暂时不可用' } }
    : { body: { versionId: 5, programId: 1, versionNo: 3, fileName: 'O1234.nc', encoding: 'utf-8', content: 'O1234\nM30', lineCount: 2, sizeBytes: 10, isCurrent: true } }
    : undefined)
  renderApp(view(), URL)
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '加载 NC 预览' }))
  await screen.findByText('NC 预览加载失败')
  expect(screen.getByRole('tab', { name: '版本差异' })).toBeDisabled()
  expect(screen.getByRole('button', { name: '查看 / 编辑当前版本' })).toBeInTheDocument()
  fail = false
  await u.click(screen.getByRole('button', { name: '重试' }))
  expect(await screen.findByLabelText('NC 程序预览')).toHaveValue('O1234\nM30')
})
