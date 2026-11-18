import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test } from 'vitest'
import { renderApp } from '../test-utils'
import PolarDrillingPage from './PolarDrillingPage'
import ArrayDrillingPage from './ArrayDrillingPage'

test.each(['polar', 'line', 'grid'] as const)('%s攻丝螺距和转速联动只读F，错误撤销导出，循环切换不混用参数', async kind => {
  renderApp(kind === 'polar' ? <PolarDrillingPage /> : <ArrayDrillingPage kind={kind} />)
  const u = userEvent.setup()
  await u.click(screen.getByRole('button', { name: '载入示例' }))
  await u.selectOptions(screen.getByLabelText('循环方式'), 'G84')
  expect(screen.getByRole('button', { name: '下载 NC' })).toBeDisabled()
  expect(screen.queryByLabelText('钻孔进给 F（mm/min）')).not.toBeInTheDocument()
  expect(screen.queryByLabelText('每次钻深 Q（mm）')).not.toBeInTheDocument()
  await u.type(screen.getByLabelText('攻丝螺距（mm）'), '1.25')
  const code = () => (screen.getByLabelText('生成的 NC 程序（只读）') as HTMLTextAreaElement).value
  expect(screen.getByLabelText('攻丝进给 F（mm/min）')).toHaveValue('1250')
  expect(screen.getByLabelText('攻丝进给 F（mm/min）')).toHaveAttribute('readonly')
  expect(code()).toContain('M29 S1000\nG98 G84')
  expect(code()).toContain('F1250.')
  await u.clear(screen.getByLabelText('主轴转速 S（r/min）'))
  await u.type(screen.getByLabelText('主轴转速 S（r/min）'), '600')
  expect(screen.getByLabelText('攻丝进给 F（mm/min）')).toHaveValue('750')
  expect(code()).toContain('F750.')
  await u.clear(screen.getByLabelText('攻丝螺距（mm）'))
  await u.type(screen.getByLabelText('攻丝螺距（mm）'), '0')
  expect(screen.getByRole('button', { name: '复制程序' })).toBeDisabled()
  expect(screen.getByLabelText('攻丝进给 F（mm/min）')).toHaveValue('')
  await u.selectOptions(screen.getByLabelText('循环方式'), 'G81')
  expect(code()).toContain('G98 G81')
  expect(code()).toContain('F100.')
  expect(code()).not.toContain('M29')
  expect(screen.getByLabelText('钻孔进给 F（mm/min）')).toHaveValue('100')
  await u.selectOptions(screen.getByLabelText('循环方式'), 'G83')
  expect(screen.getByRole('button', { name: '下载 NC' })).toBeDisabled()
  await u.type(screen.getByLabelText('每次钻深 Q（mm）'), '3')
  expect(code()).toContain('Q3. F100.')
  expect(code()).not.toContain('M29')
})
