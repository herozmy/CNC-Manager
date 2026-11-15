import { act, fireEvent, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, test, vi } from 'vitest'
import { api } from '../api'
import type { UploadResult } from '../api/types'
import { renderApp, version } from '../test-utils'
import UploadSheet from './UploadSheet'

afterEach(() => vi.restoreAllMocks())

test('上传进行中阻止关闭和重复提交，完成后只回调一次', async () => {
  let finish!: (value: Awaited<ReturnType<typeof api.uploadVersion>>) => void
  const upload = vi.spyOn(api, 'uploadVersion').mockImplementation(() => new Promise((resolve) => { finish = resolve }))
  const onClose = vi.fn(), onDone = vi.fn()
  renderApp(<UploadSheet drawingId={1} programId={1} file={new File(['O1\nM30'], 'O1.nc')} onClose={onClose} onDone={onDone} />)
  const u = userEvent.setup()
  const form = screen.getByRole('button', { name: '上传' }).closest('form')!
  fireEvent.submit(form)
  fireEvent.submit(form)
  expect(upload).toHaveBeenCalledTimes(1)
  expect(screen.getByLabelText('变更说明（可选）')).toBeDisabled()
  await u.click(screen.getByRole('button', { name: '关闭' }))
  await u.keyboard('{Escape}')
  fireEvent.click(screen.getByRole('dialog').parentElement!)
  expect(onClose).not.toHaveBeenCalled()
  const leave = new Event('beforeunload', { cancelable: true })
  window.dispatchEvent(leave)
  expect(leave.defaultPrevented).toBe(true)
  await act(async () => finish(version() as UploadResult))
  await waitFor(() => expect(onDone).toHaveBeenCalledTimes(1))
  const settledLeave = new Event('beforeunload', { cancelable: true })
  window.dispatchEvent(settledLeave)
  expect(settledLeave.defaultPrevented).toBe(false)
})

test('网络失败保留文件与说明可重试，重试成功前不显示上传完成', async () => {
  const upload = vi.spyOn(api, 'uploadVersion').mockRejectedValueOnce(new TypeError('offline')).mockResolvedValueOnce(version() as UploadResult)
  const onDone = vi.fn(), onClose = vi.fn()
  const file = new File(['O1\nM30'], '手机测试.nc')
  renderApp(<UploadSheet drawingId={1} programId={1} file={file} onClose={onClose} onDone={onDone} />)
  const u = userEvent.setup()
  await u.type(screen.getByLabelText('变更说明（可选）'), '保留说明')
  await u.click(screen.getByRole('button', { name: '上传' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('上传失败')
  expect(screen.getByLabelText('变更说明（可选）')).toHaveValue('保留说明')
  expect(onDone).not.toHaveBeenCalled()
  await u.click(screen.getByRole('button', { name: '上传' }))
  await waitFor(() => expect(onDone).toHaveBeenCalledTimes(1))
  expect(upload).toHaveBeenNthCalledWith(2, 1, file, '保留说明')
})
