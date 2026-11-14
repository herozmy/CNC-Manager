import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Route, Routes } from 'react-router-dom'
import { afterEach, expect, test, vi } from 'vitest'
import { detail, mockApi, operation, program, renderApp } from '../test-utils'
import DrawingPage from './DrawingPage'

afterEach(() => vi.unstubAllGlobals())

const view = () => (
  <Routes>
    <Route path="/drawings/:id" element={<DrawingPage />} />
    <Route path="/drawings/:id/programs/:programId" element={<p>程序页</p>} />
  </Routes>
)

test('图纸详情显示产品类型，未填写时显示未分类', async () => {
  mockApi(() => ({ body: { ...detail([]), name: '壳体类' } }))
  const { unmount } = renderApp(view(), '/drawings/1')
  expect(await screen.findByText('类型：壳体类')).toBeInTheDocument()
  unmount()
  mockApi(() => ({ body: { ...detail([]), name: '' } }))
  renderApp(view(), '/drawings/1')
  expect(await screen.findByText('类型：未分类')).toBeInTheDocument()
})

/** 有状态的假服务器：写操作后详情会变化，用来验证界面确实重新加载。 */
function server(initial: unknown[]) {
  let ops = initial
  const calls = mockApi((r) => {
    if (r.method === 'GET' && r.path === '/drawings/1/detail') return { body: detail(ops) }
    if (r.method === 'GET' && r.path.startsWith('/drawings?')) return { body: { items: [], total: 0, page: 1, size: 20 } }
    if (r.method === 'POST' && r.path === '/drawings/1/operations') {
      const b = r.body as { opNo: number; opName: string }
      ops = [...ops, operation({ id: 9, opNo: b.opNo, opName: b.opName, programs: [] })]
      return { status: 201, body: ops[ops.length - 1] }
    }
    if (r.method === 'PUT' && r.path.startsWith('/operations/')) return { body: {} }
    if (r.method === 'DELETE' && r.path.startsWith('/operations/')) {
      ops = []
      return { status: 204 }
    }
    if (r.method === 'POST' && r.path === '/operations/1/programs') return { status: 201, body: program({ id: 42, currentVersionNo: null }) }
  })
  return calls
}

test('空图纸：新增第一道工序，工序号默认 10，成功后列表刷新', async () => {
  const calls = server([])
  renderApp(view(), '/drawings/1')
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '新增第一道工序' }))
  const dialog = screen.getByRole('dialog', { name: '新增工序' })
  expect(within(dialog).getByLabelText('工序号')).toHaveValue('10')
  await u.type(within(dialog).getByLabelText('工序名称'), '粗车')
  await u.type(within(dialog).getByLabelText('X 坐标（mm）'), '-12.25')
  await u.type(within(dialog).getByLabelText('Y 坐标（mm）'), '0.125')
  await u.type(within(dialog).getByLabelText('Z 坐标（mm）'), '-25.5')
  await u.click(within(dialog).getByRole('button', { name: '保存' }))

  expect(await screen.findByRole('heading', { name: '10# 粗车' })).toBeInTheDocument()
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  expect(calls.find((c) => c.method === 'POST')?.body).toEqual({ opNo: 10, opName: '粗车', machineId: null, fixture: '', centeringMethod: '', xCoordinate: -12.25, yCoordinate: 0.125, zHeight: -25.5, remark: '' })
})

test('分中方式创建后展示，编辑回填，失败保留草稿，可修改和清空', async () => {
  let saved: ReturnType<typeof operation> | undefined
  let fail = false
  const calls = mockApi((r) => {
    if (r.path === '/drawings/1/detail') return { body: detail(saved ? [saved] : []) }
    if (r.method === 'POST' || r.method === 'PUT') {
      if (fail) return { status: 500, body: { error: '保存失败' } }
      saved = operation({ ...r.body as Record<string, unknown>, programs: [] })
      return { status: r.method === 'POST' ? 201 : 200, body: saved }
    }
  })
  renderApp(view(), '/drawings/1')
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '新增第一道工序' }))
  expect(screen.getByLabelText('分中方式')).toHaveValue('')
  expect(screen.getByLabelText('分中方式')).toHaveAttribute('maxlength', '200')
  await u.type(screen.getByLabelText('分中方式'), '  四面分中  ')
  await u.click(screen.getByRole('button', { name: '保存' }))
  await u.click(await screen.findByRole('button', { name: '展开工序 10' }))
  expect(await screen.findByText('分中方式：四面分中')).toBeInTheDocument()
  expect(calls.find((r) => r.method === 'POST')?.body).toMatchObject({ centeringMethod: '四面分中' })

  await u.click(screen.getByRole('button', { name: '编辑工序 10' }))
  expect(screen.getByLabelText('分中方式')).toHaveValue('四面分中')
  await u.clear(screen.getByLabelText('分中方式'))
  await u.type(screen.getByLabelText('分中方式'), '孔分中')
  fail = true
  await u.click(screen.getByRole('button', { name: '保存' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('保存失败')
  expect(screen.getByLabelText('分中方式')).toHaveValue('孔分中')
  fail = false
  await u.click(screen.getByRole('button', { name: '保存' }))
  expect(await screen.findByText('分中方式：孔分中')).toBeInTheDocument()

  await u.click(screen.getByRole('button', { name: '编辑工序 10' }))
  expect(screen.getByLabelText('分中方式')).toHaveValue('孔分中')
  await u.clear(screen.getByLabelText('分中方式'))
  await u.click(screen.getByRole('button', { name: '保存' }))
  await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  expect(screen.queryByText(/^分中方式：/)).not.toBeInTheDocument()
  expect(calls.filter((r) => r.method === 'PUT').at(-1)?.body).toMatchObject({ centeringMethod: '' })
})

test('建议工序号 = 现有最大号 + 10', async () => {
  server([operation({ id: 1, opNo: 10 }), operation({ id: 2, opNo: 20, programs: [] })])
  renderApp(view(), '/drawings/1')
  await userEvent.setup().click(await screen.findByRole('button', { name: '新增工序' }))
  expect(screen.getByLabelText('工序号')).toHaveValue('30')
})

test('装夹方式可选预设值，也支持自定义输入', async () => {
  const calls = server([])
  renderApp(view(), '/drawings/1')
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '新增第一道工序' }))
  const fixture = screen.getByLabelText('装夹方式')
  expect(fixture).toHaveValue('')
  expect(within(fixture).getAllByRole('option').map((option) => option.textContent)).toEqual([
    '未选择', '虎钳', '压板', 'V型工装', '吸盘', '工装板', '自定义',
  ])
  await u.selectOptions(fixture, '虎钳')
  await u.click(screen.getByRole('button', { name: '保存' }))
  expect(calls.find((c) => c.method === 'POST')?.body).toMatchObject({ fixture: '虎钳' })
})

test('自定义装夹方式会去除首尾空格后提交', async () => {
  const calls = server([])
  renderApp(view(), '/drawings/1')
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '新增第一道工序' }))
  await u.selectOptions(screen.getByLabelText('装夹方式'), '自定义')
  await u.type(screen.getByLabelText('自定义装夹方式'), '  三爪卡盘  ')
  await u.click(screen.getByRole('button', { name: '保存' }))
  expect(calls.find((c) => c.method === 'POST')?.body).toMatchObject({ fixture: '三爪卡盘' })
})

test('工序表单校验：工序号与坐标非法时不提交', async () => {
  const calls = server([])
  renderApp(view(), '/drawings/1')
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '新增第一道工序' }))
  await u.clear(screen.getByLabelText('工序号'))
  await u.type(screen.getByLabelText('工序号'), '0')
  await u.click(screen.getByRole('button', { name: '保存' }))
  expect(await screen.findByText('工序号应为 1~9999 的整数')).toBeInTheDocument()

  await u.clear(screen.getByLabelText('工序号'))
  await u.type(screen.getByLabelText('工序号'), '10')
  await u.type(screen.getByLabelText('Z 坐标（mm）'), '100001')
  await u.click(screen.getByRole('button', { name: '保存' }))
  expect(await screen.findByText('Z 坐标应为 -100000~100000 mm 的数字')).toBeInTheDocument()
  expect(calls.some((c) => c.method === 'POST')).toBe(false)
})

test('工序号重复时标在输入框上，弹层保持打开', async () => {
  mockApi((r) => {
    if (r.path === '/drawings/1/detail') return { body: detail([]) }
    if (r.method === 'POST') return { status: 409, body: { error: '记录已存在，或与现有数据冲突: 该图纸下已存在相同工序号' } }
  })
  renderApp(view(), '/drawings/1')
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '新增第一道工序' }))
  await u.click(screen.getByRole('button', { name: '保存' }))
  expect(await screen.findByText('该图纸下已存在相同工序号')).toBeInTheDocument()
  expect(screen.getByRole('dialog')).toBeInTheDocument()
})

test('编辑工序时原样带回机台，避免被后端清空', async () => {
  const calls = server([operation({ machineId: 5, machineName: 'CK6150', fixture: '三爪', xCoordinate: -10.5, yCoordinate: 20.125, zHeight: -25.5 })])
  renderApp(view(), '/drawings/1')
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '展开工序 10' }))
  await u.click(await screen.findByRole('button', { name: '编辑工序 10' }))
  const dialog = screen.getByRole('dialog', { name: '编辑工序' })
  expect(within(dialog).getByLabelText('工序名称')).toHaveValue('粗车')
  expect(within(dialog).getByLabelText('X 坐标（mm）')).toHaveValue('-10.5')
  expect(within(dialog).getByLabelText('Y 坐标（mm）')).toHaveValue('20.125')
  expect(within(dialog).getByLabelText('Z 坐标（mm）')).toHaveValue('-25.5')
  expect(within(dialog).getByLabelText('装夹方式')).toHaveDisplayValue('自定义')
  expect(within(dialog).getByLabelText('自定义装夹方式')).toHaveValue('三爪')
  await u.clear(within(dialog).getByLabelText('工序名称'))
  await u.type(within(dialog).getByLabelText('工序名称'), '精车')
  await u.click(within(dialog).getByRole('button', { name: '保存' }))
  await waitFor(() => expect(calls.some((c) => c.method === 'PUT')).toBe(true))
  expect(calls.find((c) => c.method === 'PUT')).toMatchObject({ path: '/operations/1', body: { opNo: 10, opName: '精车', machineId: 5, fixture: '三爪', xCoordinate: -10.5, yCoordinate: 20.125, zHeight: -25.5 } })
})

test.each([['X', 'abc'], ['Y', '-100001'], ['Z', 'Infinity']])('%s 坐标非法时阻止提交并保留输入', async (axis, value) => {
  const calls = server([])
  renderApp(view(), '/drawings/1')
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '新增第一道工序' }))
  const input = screen.getByLabelText(`${axis} 坐标（mm）`)
  await u.type(input, value)
  await u.click(screen.getByRole('button', { name: '保存' }))
  expect(await screen.findByRole('alert')).toHaveTextContent(`${axis} 坐标应为 -100000~100000 mm 的数字`)
  expect(input).toHaveValue(value)
  expect(input).toHaveAttribute('aria-invalid', 'true')
  expect(calls.some((c) => c.method === 'POST')).toBe(false)
})

test('旧工序未含 X/Y 时默认 0，三个坐标清空后保存为 0', async () => {
  const calls = server([operation({ xCoordinate: undefined, yCoordinate: undefined, zHeight: 25.5 })])
  renderApp(view(), '/drawings/1')
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '展开工序 10' }))
  await u.click(await screen.findByRole('button', { name: '编辑工序 10' }))
  expect(screen.getByLabelText('X 坐标（mm）')).toHaveValue('0')
  expect(screen.getByLabelText('Y 坐标（mm）')).toHaveValue('0')
  expect(screen.getByLabelText('Z 坐标（mm）')).toHaveValue('25.5')
  for (const axis of ['X', 'Y', 'Z']) await u.clear(screen.getByLabelText(`${axis} 坐标（mm）`))
  await u.click(screen.getByRole('button', { name: '保存' }))
  await waitFor(() => expect(calls.some((c) => c.method === 'PUT')).toBe(true))
  expect(calls.find((c) => c.method === 'PUT')?.body).toMatchObject({ xCoordinate: 0, yCoordinate: 0, zHeight: 0 })
})

test('保存失败保留三个坐标，可重试保存', async () => {
  let fail = true
  const calls = mockApi((r) => {
    if (r.method === 'GET') return { body: detail([operation()]) }
    if (r.method === 'PUT') return fail ? { status: 500, body: { error: '保存失败' } } : { body: operation(r.body as Record<string, unknown>) }
  })
  renderApp(view(), '/drawings/1')
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '展开工序 10' }))
  await u.click(await screen.findByRole('button', { name: '编辑工序 10' }))
  for (const [axis, value] of [['X', '-12.345'], ['Y', '67.89'], ['Z', '-0.125']]) {
    await u.clear(screen.getByLabelText(`${axis} 坐标（mm）`))
    await u.type(screen.getByLabelText(`${axis} 坐标（mm）`), value)
  }
  await u.click(screen.getByRole('button', { name: '保存' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('保存失败')
  expect(screen.getByLabelText('X 坐标（mm）')).toHaveValue('-12.345')
  expect(screen.getByLabelText('Y 坐标（mm）')).toHaveValue('67.89')
  expect(screen.getByLabelText('Z 坐标（mm）')).toHaveValue('-0.125')
  fail = false
  await u.click(screen.getByRole('button', { name: '保存' }))
  await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  expect(calls.filter((c) => c.method === 'PUT')).toHaveLength(2)
})

test('删除工序需二次确认，并说明会连带删除程序', async () => {
  const calls = server([operation()])
  renderApp(view(), '/drawings/1')
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '展开工序 10' }))
  await u.click(await screen.findByRole('button', { name: '编辑工序 10' }))
  await u.click(screen.getByRole('button', { name: '删除此工序' }))
  expect(screen.getByText(/及其下的 1 个程序和全部版本/)).toBeInTheDocument()
  expect(calls.some((c) => c.method === 'DELETE')).toBe(false)

  await u.click(screen.getByRole('button', { name: '确认删除' }))
  expect(await screen.findByText('暂无工序')).toBeInTheDocument()
  expect(calls.find((c) => c.method === 'DELETE')?.path).toBe('/operations/1')
})

test('取消删除不会发请求', async () => {
  const calls = server([operation()])
  renderApp(view(), '/drawings/1')
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '展开工序 10' }))
  await u.click(await screen.findByRole('button', { name: '编辑工序 10' }))
  await u.click(screen.getByRole('button', { name: '删除此工序' }))
  await u.click(screen.getByRole('button', { name: '取消' }))
  expect(calls.some((c) => c.method === 'DELETE')).toBe(false)
})

test('新增程序：校验程序号，成功后进入程序页', async () => {
  const calls = server([operation({ programs: [] })])
  renderApp(view(), '/drawings/1')
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '展开工序 10' }))
  await u.click(await screen.findByRole('button', { name: '在 10# 下新增程序' }))
  await u.click(screen.getByRole('button', { name: '创建并上传文件' }))
  expect(await screen.findByText('请输入程序号')).toBeInTheDocument()
  expect(calls.some((c) => c.method === 'POST')).toBe(false)

  await u.type(screen.getByLabelText('程序号'), 'O1234')
  await u.type(screen.getByLabelText('数控系统'), 'FANUC')
  await u.click(screen.getByRole('button', { name: '创建并上传文件' }))
  expect(await screen.findByText('程序页')).toBeInTheDocument()
  expect(calls.find((c) => c.method === 'POST')?.body).toEqual({ programNo: 'O1234', programName: '', controller: 'FANUC', remark: '' })
})

test('Esc 关闭弹层', async () => {
  server([])
  renderApp(view(), '/drawings/1')
  const u = userEvent.setup()
  await u.click(await screen.findByRole('button', { name: '新增第一道工序' }))
  expect(screen.getByRole('dialog')).toBeInTheDocument()
  await u.keyboard('{Escape}')
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
})
