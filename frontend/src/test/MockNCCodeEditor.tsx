import { useImperativeHandle, useRef, useState } from 'react'
import type { NCCodeEditorProps } from '../components/NCCodeEditor'

/** 弹层/API 测试隔离布局；真实编辑器由组件测试与浏览器验证。 */
export default function MockNCCodeEditor({ initialValue, label = 'NC 程序内容', readOnly, onDirtyChange, ref }: NCCodeEditorProps) {
  const [value, setValue] = useState(initialValue.replace(/\r\n/g, '\n'))
  const textarea = useRef<HTMLTextAreaElement>(null)
  useImperativeHandle(ref, () => ({ focus: () => textarea.current?.focus(), getValue: () => value }), [value])
  return <textarea ref={textarea} aria-label={label} readOnly={readOnly} value={value} onChange={(e) => {
    setValue(e.target.value)
    onDirtyChange(e.target.value !== initialValue.replace(/\r\n/g, '\n'))
  }} />
}
