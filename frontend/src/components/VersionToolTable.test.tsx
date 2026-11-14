import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, test, vi } from 'vitest'
import { mockApi, renderApp, version } from '../test-utils'
import type { ProgramTool, ToolTable, Version } from '../api/types'
import VersionToolTable from './VersionToolTable'

afterEach(() => vi.unstubAllGlobals())
const row: ProgramTool = { id: 7, programId: 1, versionId: 5, seq: 9, toolId: 42, toolNo: 'T01', offsetNo: 'H01', toolName: '旧刀具', toolDia: 12.345, cornerRadius: .8, compAmount: -.025, spindleSpeed: 250, speedMode: 1, feed: .125, feedMode: 1, cutDepth: 1.25, coolant: 3, machiningContent: '精加工', remark: '保留备注', customParams: [{ name: '品牌', value: '原品牌' }] }
const source: ToolTable = { revision: 'original-revision', items: [row] }
function mount() {
 const result = renderApp(<VersionToolTable version={version() as Version} />)
 return { ...result, u: userEvent.setup() }
}

test('三字段直接编辑，隐藏历史数据完整保留，当前版本端点固定', async () => {
 const calls = mockApi((r) => r.method === 'GET' ? { body: source } : { body: { ...source, revision: 'saved', items: [{ ...row, toolName: '精镗刀', compAmount: -.123 }] } })
 const { u } = mount()
 expect(await screen.findByLabelText('刀具名称')).not.toHaveAttribute('readonly')
 expect(screen.queryByLabelText('直径（mm）')).not.toBeInTheDocument()
 expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
 expect(screen.queryByLabelText('补偿量（mm）')).not.toBeInTheDocument()
 expect(screen.getAllByRole('textbox')).toEqual([screen.getByLabelText('刀具号'), screen.getByLabelText('刀具名称')])
 expect(screen.getAllByRole('spinbutton')).toEqual([screen.getByLabelText('刀补（mm）')])
 expect(screen.getByLabelText('刀补（mm）')).toHaveValue(-.025)
 expect(screen.queryByLabelText('刀补号')).not.toBeInTheDocument()
 expect(screen.queryByText(/加工参数|自定义/)).not.toBeInTheDocument()
 expect(screen.queryByLabelText('速度模式')).not.toBeInTheDocument()
 await u.clear(screen.getByLabelText('刀具名称'))
 await u.type(screen.getByLabelText('刀具名称'), '精镗刀')
 await u.clear(screen.getByLabelText('刀补（mm）'))
 await u.type(screen.getByLabelText('刀补（mm）'), '-0.123')
 await u.click(screen.getByRole('button', { name: '保存刀具表' }))
 expect(await screen.findByRole('status')).toHaveTextContent('已保存 V3')
 expect(calls.find((r) => r.method === 'PUT')).toMatchObject({ path: '/versions/5/tools', body: { expectedRevision: source.revision, items: [{ ...row, seq: 1, toolName: '精镗刀', compAmount: -.123 }] } })
})

test('识别仅追加缺少刀具，顺序可调整、删除，保存前不写入', async () => {
 const calls = mockApi((r) => r.path.endsWith('/recognize') ? { body: { tools: [{ toolNo: 'T01', offsetNo: 'H01' }, { toolNo: 'T02', offsetNo: 'H02' }, { toolNo: 'T02', offsetNo: 'H02' }], warnings: [] } } : { body: source })
 const { u } = mount()
 await screen.findByRole('button', { name: '手动添加' })
 await u.click(screen.getByRole('button', { name: '从 NC 识别' }))
 expect(await screen.findByRole('status')).toHaveTextContent('已补充 1 把刀具')
 expect(screen.getAllByLabelText('刀具号')).toHaveLength(2)
 expect(screen.getAllByLabelText('刀具名称')[0]).toHaveValue('旧刀具')
 await u.click(screen.getByRole('button', { name: '上移第 2 把刀具' }))
 expect(screen.getAllByLabelText('刀具号')[0]).toHaveValue('T02')
 await u.click(screen.getByRole('button', { name: '删除第 1 把刀具' }))
 expect(screen.getByLabelText('刀具号')).toHaveValue('T01')
 expect(calls.some((r) => r.method === 'PUT')).toBe(false)
})

test('删除全部刀具会显式提交空数组', async () => {
 const calls = mockApi(() => ({ body: source }))
 const { u } = mount()
 await screen.findByRole('button', { name: '手动添加' })
 await u.click(screen.getByRole('button', { name: '删除第 1 把刀具' }))
 await u.click(screen.getByRole('button', { name: '保存刀具表' }))
 await waitFor(() => expect(calls.filter((r) => r.method === 'PUT')).toHaveLength(1))
 expect(calls.find((r) => r.method === 'PUT')?.body).toMatchObject({ items: [] })
})

test('保存失败保留草稿，撤销需要确认，确认后清理草稿缓存', async () => {
 mockApi((r) => r.method === 'GET' ? { body: source } : { status: 500, body: { error: '写入失败' } })
 const { u, client } = mount()
 await screen.findByRole('button', { name: '手动添加' })
 await u.type(screen.getByLabelText('刀具名称'), '修改')
 await u.click(screen.getByRole('button', { name: '保存刀具表' }))
 expect(await screen.findByRole('alert')).toHaveTextContent('写入失败')
 expect(screen.getByLabelText('刀具名称')).toHaveValue('旧刀具修改')
 expect(client.getQueryData(['version-tools-draft', 5])).toMatchObject({ dirty: true })
 await u.click(screen.getByRole('button', { name: '撤销修改' }))
 await u.click(screen.getByRole('button', { name: '继续编辑' }))
 expect(screen.getByLabelText('刀具名称')).toHaveValue('旧刀具修改')
 await u.click(screen.getByRole('button', { name: '撤销修改' }))
 await u.click(screen.getByRole('button', { name: '放弃修改' }))
 expect(screen.getByLabelText('刀具名称')).toHaveValue('旧刀具')
 expect(client.getQueryData(['version-tools-draft', 5])).toBeUndefined()
})

test('冲突保护打开时的校验值，重载失败保留草稿，成功后使用新校验值', async () => {
 let latest = source, failReload = false, conflict = true
 const calls = mockApi((r) => r.method === 'GET' ? failReload ? { status: 500 } : { body: latest } : conflict ? { status: 409, body: { error: '刀具表已被修改' } } : { body: latest })
 const { u, client } = mount()
 await screen.findByRole('button', { name: '手动添加' })
 await u.type(screen.getByLabelText('刀具名称'), '草稿')
 client.setQueryData(['version-tools', 5], { ...source, revision: 'background' })
 await u.click(screen.getByRole('button', { name: '保存刀具表' }))
 expect(await screen.findByRole('alert')).toHaveTextContent('已被修改')
 expect(calls.find((r) => r.method === 'PUT')?.body).toMatchObject({ expectedRevision: source.revision })
 expect(screen.getByRole('button', { name: '保存刀具表' })).toBeDisabled()
 failReload = true
 await u.click(screen.getByRole('button', { name: '载入最新刀具表' }))
 await u.click(screen.getByRole('button', { name: '放弃并载入' }))
 expect(await screen.findByRole('alert')).toHaveTextContent('草稿已保留')
 expect(screen.getByLabelText('刀具名称')).toHaveValue('旧刀具草稿')
 failReload = false; conflict = false; latest = { revision: 'new-revision', items: [{ ...row, toolName: '别人保存的名称' }] }
 await u.click(screen.getByRole('button', { name: '载入最新刀具表' }))
 await u.click(screen.getByRole('button', { name: '放弃并载入' }))
 await waitFor(() => expect(screen.getByLabelText('刀具名称')).toHaveValue('别人保存的名称'))
 await u.type(screen.getByLabelText('刀具名称'), '续改')
 await u.click(screen.getByRole('button', { name: '保存刀具表' }))
 await screen.findByText(/已保存 V3/)
 expect(calls.filter((r) => r.method === 'PUT').at(-1)?.body).toMatchObject({ expectedRevision: 'new-revision' })
})

test('加载失败重试，新增空行仅草稿', async () => {
 let fail = true
 const calls = mockApi(() => fail ? { status: 500 } : { body: { revision: 'empty', items: [] } })
 const { u } = mount()
 await screen.findByText('刀具表加载失败')
 fail = false
 await u.click(screen.getByRole('button', { name: /重试/ }))
 await screen.findByRole('button', { name: '手动添加' })
 await u.click(screen.getByRole('button', { name: '手动添加' }))
 const section = screen.getByRole('region', { name: '第 1 把刀具' })
 expect(within(section).getByLabelText('刀具号')).toHaveValue('')
 expect(calls.some((r) => r.method === 'PUT')).toBe(false)
})
