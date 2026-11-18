import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test } from 'vitest'
import { renderApp } from '../test-utils'
import PolarDrillingPage from './PolarDrillingPage'
import ArrayDrillingPage from './ArrayDrillingPage'

test.each(['polar', 'line', 'grid'] as const)('%s页面G83必填Q，错误撤销导出，返回G81不带旧Q', async kind => {
  renderApp(kind === 'polar' ? <PolarDrillingPage /> : <ArrayDrillingPage kind={kind} />)
  const u = userEvent.setup()
  expect(screen.getByLabelText('循环方式')).toHaveValue('G81')
  expect(screen.queryByLabelText('每次钻深 Q（mm）')).not.toBeInTheDocument()
  await u.click(screen.getByRole('button', { name: '载入示例' }))
  await u.selectOptions(screen.getByLabelText('循环方式'), 'G83')
  expect(screen.queryByLabelText('生成的 NC 程序（只读）')).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: '下载 NC' })).toBeDisabled()
  await u.type(screen.getByLabelText('每次钻深 Q（mm）'), '2.5')
  const code = () => (screen.getByLabelText('生成的 NC 程序（只读）') as HTMLTextAreaElement).value
  expect(code()).toContain('G98 G83')
  expect(code()).toContain('Q2.5 F100.')
  await u.clear(screen.getByLabelText('每次钻深 Q（mm）'))
  await u.type(screen.getByLabelText('每次钻深 Q（mm）'), '-1')
  expect(screen.getByRole('alert')).toHaveTextContent('0.001')
  expect(screen.getByRole('button', { name: '复制程序' })).toBeDisabled()
  await u.selectOptions(screen.getByLabelText('循环方式'), 'G81')
  expect(code()).toContain('G98 G81')
  expect(code()).not.toContain('Q')
  expect(screen.queryByLabelText('每次钻深 Q（mm）')).not.toBeInTheDocument()
  await u.selectOptions(screen.getByLabelText('循环方式'), 'G83')
  expect(screen.getByLabelText('每次钻深 Q（mm）')).toHaveValue('-1')
  expect(screen.getByRole('button', { name: '下载 NC' })).toBeDisabled()
})
