import { act, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, test, vi } from 'vitest'
import App from '../App'
import { mockApi, renderApp, user } from '../test-utils'
import type { BackupPreview } from '../api/types'

afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals() })

const preview = { format: 'cnc-manager-v2/1', version: '2.1.0-dev', createdAt: '2026-11-14T08:00:00Z', sha256: 'checked-hash', summary: { users: 1, drawings: 2, operations: 3, programs: 4, versions: 5, tools: 6, files: 5, admins: ['sourceadmin'] } }

function server(role = 'admin', failCheck = false, failImport = false, result: BackupPreview = preview) {
  return mockApi((r) => {
    if (r.path === '/auth/status') return { body: { setupRequired: false } }
    if (r.path === '/auth/me') return { body: { ...user, role } }
    if (r.path === '/meta') return { body: { version: '2.1.0-dev' } }
    if (r.path === '/backup/recoveries') return { body: [] }
    if (r.path === '/backup/inspect') return failCheck ? { status: 400, body: { error: '备份文件损坏' } } : { body: result }
    if (r.path === '/backup/import') return failImport ? { status: 500, body: { error: '原数据已保留' } } : { body: { recoveryName: 'pre-import.zip' } }
    if (r.path === '/backup/create') return { status: 500, body: { error: '服务器空间不足' } }
  })
}

async function selectAndCheck() {
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '导入备份' }))
  await u.upload(screen.getByLabelText('备份文件'), new File(['zip data'], 'backup.zip', { type: 'application/zip' }))
  await u.click(screen.getByRole('button', { name: '检查备份' }))
  return u
}

test('高级管理只显示备份与迁移独立卡片，点击后才加载操作和恢复列表，返回后收起', async () => {
  const calls = server()
  renderApp(<App />, '/me')
  const u = userEvent.setup()
  const card = await screen.findByRole('link', { name: '备份与迁移' })
  expect(card).toHaveAttribute('href', '/me/backup')
  expect(screen.queryByRole('button', { name: '创建备份' })).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '导入备份' })).not.toBeInTheDocument()
  expect(screen.queryByRole('region', { name: '备份与迁移' })).not.toBeInTheDocument()
  expect(calls.filter(r => r.path.startsWith('/backup'))).toHaveLength(0)
  await u.click(card)
  expect(await screen.findByText('暂无可恢复备份，创建成功后会自动显示在这里。')).toBeVisible()
  expect(screen.getByRole('button', { name: '创建备份' })).toBeEnabled()
  expect(screen.getByRole('button', { name: '导入备份' })).toBeEnabled()
  expect(screen.getAllByRole('link', { name: '高级管理' }).every(link => link.getAttribute('aria-current') === 'page')).toBe(true)
  await u.click(screen.getByRole('link', { name: '返回高级管理' }))
  expect(await screen.findByRole('link', { name: '备份与迁移' })).toBeVisible()
  expect(screen.queryByRole('button', { name: '创建备份' })).not.toBeInTheDocument()
  expect(screen.queryByText('暂无可恢复备份，创建成功后会自动显示在这里。')).not.toBeInTheDocument()
})

test('备份页面可直接打开并归属高级管理导航，可返回高级管理', async () => {
  server()
  renderApp(<App />, '/me/backup')
  const u = userEvent.setup()
  expect(await screen.findByRole('heading', { name: '备份与迁移', level: 1 })).toBeVisible()
  expect(screen.getByRole('button', { name: '导入备份' })).toBeEnabled()
  expect(screen.getAllByRole('link', { name: '高级管理' }).every(link => link.getAttribute('aria-current') === 'page')).toBe(true)
  await u.click(screen.getByRole('link', { name: '返回高级管理' }))
  expect(await screen.findByRole('heading', { name: '高级管理' })).toBeVisible()
})

test('进入备份页面列表加载失败可重试，失败时仍能返回高级管理', async () => {
  server()
  const original = fetch
  let requests = 0
  vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
    if (String(input).endsWith('/backup/recoveries') && ++requests === 1) return Promise.resolve(new Response(JSON.stringify({ error: '暂时不可用' }), { status: 500 }))
    return original(input, init)
  }))
  renderApp(<App />, '/me')
  const u = userEvent.setup()
  await u.click(await screen.findByRole('link', { name: '备份与迁移' }))
  expect(await screen.findByText('恢复副本列表暂时不可用。')).toBeVisible()
  expect(screen.getByRole('link', { name: '返回高级管理' })).toHaveAttribute('href', '/me')
  await u.click(screen.getByRole('button', { name: '重试' }))
  expect(await screen.findByText('暂无可恢复备份，创建成功后会自动显示在这里。')).toBeVisible()
  expect(requests).toBe(2)
})

test.each([
  { role: 'editor', enabled: true },
  { role: 'viewer', enabled: true },
  { role: 'admin', enabled: false },
])('$role启用状态$enabled不提供备份入口，直接进入也不请求备份接口', async current => {
  const calls = mockApi(r => {
    if (r.path === '/auth/status') return { body: { setupRequired: false } }
    if (r.path === '/auth/me') return { body: { ...user, ...current } }
    if (r.path === '/meta') return { body: { version: 'local-test' } }
  })
  const view = renderApp(<App />, '/me')
  await screen.findByText('整站备份与导入请联系管理员。')
  expect(screen.queryByRole('link', { name: '备份与迁移' })).not.toBeInTheDocument()
  view.unmount()
  renderApp(<App />, '/me/backup')
  expect(await screen.findByText('仅系统管理员可访问备份与迁移。')).toBeVisible()
  expect(screen.getByRole('link', { name: '返回高级管理' })).toHaveAttribute('href', '/me')
  expect(screen.queryByRole('button', { name: '创建备份' })).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '导入备份' })).not.toBeInTheDocument()
  expect(calls.filter(r => r.path.startsWith('/backup'))).toHaveLength(0)
})

test('v1 备份显示转换结果和账号归属说明，确认后导入并使用原账号重新登录', async () => {
  const calls = server('admin', false, false, { ...preview, format: 'cnc-manager-v1/1', version: '1.2.13',
    migration: { skippedSessions: 2, orphanFiles: 1, skippedSettings: 1 } })
  renderApp(<App />, '/me/backup')
  const u = await selectAndCheck()
  expect(await screen.findByText('v1 迁移说明')).toBeVisible()
  expect(screen.getByText(/所有 v1 账号将成为管理员/)).toBeVisible()
  expect(screen.getByText(/图纸、机台和刀具进入“待分配”/)).toBeVisible()
  expect(screen.getByText(/旧登录会话不导入（2 个）/)).toBeVisible()
  expect(screen.getByText(/NC 文件不导入（1 个）/)).toBeVisible()
  expect(screen.getByText(/v1 扩展配置不导入（1 项）/)).toBeVisible()
  expect(screen.getByText(/检查尚未覆盖当前数据/)).toBeVisible()
  expect(screen.getByRole('button', { name: '确认覆盖导入' })).toBeDisabled()
  expect(calls.filter((r) => r.path === '/backup/import')).toHaveLength(0)
  await u.click(screen.getByRole('checkbox'))
  await u.click(screen.getByRole('button', { name: '确认覆盖导入' }))
  expect(await screen.findByRole('status')).toHaveTextContent('v1 数据迁移成功，请使用备份中的账号和密码登录')
})

test('导入先检查摘要并确认覆盖，成功清缓存并使用备份账号重新登录', async () => {
  const calls = server()
  const { client } = renderApp(<App />, '/me/backup')
  const u = await selectAndCheck()
  expect(await screen.findByText('备份检查通过')).toBeVisible()
  expect(screen.getByText('可登录管理员：sourceadmin')).toBeVisible()
  expect(screen.getByRole('button', { name: '确认覆盖导入' })).toBeDisabled()
  expect(calls.filter((r) => r.path === '/backup/import')).toHaveLength(0)
  client.setQueryData(['drawing', 7], { id: 7 })
  await u.click(screen.getByRole('checkbox'))
  await u.click(screen.getByRole('button', { name: '确认覆盖导入' }))
  expect(await screen.findByRole('status')).toHaveTextContent('备份导入成功，请使用备份中的账号和密码登录')
  expect(client.getQueryData(['drawing', 7])).toBeUndefined()
  expect(screen.getByLabelText('用户名')).toHaveValue('')
  const form = calls.find((r) => r.path === '/backup/import')?.body as FormData
  expect(form.get('sha256')).toBe('checked-hash')
  expect(form.get('confirmation')).toBe('覆盖导入')
  expect((form.get('file') as File).name).toBe('backup.zip')
})

test('损坏文件提示错误，可选择新文件重新检查', async () => {
  const calls = server('admin', true)
  renderApp(<App />, '/me/backup')
  const u = await selectAndCheck()
  expect(await screen.findByRole('alert')).toHaveTextContent('备份文件损坏')
  expect(screen.queryByRole('button', { name: '确认覆盖导入' })).not.toBeInTheDocument()
  await u.upload(screen.getByLabelText('备份文件'), new File(['new'], 'new.zip', { type: 'application/zip' }))
  expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  await u.click(screen.getByRole('button', { name: '检查备份' }))
  await screen.findByRole('alert')
  expect(calls.filter((r) => r.path === '/backup/inspect')).toHaveLength(2)
})

test('更换文件清除摘要和确认，取消不导入', async () => {
  const calls = server()
  renderApp(<App />, '/me/backup')
  const u = await selectAndCheck()
  await screen.findByText('备份检查通过')
  await u.click(screen.getByRole('checkbox'))
  await u.upload(screen.getByLabelText('备份文件'), new File(['new'], 'new.zip', { type: 'application/zip' }))
  expect(screen.queryByText('备份检查通过')).not.toBeInTheDocument()
  expect(screen.queryByRole('checkbox')).not.toBeInTheDocument()
  await u.click(screen.getByRole('button', { name: '取消' }))
  expect(calls.filter((r) => r.path === '/backup/import')).toHaveLength(0)
})

test('导入期间阻止重复提交和关闭，失败保留文件可重试', async () => {
  server('admin', false, true)
  renderApp(<App />, '/me/backup')
  const u = await selectAndCheck()
  await screen.findByText('备份检查通过')
  await u.click(screen.getByRole('checkbox'))
  let finish!: (r: Response) => void
  const original = fetch
  const pending = vi.fn(() => new Promise<Response>((resolve) => { finish = resolve }))
  vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => String(input).endsWith('/backup/import') ? pending() : original(input, init)))
  await u.click(screen.getByRole('button', { name: '确认覆盖导入' }))
  expect(screen.getByRole('button', { name: '确认覆盖导入' })).toBeDisabled()
  expect(screen.getByRole('button', { name: '取消' })).toBeDisabled()
  expect(screen.getByLabelText('备份文件')).toBeDisabled()
  await u.click(screen.getByRole('button', { name: '关闭' }))
  await u.keyboard('{Escape}')
  expect(screen.getByRole('dialog')).toBeVisible()
  expect(pending).toHaveBeenCalledOnce()
  await act(async () => finish(new Response(JSON.stringify({ error: '原数据已保留' }), { status: 500 })))
  await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('原数据已保留'))
  expect(screen.getByRole('button', { name: '确认覆盖导入' })).toBeEnabled()
  expect(screen.getByLabelText('备份文件')).toHaveProperty('files', expect.objectContaining({ length: 1 }))
})

test('有未保存刀具表时禁止导入，非管理员隐藏操作入口', async () => {
  server()
  const view = renderApp(<App />, '/me/backup')
  await screen.findByRole('button', { name: '导入备份' })
  view.client.setQueryDefaults(['version-tools-draft'], { gcTime: Infinity })
  view.client.setQueryData(['version-tools-draft', 7], { dirty: true })
  const u = await selectAndCheck()
  await screen.findByText('备份检查通过')
  await u.click(screen.getByRole('checkbox'))
  expect(screen.getByRole('button', { name: '确认覆盖导入' })).toBeDisabled()
  expect(screen.getByRole('alert')).toHaveTextContent('请先保存或撤销')
  view.unmount()
  const calls = server('viewer')
  renderApp(<App />, '/me')
  expect(await screen.findByText('整站备份与导入请联系管理员。')).toBeVisible()
  expect(screen.queryByRole('button', { name: '创建备份' })).not.toBeInTheDocument()
  expect(calls.filter((r) => r.path.startsWith('/backup'))).toHaveLength(0)
})

test('下载失败显示可重试错误，不生成假的下载成功提示', async () => {
  server()
  renderApp(<App />, '/me/backup')
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '创建备份' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('服务器空间不足')
  expect(screen.queryByText(/备份已准备好/)).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: '创建备份' })).toBeEnabled()
})

test('创建只保存到服务器，不触发下载', async () => {
  server()
  const original = fetch
  vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
    if (String(input).endsWith('/backup/create')) return Promise.resolve(new Response(JSON.stringify({ name: 'manual-20261114-200000-123.zip' }), { status: 201 }))
    return original(input, init)
  }))
  const createURL = vi.fn(() => 'blob:test')
  vi.stubGlobal('URL', class extends URL { static createObjectURL = createURL; static revokeObjectURL = vi.fn() })
  const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
  renderApp(<App />, '/me/backup')
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '创建备份' }))
  expect(await screen.findByRole('status')).toHaveTextContent('备份已保存到服务器')
  expect(createURL).not.toHaveBeenCalled()
  expect(click).not.toHaveBeenCalled()
})

test('成功创建后刷新列表，只有点击下载才触发下载且不重新创建备份', async () => {
  server()
  const original = fetch
  let exported = false
  let exports = 0
  vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
    const path = String(input)
    if (path.endsWith('/backup/create')) { exported = true; exports++; return Promise.resolve(new Response(JSON.stringify({ name: 'manual-20261114-200000-123.zip' }), { status: 201 })) }
    if (path.endsWith('/backup/recoveries')) return Promise.resolve(new Response(JSON.stringify(exported ? [{ name: 'manual-20261114-200000-123.zip', createdAt: '2026-11-14T12:00:00Z', sizeBytes: 1024 }] : [])))
    if (path.endsWith('/backup/recoveries/manual-20261114-200000-123.zip')) return Promise.resolve(new Response('ZIP'))
    return original(input, init)
  }))
  vi.stubGlobal('URL', class extends URL { static createObjectURL = vi.fn(() => 'blob:test'); static revokeObjectURL = vi.fn() })
  const downloadClick = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
  renderApp(<App />, '/me/backup')
  const u = userEvent.setup()
  await screen.findByText('暂无可恢复备份，创建成功后会自动显示在这里。')
  await u.click(screen.getByRole('button', { name: '创建备份' }))
  await u.click(await screen.findByText('可恢复备份（1）'))
  expect(downloadClick).not.toHaveBeenCalled()
  await u.click(screen.getByRole('button', { name: /下载 .* 的恢复副本/ }))
  expect(downloadClick).toHaveBeenCalledOnce()
  expect(exports).toBe(1)
})

test('删除备份须确认，取消不删除，失败可重试，成功刷新列表', async () => {
  server()
  const original = fetch
  let exists = true
  let deletes = 0
  vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
    const path = String(input)
    if (path.endsWith('/backup/recoveries')) return Promise.resolve(new Response(JSON.stringify(exists ? [{ name: 'manual-20261114-200000-123.zip', createdAt: '2026-11-14T12:00:00Z', sizeBytes: 1024 }] : [])))
    if (init?.method === 'DELETE') {
      deletes++
      if (deletes === 1) return Promise.resolve(new Response(JSON.stringify({ error: '删除失败' }), { status: 500 }))
      exists = false
      return Promise.resolve(new Response(null, { status: 204 }))
    }
    return original(input, init)
  }))
  renderApp(<App />, '/me/backup')
  const u = userEvent.setup()
  await u.click(await screen.findByText('可恢复备份（1）'))
  await u.click(screen.getByRole('button', { name: /删除 .* 的备份/ }))
  expect(screen.getByText(/删除后无法从服务器恢复此备份/)).toBeVisible()
  await u.click(screen.getByRole('button', { name: '取消' }))
  expect(deletes).toBe(0)
  await u.click(screen.getByRole('button', { name: /删除 .* 的备份/ }))
  await u.click(screen.getByRole('button', { name: '确认删除' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('删除失败')
  await u.click(screen.getByRole('button', { name: '确认删除' }))
  expect(await screen.findByText('暂无可恢复备份，创建成功后会自动显示在这里。')).toBeVisible()
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  expect(deletes).toBe(2)
})
