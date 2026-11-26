import { screen } from '@testing-library/react'
import { expect, test, vi } from 'vitest'
import { api } from '../api'
import { renderApp } from '../test-utils'
import OperationPhotos from './OperationPhotos'

test('拍照请求后置相机，手动上传使用普通图片选择器', () => {
  const list = vi.spyOn(api, 'operationPhotos').mockResolvedValue([])
  try {
    renderApp(<OperationPhotos operationId={1} editable />)
    expect(screen.getByLabelText('拍摄装夹照片')).toHaveAttribute('capture', 'environment')
    const upload = screen.getByLabelText('上传装夹照片')
    expect(upload).toHaveAttribute('type', 'file')
    expect(upload).toHaveAttribute('accept', 'image/*')
    expect(upload).not.toHaveAttribute('capture')
  } finally { list.mockRestore() }
})
