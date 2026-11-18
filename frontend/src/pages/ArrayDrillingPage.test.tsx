import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test, vi } from 'vitest'
import { renderApp, mockApi, user } from '../test-utils'
import ArrayDrillingPage from './ArrayDrillingPage'
import App from '../App'

test('直线默认高度、示例、方向与方式联动，错误输入撤销程序和下载', async () => {
  renderApp(<ArrayDrillingPage kind="line" />)
  const u = userEvent.setup()
  expect(screen.getByLabelText('初始安全高度 Z（mm）')).toHaveValue('30')
  expect(screen.getByLabelText('循环 R 平面 Z（mm）')).toHaveValue('3.0')
  expect(screen.getByRole('button', { name: '下载 NC' })).toBeDisabled()
  await u.click(screen.getByRole('button', { name: '载入示例' }))
  const code = () => (screen.getByLabelText('生成的 NC 程序（只读）') as HTMLTextAreaElement).value
  expect(code()).toContain('G91 X20. K4')
  await u.selectOptions(screen.getByLabelText('排列轴'), 'Y')
  await u.selectOptions(screen.getByLabelText('排列方向'), 'negative')
  expect(code()).toContain('G91 Y-20. K4')
  await u.selectOptions(screen.getByLabelText('编程方式'), 'absolute')
  expect(code()).toContain('X0. Y-80.')
  const write = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue(undefined)
  await u.click(screen.getByRole('button', { name: '复制程序' }))
  expect(write).toHaveBeenCalledWith(code().replaceAll('\n','\r\n'));write.mockRestore()
  await u.clear(screen.getByLabelText('初始安全高度 Z（mm）'))
  await u.type(screen.getByLabelText('初始安全高度 Z（mm）'), '2')
  expect(screen.getByRole('alert')).toHaveTextContent('不低于')
  expect(screen.queryByLabelText('生成的 NC 程序（只读）')).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: '下载 NC' })).toBeDisabled()
})
test('矩形蛇形切换顺序，过多孔数不能导出', async () => {
  renderApp(<ArrayDrillingPage kind="grid" />)
  const u = userEvent.setup()
  await u.click(screen.getByRole('button', { name: '载入示例' }))
  const code = () => (screen.getByLabelText('生成的 NC 程序（只读）') as HTMLTextAreaElement).value
  expect(code()).toContain('X60. Y0.\nX60. Y20.')
  await u.click(screen.getByLabelText('蛇形顺序（相邻行反向）'))
  expect(code()).toContain('X60. Y0.\nX0. Y20.')
  await u.clear(screen.getByLabelText('行数（Y 方向）'))
  await u.type(screen.getByLabelText('行数（Y 方向）'), '121')
  expect(screen.getByRole('alert')).toHaveTextContent('总孔数不能超过 360')
  expect(screen.getByRole('button', { name: '复制程序' })).toBeDisabled()
})
test('工具卡片进入和返回、两种表单互不串值', async () => {
  const calls = mockApi(r => {
    if (r.path === '/auth/status') return { body: { setupRequired: false } }
    if (r.path === '/auth/me') return { body: user }
    if (r.path === '/meta') return { body: { version: '2.4.0' } }
  })
  renderApp(<App />, '/tools/gcode')
  const u = userEvent.setup()
  await u.click(await screen.findByRole('link', { name: /^钻孔编程/ }))
  await u.selectOptions(screen.getByLabelText('钻孔类型'), 'line-drilling')
  expect(screen.getByRole('heading', { name: '直线等距钻孔' })).toBeInTheDocument()
  await u.click(screen.getByRole('button', { name: '载入示例' }))
  await u.selectOptions(screen.getByLabelText('钻孔类型'), 'grid-drilling')
  expect(screen.getByRole('heading', { name: '矩形阵列钻孔' })).toBeInTheDocument()
  expect(screen.queryByLabelText('生成的 NC 程序（只读）')).not.toBeInTheDocument()
  await u.selectOptions(screen.getByLabelText('钻孔类型'), 'polar-drilling')
  expect(screen.getByRole('heading', { name: '极坐标打孔' })).toBeInTheDocument()
  expect(screen.queryByRole('link', { name: '进入编程' })).not.toBeInTheDocument()
  await u.click(screen.getByRole('link', { name: '返回 G 代码编程' }))
  await u.click(screen.getByRole('link', { name: /^钻孔编程/ }))
  expect(screen.getByLabelText('钻孔类型')).toHaveValue('polar-drilling')
  expect(calls.every(r => r.method === 'GET')).toBe(true)
})
