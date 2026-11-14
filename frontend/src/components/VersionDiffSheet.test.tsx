import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, test, vi } from 'vitest'
import { mockApi, renderApp, version } from '../test-utils'
import VersionDiffSheet from './VersionDiffSheet'

afterEach(() => vi.unstubAllGlobals())
const changed = { type: 'change', leftNo: 2, rightNo: 2, leftText: 'G00 X1', rightText: 'G01 X2' }
const result = { leftVersionNo: 2, rightVersionNo: 3, totalChanges: 1, totalLines: 1, identical: false, truncated: false, lines: [changed] }
const versions = [version(), version({ id: 4, versionNo: 2, isCurrent: false })]

test('默认比较前一版与目标版，支持全部行、换向和相同版本提示', async () => {
  const calls = mockApi((r) => r.path === '/programs/1/versions' ? { body: versions } : { body: result })
  renderApp(<VersionDiffSheet programId={1} versionId={5} onClose={() => {}} />)
  const u = userEvent.setup()
  expect(await screen.findByText('G01 X2')).toBeInTheDocument()
  expect(screen.getByText('G00 X1')).toBeInTheDocument()
  expect(calls.at(-1)?.path).toBe('/versions/4/diff?with=5&offset=0&limit=200&changesOnly=1')
  await u.click(screen.getByLabelText('仅显示差异'))
  await waitFor(() => expect(calls.at(-1)?.path).toContain('changesOnly=0'))
  await u.click(screen.getByRole('button', { name: '交换方向' }))
  await waitFor(() => expect(calls.at(-1)?.path).toBe('/versions/5/diff?with=4&offset=0&limit=200&changesOnly=0'))
  await u.selectOptions(screen.getByLabelText('对照版本'), '4')
  expect(screen.getByText('请选择两个不同的版本')).toBeInTheDocument()
  expect(screen.queryByRole('table')).not.toBeInTheDocument()
})

test('分页仅加载本页、切换筛选重置页码、超大差异提示', async () => {
  const calls = mockApi((r) => r.path === '/programs/1/versions' ? { body: versions } : { body: { ...result, totalLines: 201, totalChanges: 201, truncated: true, lines: r.path.includes('offset=200') ? [{ ...changed, rightText: 'last line' }] : [changed] } })
  renderApp(<VersionDiffSheet programId={1} versionId={5} onClose={() => {}} />)
  const u = userEvent.setup()
  expect(await screen.findByText(/部分内容按整块替换/)).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '上一页' })).toBeDisabled()
  await u.click(screen.getByRole('button', { name: '下一页' }))
  expect(await screen.findByText('last line')).toBeInTheDocument()
  expect(screen.queryByText('G01 X2')).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: '下一页' })).toBeDisabled()
  await u.click(screen.getByLabelText('仅显示差异'))
  await waitFor(() => expect(calls.at(-1)?.path).toContain('offset=0&limit=200&changesOnly=0'))
})

test('错误可重试，相同内容给出明确结果', async () => {
  let fail = true
  mockApi((r) => r.path === '/programs/1/versions' ? { body: versions } : fail ? { status: 500 } : { body: { ...result, identical: true, totalLines: 0, totalChanges: 0, lines: [] } })
  renderApp(<VersionDiffSheet programId={1} versionId={5} onClose={() => {}} />)
  expect(await screen.findByText('版本对比失败')).toBeInTheDocument()
  fail = false
  await userEvent.click(screen.getByRole('button', { name: /重试/ }))
  expect(await screen.findByText('两版内容相同')).toBeInTheDocument()
})
