import { EditorView } from '@codemirror/view'
import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createRef } from 'react'
import { beforeAll, expect, test, vi } from 'vitest'
import NCCodeEditor from './NCCodeEditor'
import type { NCCodeEditorHandle } from './NCCodeEditor'

beforeAll(() => {
  // jsdom 不计算布局，CodeMirror 的文本测量使用空矩形。
  Range.prototype.getClientRects = () => [] as unknown as DOMRectList
  Range.prototype.getBoundingClientRect = () => new DOMRect()
})

test('显示行号、只读切换，编辑和撤销恢复原文状态', async () => {
  const ref = createRef<NCCodeEditorHandle>()
  const dirty = vi.fn()
  const props = { ref, initialValue: 'O1000\nG01 X2 (精车)\nM30\n', onDirtyChange: dirty }
  const { rerender, container } = render(<NCCodeEditor {...props} readOnly />)
  expect(screen.getByRole('textbox', { name: 'NC 程序内容' })).toHaveAttribute('aria-readonly', 'true')
  expect(container.querySelector('.cm-lineNumbers')).toBeInTheDocument()
  rerender(<NCCodeEditor {...props} readOnly={false} />)
  expect(screen.getByRole('textbox', { name: 'NC 程序内容' })).toHaveAttribute('contenteditable', 'true')
  const editor = EditorView.findFromDOM(container.querySelector('.cm-editor') as HTMLElement)!
  act(() => editor.dispatch({ changes: { from: 0, to: 5, insert: 'O2000' } }))
  expect(ref.current?.getValue()).toContain('O2000')
  expect(dirty).toHaveBeenLastCalledWith(true)
  act(() => editor.dispatch({ changes: { from: 0, to: 5, insert: 'O1000' } }))
  expect(dirty).toHaveBeenLastCalledWith(false)
  const u = userEvent.setup()
  await u.type(screen.getByLabelText('跳转行号'), '999')
  await u.click(screen.getByRole('button', { name: '跳转' }))
  expect(screen.getByRole('alert')).toHaveTextContent('请输入 1~4 的行号')
  await u.clear(screen.getByLabelText('跳转行号'))
  await u.type(screen.getByLabelText('跳转行号'), '2')
  await u.click(screen.getByRole('button', { name: '跳转' }))
  expect(editor.state.selection.main.head).toBe(6)
  await u.click(screen.getByRole('button', { name: '查找 / 替换' }))
  expect(container.querySelector('.cm-search')).toBeInTheDocument()
})

test('大文件仅渲染可见行，读取仍包含完整内容', () => {
  const source = 'G01 X1.25 Y2.5 F100\n'.repeat(100000)
  const ref = createRef<NCCodeEditorHandle>()
  const { container } = render(<NCCodeEditor ref={ref} initialValue={source} readOnly onDirtyChange={() => {}} />)
  expect(container.querySelectorAll('.cm-line').length).toBeLessThan(1000)
  expect(ref.current?.getValue()).toBe(source)
})
