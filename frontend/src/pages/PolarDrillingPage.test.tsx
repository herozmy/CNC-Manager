import { act, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, test, vi } from 'vitest'
import App from '../App'
import { mockApi, renderApp, user } from '../test-utils'
import PolarDrillingPage from './PolarDrillingPage'

afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks() })

test('示例生成NC与孔位，改成无效参数立即撤销程序并禁止复制下载', async () => {
  renderApp(<PolarDrillingPage />)
  const u = userEvent.setup()
  expect(screen.getByLabelText('初始安全高度 Z（mm）')).toHaveValue('30')
  expect(screen.getByLabelText('循环 R 平面 Z（mm）')).toHaveValue('3.0')
  expect(screen.queryByLabelText('刀长补偿号 H')).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: '复制程序' })).toBeDisabled()
  expect(screen.queryByLabelText('生成的 NC 程序（只读）')).not.toBeInTheDocument()
  await u.click(screen.getByRole('button', { name: '载入示例' }))
  expect(screen.getByRole('img', { name: /6 孔极坐标示意图/ })).toBeInTheDocument()
  expect((screen.getByLabelText('生成的 NC 程序（只读）') as HTMLTextAreaElement).value).toContain('G98 G81 X50. Y0.')
  await u.clear(screen.getByLabelText('刀号 T'))
  await u.type(screen.getByLabelText('刀号 T'), '12')
  expect((screen.getByLabelText('生成的 NC 程序（只读）') as HTMLTextAreaElement).value).toContain('G00 G43 H12 Z30.')
  expect(screen.getByRole('button', { name: '下载 NC' })).toBeEnabled()
  await u.clear(screen.getByLabelText('初始安全高度 Z（mm）'))
  await u.type(screen.getByLabelText('初始安全高度 Z（mm）'), '2')
  expect(screen.getByRole('alert')).toHaveTextContent('不低于循环 R 平面')
  expect(screen.queryByLabelText('生成的 NC 程序（只读）')).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: '下载 NC' })).toBeDisabled()
  expect(screen.getByRole('button', { name: '复制程序' })).toBeDisabled()
})

test('顺时针、指定间角、返回方式和冷却均反映到程序', async () => {
  renderApp(<PolarDrillingPage />)
  const u = userEvent.setup()
  await u.click(screen.getByRole('button', { name: '载入示例' }))
  await u.selectOptions(screen.getByLabelText('布孔方式'), 'step')
  await u.type(screen.getByLabelText('孔间角度（°）'), '30')
  await u.selectOptions(screen.getByLabelText('孔位顺序'), 'cw')
  await u.selectOptions(screen.getByLabelText('孔间返回方式'), 'G99')
  await u.click(screen.getByLabelText('钻孔时开启冷却 M08'))
  const code = (screen.getByLabelText('生成的 NC 程序（只读）') as HTMLTextAreaElement).value
  expect(code).toContain('G99 G81')
  expect(code).toContain('Y330.')
  expect(code).not.toContain('M08')
  await u.clear(screen.getByLabelText('孔数'))
  await u.type(screen.getByLabelText('孔数'), '13')
  expect(screen.getByRole('alert')).toHaveTextContent('首末孔跨度须小于 360')
  expect(screen.queryByLabelText('生成的 NC 程序（只读）')).not.toBeInTheDocument()
})

test('复制失败保留代码并可重试，迟到复制消息不标记新程序已复制', async () => {
  renderApp(<PolarDrillingPage />)
  const u = userEvent.setup()
  const copy = vi.spyOn(navigator.clipboard, 'writeText').mockRejectedValueOnce(new Error('denied'))
  await u.click(screen.getByRole('button', { name: '载入示例' }))
  await u.click(screen.getByRole('button', { name: '复制程序' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('复制失败')
  expect(screen.getByLabelText('生成的 NC 程序（只读）')).toBeInTheDocument()
  let finish!: () => void
  copy.mockImplementationOnce(() => new Promise<void>(resolve => { finish = resolve }))
  await u.click(screen.getByRole('button', { name: '复制程序' }))
  await u.clear(screen.getByLabelText('孔圆半径（mm）'))
  await u.type(screen.getByLabelText('孔圆半径（mm）'), '60')
  await act(async () => finish())
  expect(screen.queryByText('已复制程序。')).not.toBeInTheDocument()
  copy.mockResolvedValueOnce()
  await u.click(screen.getByRole('button', { name: '复制程序' }))
  expect(await screen.findByText('已复制程序。')).toBeInTheDocument()
  expect(copy).toHaveBeenLastCalledWith(expect.stringContaining('X60. Y0.'))
})

test('下载以程序号命名，Blob使用当前ASCII程序；缺少输入不会再次下载', async () => {
  renderApp(<PolarDrillingPage />)
  const u = userEvent.setup()
  const create = vi.fn((_blob: Blob) => 'blob:polar-test'), revoke = vi.fn()
  vi.stubGlobal('URL', class extends URL { static createObjectURL = create; static revokeObjectURL = revoke })
  const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (this: HTMLAnchorElement) {
    expect(this.download).toBe('O0123.nc')
  })
  await u.click(screen.getByRole('button', { name: '载入示例' }))
  await u.clear(screen.getByLabelText('程序号 O'))
  await u.type(screen.getByLabelText('程序号 O'), '123')
  await u.click(screen.getByRole('button', { name: '下载 NC' }))
  expect(click).toHaveBeenCalledTimes(1)
  expect(create).toHaveBeenCalledTimes(1)
  const blob = create.mock.calls[0][0] as Blob
  const raw = await new Promise<string>(resolve => { const reader = new FileReader(); reader.onload = () => resolve(String(reader.result)); reader.readAsText(blob) })
  expect(raw).toContain('O0123\r\n')
  expect([...raw].every(c => c.charCodeAt(0) <= 127)).toBe(true)
  expect(raw).toContain('G80\r\nG15')
  await u.clear(screen.getByLabelText('主轴转速 S（r/min）'))
  await u.click(screen.getByRole('button', { name: '下载 NC' }))
  expect(click).toHaveBeenCalledTimes(1)
})

test.each(['admin', 'editor'] as const)('%s由工具卡片逐级进入与返回，不恢复暂停参数卡片，不写入业务数据', async role => {
  const calls = mockApi(r => {
    if (r.path === '/auth/status') return { body: { setupRequired: false } }
    if (r.path === '/auth/me') return { body: { ...user, role } }
    if (r.path === '/meta') return { body: { version: '2.4.0' } }
  })
  renderApp(<App />, '/tools')
  const u = userEvent.setup()
  await u.click(await screen.findByRole('link', { name: /G 代码编程/ }))
  expect(await screen.findByRole('heading', { name: 'G 代码编程' })).toBeInTheDocument()
  expect(screen.queryByLabelText('孔圆半径（mm）')).not.toBeInTheDocument()
  expect(screen.queryByRole('link', { name: /加工参数计算/ })).not.toBeInTheDocument()
  await u.click(screen.getByRole('link', { name: /^钻孔编程/ }))
  expect(screen.getByLabelText('钻孔类型')).toHaveValue('polar-drilling')
  expect(await screen.findByRole('heading', { name: '极坐标打孔' })).toBeInTheDocument()
  expect(screen.getAllByRole('link', { name: '加工助手' }).every(link => link.getAttribute('aria-current') === 'page')).toBe(true)
  await u.click(screen.getByRole('link', { name: '返回 G 代码编程' }))
  expect(await screen.findByRole('heading', { name: 'G 代码编程' })).toBeInTheDocument()
  expect(screen.queryByLabelText('孔圆半径（mm）')).not.toBeInTheDocument()
  await u.click(screen.getByRole('link', { name: '返回加工助手' }))
  expect(await screen.findByRole('link', { name: /公制攻丝/ })).toBeInTheDocument()
  expect(calls.every(r => r.method === 'GET' && ['/auth/status', '/auth/me', '/meta'].includes(r.path))).toBe(true)
})

test('直接访问极坐标路由也能生成示例', async () => {
  mockApi(r => {
    if (r.path === '/auth/status') return { body: { setupRequired: false } }
    if (r.path === '/auth/me') return { body: user }
  })
  renderApp(<App />, '/tools/gcode/polar-drilling')
  await userEvent.setup().click(await screen.findByRole('button', { name: '载入示例' }))
  await waitFor(() => expect(screen.getByRole('button', { name: '下载 NC' })).toBeEnabled())
})

test('切换G16内G90逐孔与G91重复，复制使用当前方式且总孔数含首孔', async () => {
  renderApp(<PolarDrillingPage />)
  const u = userEvent.setup()
  const writeText = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue(undefined)
  await u.click(screen.getByRole('button', { name: '载入示例' }))
  await u.clear(screen.getByLabelText('孔数'))
  await u.type(screen.getByLabelText('孔数'), '8')
  await u.selectOptions(screen.getByLabelText('编程方式'), 'repeat')
  const code = (screen.getByLabelText('生成的 NC 程序（只读）') as HTMLTextAreaElement).value
  expect(code).toContain('G91 Y45. K7')
  expect(code).not.toContain('\nY45.')
  await u.click(screen.getByRole('button', { name: '复制程序' }))
  await waitFor(() => expect(writeText).toHaveBeenCalledWith(code.replaceAll('\n', '\r\n')))
  await u.selectOptions(screen.getByLabelText('编程方式'), 'absolute')
  expect((screen.getByLabelText('生成的 NC 程序（只读）') as HTMLTextAreaElement).value).toContain('\nY45.')
  expect((screen.getByLabelText('生成的 NC 程序（只读）') as HTMLTextAreaElement).value).not.toContain('G91')
})
