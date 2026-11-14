import { useImperativeHandle, useState } from 'react'
import type { NCCodeEditorProps } from '../components/NCCodeEditor'

/** 弹层/API 测试隔离布局；真实编辑器由组件测试与浏览器验证。 */
export default function MockNCCodeEditor({ initialValue, readOnly, onDirtyChange, ref }: NCCodeEditorProps) {
  const [value, setValue] = useState(initialValue.replace(/\r\n/g, '\n'))
  useImperativeHandle(ref, () => ({ getValue: () => value }), [value])
  return <textarea aria-label="NC 程序内容" readOnly={readOnly} value={value} onChange={(e) => {
    setValue(e.target.value)
    onDirtyChange(e.target.value !== initialValue.replace(/\r\n/g, '\n'))
  }} />
}
