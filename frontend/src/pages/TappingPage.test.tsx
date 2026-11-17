import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, test, vi } from 'vitest'
import { mockApi, renderApp, user } from '../test-utils'
import TappingPage from './TappingPage'
import App from '../App'

afterEach(() => vi.unstubAllGlobals())

test('选择粗细牙即时更新底孔、螺距和进给，更换直径恢复对应粗牙', async () => {
  renderApp(<TappingPage />)
  const u = userEvent.setup()
  expect(screen.getByLabelText('底孔参考直径')).toHaveTextContent('5 mm')
  expect(screen.queryByLabelText('每分钟进给')).not.toBeInTheDocument()
  await u.type(screen.getByLabelText('主轴转速（r/min）'), '500')
  expect(screen.getByLabelText('每分钟进给')).toHaveTextContent('500 mm/min')
  await u.selectOptions(screen.getByLabelText('公称直径'), '10')
  expect(screen.getByLabelText('螺距（mm）')).toHaveValue('1.5')
  await u.selectOptions(screen.getByLabelText('螺距（mm）'), '1.25')
  expect(screen.getByLabelText('底孔参考直径')).toHaveTextContent('8.8 mm')
  expect(screen.getByLabelText('每分钟进给')).toHaveTextContent('625 mm/min')
  expect(screen.getByLabelText('每转进给')).toHaveTextContent('1.25 mm/rev')
  await u.selectOptions(screen.getByLabelText('公称直径'), '12')
  expect(screen.getByLabelText('螺距（mm）')).toHaveValue('1.75')
  expect(screen.getByLabelText('底孔参考直径')).toHaveTextContent('10.2 mm')
  expect(screen.getByLabelText('每分钟进给')).toHaveTextContent('875 mm/min')
  expect(screen.getByRole('link', { name: /底孔数据来源/ })).toHaveAttribute('href', 'https://www.guhring.co.jp/techinfo/techinfo03/')
})

test('仅输入转速，空值和错误输入清除结果，修正后恢复计算', async () => {
  renderApp(<TappingPage />)
  const u = userEvent.setup()
  expect(screen.queryByText(/切削速度/)).not.toBeInTheDocument()
  expect(screen.queryByRole('radio')).not.toBeInTheDocument()
  const input = screen.getByLabelText('主轴转速（r/min）')
  await u.type(input, '500')
  expect(screen.getByLabelText('每分钟进给')).toHaveTextContent('500 mm/min')
  await u.clear(input)
  expect(screen.queryByLabelText('每分钟进给')).not.toBeInTheDocument()
  for (const value of ['0', '-1', '500.5', 'abc']) {
    await u.clear(input)
    await u.type(input, value)
    expect(screen.getByRole('alert')).toHaveTextContent('请输入大于 0 的有效整数转速')
    expect(screen.queryByLabelText('每分钟进给')).not.toBeInTheDocument()
  }
  await u.clear(input)
  await u.type(input, '600')
  expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  expect(screen.getByLabelText('每分钟进给')).toHaveTextContent('600 mm/min')
})

test.each(['admin', 'editor'] as const)('%s可直接访问助手，导航选中正确，不请求业务或写入接口', async role => {
  const calls = mockApi(r => {
    if (r.path === '/auth/status') return { body: { setupRequired: false } }
    if (r.path === '/auth/me') return { body: { ...user, role } }
    if (r.path === '/meta') return { body: { version: '2.3.4', initialized: true } }
  })
  renderApp(<App />, '/tools/tapping')
  expect(await screen.findByRole('heading', { name: '公制攻丝' })).toBeInTheDocument()
  for (const name of ['主导航', '桌面导航']) {
    const nav = within(screen.getByRole('navigation', { name }))
    expect(nav.getByRole('link', { name: '加工助手' })).toHaveAttribute('aria-current', 'page')
    expect(nav.getByRole('link', { name: '总览' })).not.toHaveAttribute('aria-current')
    expect(nav.getByRole('link', { name: '高级管理' })).not.toHaveAttribute('aria-current')
  }
  expect(calls.every(r => r.method === 'GET' && ['/auth/status', '/auth/me', '/meta'].includes(r.path))).toBe(true)
})


test.each(['admin', 'editor'] as const)('%s从助手卡片进入才显示攻丝内容，返回后收起，再进入默认粗牙', async role => {
  mockApi(r => {
    if (r.path === '/auth/status') return { body: { setupRequired: false } }
    if (r.path === '/auth/me') return { body: { ...user, role } }
    if (r.path === '/meta') return { body: { version: '2.3.4' } }
  })
  renderApp(<App />, '/tools')
  const u = userEvent.setup()
  expect(await screen.findByRole('heading', { name: '加工助手' })).toBeInTheDocument()
  expect(screen.queryByLabelText('公称直径')).not.toBeInTheDocument()
  expect(screen.queryByLabelText('主轴转速（r/min）')).not.toBeInTheDocument()
  const card = screen.getByRole('link', { name: /公制攻丝/ })
  expect(card).toHaveAttribute('href', '/tools/tapping')
  await u.click(card)
  expect(await screen.findByRole('heading', { name: '公制攻丝' })).toBeInTheDocument()
  expect(screen.getByLabelText('公称直径')).toHaveValue('6')
  expect(screen.getByLabelText('螺距（mm）')).toHaveDisplayValue('1 · 粗牙')
  await u.selectOptions(screen.getByLabelText('螺距（mm）'), '0.75')
  await u.click(screen.getByRole('link', { name: '返回加工助手' }))
  expect(await screen.findByRole('heading', { name: '加工助手' })).toBeInTheDocument()
  expect(screen.queryByLabelText('公称直径')).not.toBeInTheDocument()
  await u.click(screen.getByRole('link', { name: /公制攻丝/ }))
  expect(screen.getByLabelText('螺距（mm）')).toHaveDisplayValue('1 · 粗牙')
  await u.selectOptions(screen.getByLabelText('公称直径'), '8')
  expect(screen.getByLabelText('螺距（mm）')).toHaveDisplayValue('1.25 · 粗牙')
})
