import { fireEvent, screen, waitFor } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'
import { api } from '../api'
import { renderApp } from '../test-utils'
import PhotoPreview from './PhotoPreview'

afterEach(() => { vi.restoreAllMocks() })

test('保存失败保留修改的说明，重试沿用原修订号', async () => {
  const mark = { kind: 'text' as const, x: 30, y: 40, x2: 30, y2: 40, text: '旧说明' }
  vi.spyOn(api, 'photoAnnotations').mockResolvedValue({ marks: [mark], revision: 3 })
  const save = vi.spyOn(api, 'savePhotoAnnotations').mockRejectedValueOnce(new Error('连接中断'))
    .mockResolvedValue({ marks: [{ ...mark, text: '固定端' }], revision: 4 })
  renderApp(<PhotoPreview operationId={1} digest="photo" src="/photo" editable onClose={() => {}}>{null}</PhotoPreview>)
  const img = screen.getByAltText('装夹照片大图')
  Object.defineProperties(img, { naturalWidth: { value: 800 }, naturalHeight: { value: 600 } })
  fireEvent.load(img)
  await screen.findByText('旧说明')
  fireEvent.click(screen.getByText('编辑标注'))
  fireEvent.click(screen.getByText('修改已有标注（1）'))
  fireEvent.change(screen.getByLabelText('修改第 1 处说明'), { target: { value: '固定端' } })
  fireEvent.click(screen.getByText('保存标注'))
  expect(await screen.findByRole('alert')).toHaveTextContent('连接中断')
  expect(screen.getByLabelText('修改第 1 处说明')).toHaveValue('固定端')
  fireEvent.click(screen.getByText('保存标注'))
  await waitFor(() => expect(screen.queryByText('保存标注')).not.toBeInTheDocument())
  expect(save).toHaveBeenLastCalledWith(1, 'photo', {revision:3, marks:[{ ...mark, text:'固定端' }]})
})
