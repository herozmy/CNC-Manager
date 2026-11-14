import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, test, vi } from 'vitest'
import { mockApi, renderApp, version } from '../test-utils'
import NCEditorSheet from './NCEditorSheet'

afterEach(() => vi.unstubAllGlobals())
const source = { versionId: 5, programId: 1, versionNo: 3, fileName: 'part.nc', encoding: 'gbk', content: 'O1000\r\nM30\r\n', lineCount: 2, sizeBytes: 14, isCurrent: false }
function setup(fail = false) {
  const calls = mockApi((r) => r.method === 'GET' ? { body: source } : fail ? { status: 500, body: { error: '磁盘写入失败' } } : { body: { ...version({ id: 6, versionNo: 4 }), warning: '编码替换提示' } })
  const onClose = vi.fn(), onSaved = vi.fn()
  renderApp(<NCEditorSheet versionId={5} drawingId={1} programId={1} onClose={onClose} onSaved={onSaved} />)
  return { calls, onClose, onSaved, u: userEvent.setup() }
}

test('默认只读，另存保留文件名、GBK 和 CRLF，返回警告', async () => {
  const { calls, onSaved, u } = setup()
  const text = await screen.findByLabelText('NC 程序内容')
  expect(text).toHaveAttribute('readonly')
  await u.click(screen.getByRole('button', { name: '编辑内容' }))
  expect(text).not.toHaveAttribute('readonly')
  await u.clear(text)
  await u.type(text, 'O1000\nG01 X2\nM30\n')
  await u.type(screen.getByLabelText('变更说明（另存新版本时生效）'), '调整尺寸')
  await u.click(screen.getByRole('button', { name: '另存新版本' }))
  await waitFor(() => expect(onSaved).toHaveBeenCalledWith(expect.objectContaining({ warning: '编码替换提示' })))
  expect(calls.find((r) => r.method === 'POST')).toMatchObject({ path: '/programs/1/versions/content', body: { content: 'O1000\r\nG01 X2\r\nM30\r\n', encoding: 'gbk', fileName: 'part.nc', changeNote: '调整尺寸' } })
})

test('覆盖历史版本须确认，失败保留草稿', async () => {
  const { calls, onSaved, u } = setup(true)
  await u.click(await screen.findByRole('button', { name: '编辑内容' }))
  await u.type(screen.getByLabelText('NC 程序内容'), 'G00')
  await u.click(screen.getByRole('button', { name: '覆盖此版本' }))
  expect(calls.some((r) => r.method === 'PUT')).toBe(false)
  await u.click(screen.getByRole('button', { name: '确认覆盖' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('磁盘写入失败')
  expect((screen.getByLabelText('NC 程序内容') as HTMLTextAreaElement).value).toContain('G00')
  expect(calls.find((r) => r.method === 'PUT')?.path).toBe('/versions/5/content')
  expect(onSaved).not.toHaveBeenCalled()
})

test('未保存关闭须确认，继续编辑保留草稿', async () => {
  const { onClose, u } = setup()
  await u.click(await screen.findByRole('button', { name: '编辑内容' }))
  await u.type(screen.getByLabelText('NC 程序内容'), 'X1')
  await u.click(screen.getByRole('button', { name: '关闭' }))
  expect(onClose).not.toHaveBeenCalled()
  await u.click(screen.getByRole('button', { name: '继续编辑' }))
  expect(screen.queryByText('有未保存的修改，确定放弃吗？')).not.toBeInTheDocument()
  await u.keyboard('{Escape}')
  await u.click(screen.getByRole('button', { name: '放弃修改' }))
  expect(onClose).toHaveBeenCalledOnce()
})

test('加载失败后可重试', async () => {
  let fail = true
  mockApi(() => fail ? { status: 500 } : { body: source })
  renderApp(<NCEditorSheet versionId={5} drawingId={1} programId={1} onClose={() => {}} onSaved={() => {}} />)
  expect(await screen.findByText('NC 内容加载失败')).toBeInTheDocument()
  fail = false
  await userEvent.click(screen.getByRole('button', { name: /重试/ }))
  expect(await screen.findByLabelText('NC 程序内容')).toHaveAttribute('readonly')
})
